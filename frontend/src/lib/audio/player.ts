/**
 * Gapless PCM16 player: binary WebSocket frames → Float32 AudioBuffers scheduled
 * back-to-back on one AudioContext. Reports the first scheduled buffer (for the
 * client-measured end-to-end latency) and when the queue drains.
 */
export class PCMPlayer {
  private ctx: AudioContext | null = null;
  private gain: GainNode | null = null;
  private sources = new Set<AudioBufferSourceNode>();
  private nextStartTime = 0;
  private muted = false;
  private firstReported = false;
  sampleRate = 24000;
  onFirstAudio: ((t: number) => void) | null = null;
  onDrain: (() => void) | null = null;

  static supported(): boolean {
    return typeof AudioContext !== "undefined";
  }

  configure(sampleRate: number): void {
    if (sampleRate > 0) this.sampleRate = sampleRate;
  }

  get busy(): boolean {
    return this.sources.size > 0;
  }

  /** Create/resume the context. Call from a user gesture so autoplay policies are satisfied. */
  async ensure(): Promise<boolean> {
    if (!PCMPlayer.supported()) return false;
    try {
      if (!this.ctx) {
        this.ctx = new AudioContext();
        this.gain = this.ctx.createGain();
        this.gain.gain.value = this.muted ? 0 : 1;
        this.gain.connect(this.ctx.destination);
      }
      if (this.ctx.state === "suspended") await this.ctx.resume();
      return this.ctx.state === "running";
    } catch {
      return false;
    }
  }

  /** Marks the start of a new reply so the next buffer counts as "first audio". */
  beginReply(): void {
    this.firstReported = false;
  }

  enqueue(data: ArrayBuffer): void {
    if (!this.ctx || !this.gain) {
      void this.ensure().then((ok) => {
        if (ok) this.enqueue(data);
      });
      return;
    }
    const ctx = this.ctx;
    const samples = Math.floor(data.byteLength / 2);
    if (samples === 0) return;
    const i16 = new Int16Array(data, 0, samples);
    const f32 = new Float32Array(samples);
    for (let i = 0; i < samples; i++) f32[i] = i16[i] / 32768;
    let buffer: AudioBuffer;
    try {
      buffer = ctx.createBuffer(1, samples, this.sampleRate);
    } catch {
      return;
    }
    buffer.copyToChannel(f32, 0);
    const src = ctx.createBufferSource();
    src.buffer = buffer;
    src.connect(this.gain);
    const now = ctx.currentTime;
    const start = Math.max(now + 0.03, this.nextStartTime);
    src.onended = () => {
      this.sources.delete(src);
      if (this.sources.size === 0) this.onDrain?.();
    };
    this.sources.add(src);
    try {
      src.start(start);
    } catch {
      this.sources.delete(src);
      return;
    }
    this.nextStartTime = start + buffer.duration;
    if (!this.firstReported) {
      this.firstReported = true;
      const delayMs = Math.max(0, (start - now) * 1000);
      this.onFirstAudio?.(performance.now() + delayMs);
    }
  }

  stop(): void {
    for (const s of this.sources) {
      try {
        s.onended = null;
        s.stop();
      } catch {
        // already stopped
      }
    }
    this.sources.clear();
    this.nextStartTime = 0;
  }

  setMuted(muted: boolean): void {
    this.muted = muted;
    if (this.gain) this.gain.gain.value = muted ? 0 : 1;
  }

  dispose(): void {
    this.stop();
    void this.ctx?.close().catch(() => undefined);
    this.ctx = null;
    this.gain = null;
  }
}

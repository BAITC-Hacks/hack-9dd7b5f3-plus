export interface MicChunk {
  pcm: Int16Array;
  rms: number;
}

/**
 * Microphone capture: getUserMedia → AudioWorklet (public/worklets/pcm-processor.js)
 * → ~100 ms Int16 PCM chunks at 16 kHz plus an RMS level for the meter.
 * Every browser API is guarded; `start()` throws a readable error instead.
 */
export class MicCapture {
  private ctx: AudioContext | null = null;
  private stream: MediaStream | null = null;
  private node: AudioWorkletNode | null = null;
  private source: MediaStreamAudioSourceNode | null = null;
  private starting: Promise<void> | null = null;
  onChunk: ((c: MicChunk) => void) | null = null;

  get active(): boolean {
    return this.stream !== null;
  }

  static supported(): boolean {
    return (
      typeof navigator !== "undefined" &&
      !!navigator.mediaDevices &&
      typeof navigator.mediaDevices.getUserMedia === "function" &&
      typeof AudioWorkletNode !== "undefined" &&
      typeof AudioContext !== "undefined"
    );
  }

  start(): Promise<void> {
    if (this.stream) return Promise.resolve();
    if (!this.starting) {
      this.starting = this.open().finally(() => {
        this.starting = null;
      });
    }
    return this.starting;
  }

  private async open(): Promise<void> {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      throw new Error("microphone API is not available (needs HTTPS or localhost)");
    }
    if (typeof AudioWorkletNode === "undefined" || typeof AudioContext === "undefined") {
      throw new Error("AudioWorklet is not supported in this browser");
    }
    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
        video: false,
      });
    } catch (e) {
      const name = e instanceof Error ? e.name : "";
      if (name === "NotAllowedError" || name === "SecurityError") throw new Error("microphone permission denied");
      if (name === "NotFoundError") throw new Error("no microphone found");
      throw new Error(`microphone unavailable (${e instanceof Error ? e.message : String(e)})`);
    }
    try {
      const ctx = new AudioContext();
      if (ctx.state === "suspended") await ctx.resume();
      await ctx.audioWorklet.addModule("/worklets/pcm-processor.js");
      const node = new AudioWorkletNode(ctx, "pcm-processor", {
        numberOfInputs: 1,
        numberOfOutputs: 1,
        outputChannelCount: [1],
        processorOptions: { targetRate: 16000, chunkMs: 100 },
      });
      node.port.onmessage = (ev: MessageEvent<{ pcm?: ArrayBuffer; rms?: number }>) => {
        const d = ev.data;
        if (!d || !d.pcm) return;
        const pcm = new Int16Array(d.pcm, 0, Math.floor(d.pcm.byteLength / 2));
        this.onChunk?.({ pcm, rms: typeof d.rms === "number" ? d.rms : 0 });
      };
      const source = ctx.createMediaStreamSource(stream);
      source.connect(node);
      // The processor writes nothing to its output, so this only keeps the graph alive.
      node.connect(ctx.destination);
      this.ctx = ctx;
      this.stream = stream;
      this.node = node;
      this.source = source;
    } catch (e) {
      for (const t of stream.getTracks()) t.stop();
      throw new Error(`audio pipeline failed (${e instanceof Error ? e.message : String(e)})`);
    }
  }

  stop(): void {
    try {
      this.source?.disconnect();
      this.node?.disconnect();
      if (this.node) this.node.port.onmessage = null;
      for (const t of this.stream?.getTracks() ?? []) t.stop();
      void this.ctx?.close().catch(() => undefined);
    } catch {
      // ignore teardown errors
    }
    this.source = null;
    this.node = null;
    this.stream = null;
    this.ctx = null;
  }
}

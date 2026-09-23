export type VadEvent = "start" | "end" | "abort" | null;

/**
 * Energy VAD with an adaptive noise floor (running minimum of RMS).
 * - speech starts after ≥3 consecutive chunks above max(floor×3, threshold)
 * - ends after 700 ms of silence following ≥300 ms of speech
 * - a burst shorter than 300 ms followed by silence is aborted (false start)
 * - utterances are capped at 15 s
 */
export class EnergyVAD {
  private floor = 0.01;
  private run = 0;
  private speechMs = 0;
  private silenceMs = 0;
  private utteranceMs = 0;
  inSpeech = false;
  sensitivity: number;

  static readonly START_CHUNKS = 3;
  static readonly END_SILENCE_MS = 700;
  static readonly MIN_SPEECH_MS = 300;
  static readonly MAX_UTTERANCE_MS = 15_000;

  constructor(sensitivity = 0.5) {
    this.sensitivity = sensitivity;
  }

  /** Absolute RMS threshold: 0.075 (low sensitivity) … 0.008 (high). */
  get threshold(): number {
    const s = Math.min(1, Math.max(0, this.sensitivity));
    return 0.075 - s * 0.067;
  }

  get noiseFloor(): number {
    return this.floor;
  }

  reset(): void {
    this.run = 0;
    this.speechMs = 0;
    this.silenceMs = 0;
    this.utteranceMs = 0;
    this.inSpeech = false;
  }

  /** Track the noise floor without detecting speech (used while the robot talks). */
  observe(rms: number): void {
    this.adapt(rms);
  }

  private adapt(rms: number): void {
    if (rms < this.floor) this.floor = Math.max(0.002, rms);
    else this.floor += (rms - this.floor) * 0.02;
  }

  feed(rms: number, dtMs: number): VadEvent {
    if (!this.inSpeech) {
      this.adapt(rms);
      const th = Math.max(this.floor * 3, this.threshold);
      this.run = rms > th ? this.run + 1 : 0;
      if (this.run >= EnergyVAD.START_CHUNKS) {
        this.inSpeech = true;
        this.speechMs = this.run * dtMs;
        this.utteranceMs = this.speechMs;
        this.silenceMs = 0;
        this.run = 0;
        return "start";
      }
      return null;
    }
    this.utteranceMs += dtMs;
    const th = Math.max(this.floor * 2.5, this.threshold * 0.8); // hysteresis
    if (rms > th) {
      this.speechMs += dtMs;
      this.silenceMs = 0;
    } else {
      this.silenceMs += dtMs;
    }
    if (this.utteranceMs >= EnergyVAD.MAX_UTTERANCE_MS) {
      this.inSpeech = false;
      return "end";
    }
    if (this.silenceMs >= EnergyVAD.END_SILENCE_MS) {
      this.inSpeech = false;
      return this.speechMs >= EnergyVAD.MIN_SPEECH_MS ? "end" : "abort";
    }
    return null;
  }
}

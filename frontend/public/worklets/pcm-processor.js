// AudioWorklet: downsamples the microphone signal from the context rate to
// 16 kHz (box-average anti-aliasing), packs ~100 ms Int16 chunks and posts them
// with the chunk RMS (0..1) to the main thread.
class PCMProcessor extends AudioWorkletProcessor {
  constructor(options) {
    super();
    const o = (options && options.processorOptions) || {};
    this.targetRate = o.targetRate || 16000;
    this.chunkSamples = Math.max(160, Math.round((this.targetRate * (o.chunkMs || 100)) / 1000));
    this.ratio = sampleRate / this.targetRate; // e.g. 3 for a 48 kHz context
    this.out = new Int16Array(this.chunkSamples);
    this.fill = 0;
    this.sumSq = 0;
    this.carry = new Float32Array(0);
    this.pos = 0;
  }

  process(inputs) {
    const input = inputs[0];
    if (!input || !input[0] || input[0].length === 0) return true;
    const ch = input[0];
    const buf = new Float32Array(this.carry.length + ch.length);
    buf.set(this.carry, 0);
    buf.set(ch, this.carry.length);
    let pos = this.pos;
    const ratio = this.ratio;
    while (pos + ratio <= buf.length) {
      const start = Math.floor(pos);
      const end = Math.max(start + 1, Math.floor(pos + ratio));
      let sum = 0;
      let n = 0;
      for (let i = start; i < end && i < buf.length; i++) {
        sum += buf[i];
        n++;
      }
      const v = Math.max(-1, Math.min(1, n ? sum / n : 0));
      this.out[this.fill++] = v < 0 ? v * 32768 : v * 32767;
      this.sumSq += v * v;
      pos += ratio;
      if (this.fill >= this.chunkSamples) this.flush();
    }
    const consumed = Math.floor(pos);
    this.carry = buf.slice(consumed);
    this.pos = pos - consumed;
    return true;
  }

  flush() {
    const n = this.fill;
    if (n === 0) return;
    const rms = Math.sqrt(this.sumSq / n);
    const pcm = this.out.slice(0, n);
    this.port.postMessage({ pcm: pcm.buffer, rms }, [pcm.buffer]);
    this.fill = 0;
    this.sumSq = 0;
  }
}

registerProcessor("pcm-processor", PCMProcessor);

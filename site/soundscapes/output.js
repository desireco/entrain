// The worker sends consecutive planar stereo chunks from a 44.1 kHz source.
// Position tracks the audio clock independently of timers and tab visibility.
const BLOCK = 16384, RATE = 44100;
class EntrainSoundscapeOutput extends AudioWorkletProcessor {
  constructor(options) {
    super();
    const {frame, index, blocks, startFrame} = options.processorOptions;
    this.firstFrame = frame;
    this.blocks = new Map();
    this.store(index, blocks);
    this.ahead = blocks.length;
    this.lastBlock = -1;
    this.requested = index + blocks.length - 1;
    this.startFrame = startFrame;
    this.stopFrame = Infinity;
    this.last = [0, 0];
    this.port.onmessage = ({data}) => {
      if (data.type === 'stream') {
        this.stream = data.port;
        this.stream.onmessage = ({data: msg}) => this.store(msg.index, msg.blocks);
      } else if (data.type === 'stop') {
        this.stopFrame = data.frame;
      }
    };
  }
  store(index, blocks) {
    for (let i = 0; i < blocks.length; i++) this.blocks.set(index+i, new Float32Array(blocks[i]));
  }
  sample(frame, ch) {
    frame = Math.max(0, frame);
    return this.blocks.get(Math.floor(frame / BLOCK))?.[ch*BLOCK + frame%BLOCK];
  }
  converted(frame, fraction, ch) {
    const b = this.sample(frame, ch);
    if (!fraction) return b;
    // Cubic interpolation is only needed when the hardware graph runs at
    // a different rate. The normal 44.1 kHz path copies samples exactly.
    const a = this.sample(frame-1,ch), c = this.sample(frame+1,ch), d = this.sample(frame+2,ch);
    return b + 0.5*fraction*(c-a+fraction*(2*a-5*b+4*c-d+fraction*(3*(b-c)+d-a)));
  }
  process(_, outputs) {
    const channels = outputs[0];
    if (!channels?.[0] || !channels[1]) {
      if (currentFrame >= this.stopFrame) {
        this.blocks.clear(); this.stream?.close();
        this.port.postMessage({type:'ended'});
        return false;
      }
      return true;
    }
    for (let i = 0; i < channels[0].length; i++) {
      const clock = currentFrame + i;
      if (clock >= this.stopFrame) {
        this.blocks.clear();
        this.stream?.close();
        this.port.postMessage({type:'ended'});
        return false;
      }
      if (clock < this.startFrame) continue;
      const pos = this.firstFrame + (clock-this.startFrame)*RATE/sampleRate;
      const frame = Math.floor(pos), part = pos-frame;
      let l,r;
      if (part === 0) {
        const chunk = this.blocks.get(Math.floor(frame/BLOCK)), off = frame%BLOCK;
        l = chunk?.[off]; r = chunk?.[BLOCK+off];
      } else {
        l = this.converted(frame, part, 0); r = this.converted(frame, part, 1);
      }
      if (!Number.isFinite(l) || !Number.isFinite(r)) {
        // Avoid a click. Tell transport this attempt failed instead of
        // skipping data and continuing at an unreported timeline position.
        const rest = channels[0].length - i;
        for (let j = 0; j < rest; j++) {
          const gain = (rest-1-j)/rest;
          channels[0][i+j] = this.last[0]*gain;
          channels[1][i+j] = this.last[1]*gain;
        }
        this.blocks.clear();
        this.stream?.close();
        this.port.postMessage({type:'error', message:'Sound stream ran out of buffered audio.'});
        return false;
      }
      channels[0][i] = this.last[0] = l;
      channels[1][i] = this.last[1] = r;
    }
    if (this.stream && currentFrame + channels[0].length > this.startFrame) {
      const pos = this.firstFrame + Math.max(0, currentFrame-this.startFrame)*RATE/sampleRate;
      const current = Math.floor(pos/BLOCK);
      if (current !== this.lastBlock) {
        this.lastBlock = current;
        for (const index of this.blocks.keys()) if (index < current-1) this.blocks.delete(index);
        while (this.requested < current+this.ahead-1) this.stream.postMessage(++this.requested);
      }
    }
    return true;
  }
}
registerProcessor('entrain-soundscape', EntrainSoundscapeOutput);

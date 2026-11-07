// The small entry script is idle until a procedural scene is played.
(() => {
  'use strict';
  const base = new URL('.', document.currentScript.src);
  const worklets = new WeakMap(), clients = new Map();
  let worker, nextId = 1;

  // AudioContext.currentTime is a frame clock expressed in seconds. Recover
  // that frame when conversion back lands a floating-point ulp above it.
  function clockFrame(seconds, rate) {
    const f = seconds*rate, nearest = Math.round(f);
    return Math.abs(f-nearest) <= 4*Number.EPSILON*Math.max(1,Math.abs(f)) ? nearest : Math.ceil(f);
  }

  function failWorker(failed, message) {
    if (worker !== failed) return;
    const old = worker;
    worker = null;
    for (const client of [...clients.values()]) client.fail(Error(message));
    old?.terminate();
  }
  function getWorker() {
    if (worker) return worker;
    worker = new Worker(new URL('worker.js', base));
    const current = worker;
    worker.onmessage = ({data}) => {
      if (data.type === 'fatal') failWorker(current, data.message);
      if (data.type === 'ready') clients.get(data.id)?.ready(data);
    };
    worker.onerror = event => {
      event.preventDefault();
      failWorker(current, event.message || 'Could not start the sound generator.');
    };
    return worker;
  }

  function waitForModule(ctx, load, signal) {
    return new Promise((resolve,reject) => {
      function clean() {
        ctx.removeEventListener('statechange',closed);
        signal?.removeEventListener('abort',abort);
      }
      function abort() { clean(); reject(new DOMException('Canceled','AbortError')); }
      function closed() { if (ctx.state === 'closed') {clean();reject(Error('Audio context closed.'));} }
      signal?.addEventListener('abort',abort,{once:true});
      ctx.addEventListener('statechange',closed);
      load.then(() => {clean(); resolve();}, e => {clean(); reject(e);});
    });
  }

  async function prepare(ctx, {kind, position = 0, seed = 0, signal, onerror}) {
    if (ctx.state === 'closed') throw Error('Audio context is closed.');
    if (!['rain', 'wind', 'surf'].includes(kind)) throw Error('Unknown procedural soundscape.');
    if (!Number.isInteger(seed) || seed < 0 || seed > 2147483647) throw Error('Seed must be an integer from 0 to 2147483647.');
    if (!ctx.audioWorklet) throw Error('Live soundscapes require Web Audio on localhost or HTTPS.');
    if (!Number.isSafeInteger(Math.round(position*44100)) || position < 0) throw Error('Invalid soundscape position.');
    if (signal?.aborted) throw new DOMException('Canceled','AbortError');
    if (!worklets.has(ctx)) {
      const loaded = ctx.audioWorklet.addModule(new URL('output.js', base)).catch(e => {worklets.delete(ctx); throw e;});
      worklets.set(ctx, loaded);
    }
    await waitForModule(ctx,worklets.get(ctx),signal);
    if (signal?.aborted) throw new DOMException('Canceled','AbortError');
    const dsp = getWorker(), id = nextId++, frame = Math.round(position*44100);
    const {port1, port2} = new MessageChannel();
    let node, handle, preroll, stopped = false, started = false, cleanupTimer;
    let rejectReady, stopTime = Infinity;
    function dispose() {
      if (stopped) return;
      stopped = true;
      clients.delete(id);
      dsp.postMessage({type:'close', id});
      signal?.removeEventListener('abort', abort);
      ctx.removeEventListener('statechange', closed);
      preroll = null;
      clearTimeout(cleanupTimer);
      node?.port.postMessage({type:'stop',frame:0});
      node?.disconnect();
      node?.port.close();
      node = null;
      port2.close();
      if (handle?.onended) {const ended = handle.onended; handle.onended = null; ended();}
    }
    function fail(error) {
      if (stopped) return;
      dispose();
      rejectReady(error);
      if (started) onerror?.(error);
    }
    function closed() {
      if (ctx.state === 'closed') {
        dispose();
        rejectReady(Error('Audio context closed.'));
      }
    }
    function abort() { fail(new DOMException('Canceled', 'AbortError')); }
    return new Promise((resolve, reject) => {
      rejectReady = reject;
      clients.set(id, {fail, ready(data) {
        try {
          preroll = data.blocks;
          data.blocks = null;
          handle = {
            onended:null,
            start(destination, when = ctx.currentTime) {
              if (stopped || started) throw Error('Soundscape source is no longer available.');
              try {
                node = new AudioWorkletNode(ctx,'entrain-soundscape', {
                  numberOfInputs:0, numberOfOutputs:1, outputChannelCount:[2],
                  processorOptions:{frame, index:data.index, blocks:preroll, startFrame:clockFrame(when,ctx.sampleRate)}
                });
                preroll = null;
                node.port.postMessage({type:'stream', port:port2}, [port2]);
                node.port.onmessage = ({data: msg}) => {
                  if (msg.type === 'error') fail(Error(msg.message));
                  if (msg.type === 'ended') dispose();
                };
                node.onprocessorerror = () => fail(Error('Soundscape output failed.'));
                started = true;
                signal?.removeEventListener('abort',abort);
                node.connect(destination);
              } catch (error) {
                fail(error);
                throw error;
              }
            },
            stop(when = ctx.currentTime) {
              if (stopped || when >= stopTime) return;
              stopTime = when;
              if (!started || ctx.state !== 'running' || when <= ctx.currentTime) {dispose(); return;}
              node.port.postMessage({type:'stop',frame:clockFrame(when,ctx.sampleRate)});
              clearTimeout(cleanupTimer);
              // A context may suspend before the scheduled audio-clock end.
              cleanupTimer = setTimeout(dispose, 500+1000*(when-ctx.currentTime));
            }
          };
          resolve(handle);
        } catch (e) { fail(e); }
      }});
      ctx.addEventListener('statechange',closed);
      signal?.addEventListener('abort',abort,{once:true});
      dsp.postMessage({type:'prepare', kind, seed, id, frame, port:port1}, [port1]);
    });
  }
  // Keep user-authored TOML work off the rendering worker and audio clock.
  // The parser uses the very same train.Read that entrain render calls.
  let parser;
  const pendingParses = new Map();
  function getParser() {
    if (parser) return parser;
    const thread = new Worker(new URL('worker.js',base));
    parser = thread;
    function failed(message) {
      if (parser !== thread) return;
      parser = null;
      thread.terminate();
      for (const request of [...pendingParses.values()]) request.fail(Error(message));
    }
    thread.onmessage = ({data}) => {
      if (data.type === 'fatal') failed(data.message);
      if (data.type !== 'parsed') return;
      const request = pendingParses.get(data.id);
      if (request) data.error ? request.fail(Error(data.error)) : request.done(data.session);
    };
    thread.onerror = event => {
      event.preventDefault();
      failed(event.message || 'Could not start the native session reader.');
    };
    return thread;
  }
  async function parseSession(text, {signal} = {}) {
    if (typeof text !== 'string') throw TypeError('Expected native session text.');
    if (signal?.aborted) throw new DOMException('Canceled','AbortError');
    const id = nextId++, thread = getParser();
    return new Promise((resolve,reject) => {
      function clean() {
        pendingParses.delete(id);
        signal?.removeEventListener('abort',abort);
      }
      function abort() { clean(); reject(new DOMException('Canceled','AbortError')); }
      pendingParses.set(id,{
        fail(error) { clean(); reject(error); },
        done(session) { clean(); resolve(session); }
      });
      signal?.addEventListener('abort',abort,{once:true});
      thread.postMessage({type:'parse',id,text});
    });
  }
  window.EntrainSoundscapes = {prepare, parseSession};
})();

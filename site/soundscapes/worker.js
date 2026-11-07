// Go DSP stays off both the main and real-time audio threads. Cache a small
// moving window per scene so pause/resume doesn't replay the whole session.
'use strict';
importScripts('wasm_exec.js');
const BLOCK = 16384, HISTORY = 24, PREROLL = 12;
const scenes = new Map(), clients = new Map(), jobs = [];
let boot, running = false;

function runtime() {
  if (!boot) {
    boot = (async () => {
      const go = new Go();
      const response = await fetch(new URL('synth.wasm', self.location.href));
      if (!response.ok) throw Error('Could not load the sound generator.');
      const {instance} = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
      go.run(instance).then(
        () => postMessage({type:'fatal',message:'The sound generator stopped.'}),
        e => postMessage({type:'fatal',message:String(e.message || e)})
      );
      if (typeof soundscapePull !== 'function') throw Error('Sound generator failed to initialize.');
    })();
  }
  return boot;
}

// Keep at most three cached variant states when inactive. Extra active
// crossfades retain their streams only until their clients close.
function trimScenes() {
  const live = new Set(Array.from(clients.values(), client => client.key));
  for (const key of scenes.keys()) {
    if (scenes.size <= 3) break;
    if (!live.has(key)) {
      scenes.delete(key);
      soundscapeDrop(key);
    }
  }
}
function newScene({kind, seed, key}) {
  if (!soundscapeReset(kind,key,seed)) throw Error('Unknown soundscape: ' + kind);
  const scene = {next: 0, history: new Map()};
  scenes.set(key, scene);
  trimScenes();
  return scene;
}

function generate(key, scene) {
  const pcm = new Float32Array(2 * BLOCK);
  soundscapePull(key, new Uint8Array(pcm.buffer));
  scene.history.set(scene.next++, pcm);
  if (scene.history.size > HISTORY) scene.history.delete(scene.next - HISTORY - 1);
}

// Yield during cold seeks so cancel and work for other voices are serviced;
// ordinary playback refills are driven by the audio thread, never a UI timer.
const wake = new MessageChannel();
let wakeup;
wake.port1.onmessage = () => wakeup();
function yieldTask() { return new Promise(resolve => { wakeup = resolve; wake.port2.postMessage(0); }); }

async function drain() {
  if (running) return;
  running = true;
  try {
    await runtime();
    while (jobs.length) {
      const job = jobs.shift(), client = clients.get(job.id);
      if (!client) continue;
      let scene = scenes.get(client.key);
      if (!scene || (job.index < scene.next && !scene.history.has(job.index))) scene = newScene(client);
      else {scenes.delete(client.key); scenes.set(client.key,scene);}
      const end = job.index + job.count;
      const skip = Math.min(8, Math.max(0,job.index-scene.next));
      if (skip) {
        soundscapeSkip(client.key,skip);
        scene.next += skip;
        scene.history.clear();
      }
      for (let n = skip; scene.next < end && n < 8; n++) generate(client.key, scene);
      if (scene.next < end) {
        jobs.push(job);
        await yieldTask();
        continue;
      }
      const blocks = [];
      for (let i = job.index; i < end; i++) blocks.push(scene.history.get(i).slice().buffer);
      if (job.initial) {
        postMessage({type: 'ready', id: job.id, index: job.index, blocks}, blocks);
      } else {
        client.port.postMessage({index: job.index, blocks}, blocks);
      }
    }
  } catch (e) {
    postMessage({type: 'fatal', message: String(e.message || e)});
  } finally {
    running = false;
  }
}

onmessage = ({data}) => {
  const {type, id} = data;
  if (type === 'parse') {
    runtime().then(() => postMessage({type:'parsed',id,...soundscapeParseSession(data.text)}))
      .catch(error => postMessage({type:'fatal',message:String(error.message || error)}));
    return;
  }
  if (type === 'close') {
    clients.get(id)?.port.close();
    clients.delete(id);
    trimScenes();
    return;
  }
  if (type !== 'prepare') return;
  const {kind, seed, port, frame} = data;
  clients.set(id, {kind, seed, key:kind + '/' + seed, port});
  port.onmessage = ({data: index}) => {
    if (!clients.has(id) || !Number.isSafeInteger(index) || index < 0) return;
    jobs.push({id, index, count: 1});
    drain();
  };
  const index = Math.max(0, Math.floor((frame - 1) / BLOCK));
  jobs.push({id, index, count: PREROLL, initial: true});
  drain();
};

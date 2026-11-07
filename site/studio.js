
(function () {
  'use strict';
  var $ = function (s) { return document.querySelector(s); };
  var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  /* ============ Session definitions (mirror the shipped examples/) ============ */
  var SESSIONS = {
    'focus-40-pink': {
      name: 'focus-40-pink', sub: '40 Hz γ over pink noise',
      tones: [{ l: 180, r: 220, beat: 40, band: 'γ', amp: 0.30 }],
      bed: 'pink', bedLevel: 0.09, out: 'focus-40-pink.mp3 · 320 kbps'
    },
    'meditate-alpha': {
      name: 'meditate-alpha', sub: '10 Hz α over soft pink',
      tones: [{ l: 200, r: 210, beat: 10, band: 'α', amp: 0.25 }],
      bed: 'pink', bedLevel: 0.045, out: 'meditate-alpha.flac · 24-bit'
    },
    'sleep-delta': {
      name: 'sleep-delta', sub: '3 Hz δ over brown noise',
      tones: [{ l: 100, r: 103, beat: 3, band: 'δ', amp: 0.30 }],
      bed: 'brown', bedLevel: 0.085, out: 'sleep-delta.flac · 24-bit'
    }
  };
  var BACKGROUNDS = { pink: 'Pink noise', white: 'White noise', rain: 'Soft rain', wind: 'Wind', surf: 'Ocean surf' };
  var generatedBeds = new Set(['rain', 'wind', 'surf']);
  var cueNames = { off: 'Off', tone: 'Tone', bell: 'Bell', noise: 'Noise', sections: 'Timed sections' };
  var creatorPage = document.body.dataset.page === 'session-creator';
  var current = localStorage.getItem('entrain.session');
  var handoff = null;
  if (creatorPage && SESSIONS[current]) {
    // A binaural audition continues as its bed; brown has no preview bed.
    handoff = {preset: current, brown: SESSIONS[current].bed === 'brown'};
    var carried = localStorage.getItem('entrain.background');
    current = Object.hasOwn(BACKGROUNDS, carried) ? carried : 'pink';
  }
  var arriving = !!localStorage.getItem('entrain.session');
  if (!Object.hasOwn(SESSIONS, current) && !Object.hasOwn(BACKGROUNDS, current)) current = 'pink';
  var lastBackground = Object.hasOwn(BACKGROUNDS, current) ? current : localStorage.getItem('entrain.background');
  if (!Object.hasOwn(BACKGROUNDS, lastBackground)) lastBackground = 'pink';
  var cue = localStorage.getItem('entrain.cue');
  if (!Object.hasOwn(cueNames, cue)) cue = 'off';
  function restoreSlider(id, storageKey) {
    var el = $(id), saved = localStorage.getItem(storageKey);
    if (saved !== null && saved.trim() && Number(saved) >= Number(el.min) && Number(saved) <= Number(el.max))
      el.value = saved;
    return el.valueAsNumber;
  }
  var cueVolume = restoreSlider('#cue-volume', 'entrain.cueVolume');
  var cueInterval = restoreSlider('#cue-interval', 'entrain.cueInterval');
  function durationSeconds(text) {
    return EntrainEditor.timeMS(text) / 1000 || 0;
  }
  var fileDuration = localStorage.getItem('entrain.duration') || '6m';
  if (!durationSeconds(fileDuration)) fileDuration = '6m';
  $('#mix-duration').value = fileDuration;
  var presets = Array.from(document.querySelectorAll('[data-length]'));
  function durationPreset(value) {
    return presets.find(b => b.dataset.length !== 'custom' && durationSeconds(b.dataset.length) === durationSeconds(value))?.dataset.length || 'custom';
  }
  var lengthChoice = localStorage.getItem('entrain.lengthChoice') === 'custom' ? 'custom' : durationPreset(fileDuration);
  function syncDuration() {
    presets.forEach(b => b.setAttribute('aria-pressed', String(b.dataset.length === lengthChoice)));
    $('#custom-length').hidden = lengthChoice !== 'custom';
  }
  syncDuration();
  $('#session-details').open = !BACKGROUNDS[current];

  var seedText = localStorage.getItem('entrain.seed') || '';
  $('#mix-seed').value = seedText;
  var sectionEditor = new EntrainEditor.Sections(onScheduleEdit, selectSectionRange, function () {
    return EntrainEditor.timeMS($('#mix-duration').value);
  });
  function selectedSeed(bed = current) {
    if (!generatedBeds.has(bed) || !seedText.trim()) return 0;
    if (!/^\d+$/.test(seedText.trim()) || Number(seedText) > 2147483647) return NaN;
    return Number(seedText);
  }
  function selectedRange() {
    if (!$('#preview-excerpt').checked || !BACKGROUNDS[current]) return null;
    var start = EntrainEditor.timeMS($('#preview-start').value);
    var length = EntrainEditor.timeMS($('#preview-length').value);
    var total = EntrainEditor.timeMS($('#mix-duration').value);
    if (!(start >= 0 && start < total && length > 0)) return {error:true};
    var first = Math.ceil(start * 44100 / 1000);
    var last = Math.min(Math.floor(total * 44100 / 1000), Math.ceil((start + length) * 44100 / 1000));
    if (first >= last) return {error:true};
    return {start:first / 44100, end:last / 44100, startMS:start, endMS:Math.min(total, start + length), lengthMS:length};
  }
  function previewStart() {
    var range = selectedRange();
    return range && !range.error ? range.start : 0;
  }
  var validControls = true;
  function sourceValid() { return validControls; }

  function holdGain(gain, time) {
    if (gain.cancelAndHoldAtTime) gain.cancelAndHoldAtTime(time);
    else {
      var level = gain.value;
      gain.cancelScheduledValues(time);
      gain.setValueAtTime(level, time);
    }
  }
  function fadeGain(gain, target, time, duration) {
    holdGain(gain, time);
    gain.linearRampToValueAtTime(target, time + duration);
  }

  /* ============ Binaural engine ============ */
  var engine = {
    ctx: null, master: null, analyser: null,
    toneSlots: [],       // [{oL,oR,gL,gR}]
    bed: null, bedGain: null, bedType: null,
    playing: false, elapsed: 0, startedAt: null, suspendTimer: null,
    synthBus: null, loopBus: null, loops: { bed: null, cue: null },
    load: null, loading: false, deadline: null, activeSynth: null, retuneTimer: null,
    timeline: null, cueBuffers: new Map(),

    init: function () {
      var AC = window.AudioContext || window.webkitAudioContext;
      try { this.ctx = new AC({sampleRate: 44100}); }
      catch (err) { this.ctx = new AC(); }
      this.master = this.ctx.createGain();
      this.master.gain.value = 0;
      this.analyser = this.ctx.createAnalyser();
      this.analyser.fftSize = 4096;
      this.analyser.smoothingTimeConstant = 0.82;
      this.master.connect(this.analyser);
      this.analyser.connect(this.ctx.destination);
      this.synthBus = this.ctx.createGain();
      this.loopBus = this.ctx.createGain();
      this.synthBus.gain.value = 0;
      this.loopBus.gain.value = 0;
      this.synthBus.connect(this.master);
      this.loopBus.connect(this.master);
      this.setVolume($('#volume').valueAsNumber);
    },
    noiseBuffer: function (type) {
      var len = this.ctx.sampleRate * 4, buf = this.ctx.createBuffer(2, len, this.ctx.sampleRate);
      for (var ch = 0; ch < 2; ch++) {
        var d = buf.getChannelData(ch);
        if (type === 'pink') {
          var b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0;
          for (var i = 0; i < len; i++) {
            var w = Math.random() * 2 - 1;
            b0 = 0.99886 * b0 + w * 0.0555179; b1 = 0.99332 * b1 + w * 0.0750759;
            b2 = 0.96900 * b2 + w * 0.1538520; b3 = 0.86650 * b3 + w * 0.3104856;
            b4 = 0.55000 * b4 + w * 0.5329522; b5 = -0.7616 * b5 - w * 0.0168980;
            d[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.06;
            b6 = w * 0.115926;
          }
        } else { /* brown */
          var last = 0;
          for (var j = 0; j < len; j++) {
            var w2 = Math.random() * 2 - 1;
            last = (last + 0.02 * w2) / 1.02;
            d[j] = last * 3.2;
          }
        }
      }
      return buf;
    },
    startBed: function (type, level, immediate) {
      var t = this.ctx.currentTime, fade = immediate ? 0.001 : 0.8;
      if (this.bed) {
        var old = this.bed, oldG = this.bedGain;
        oldG.gain.cancelScheduledValues(t); oldG.gain.setValueAtTime(oldG.gain.value, t);
        oldG.gain.linearRampToValueAtTime(0.0001, t + fade);
        setTimeout(function () { try { old.stop(); } catch (e) {} try { old.disconnect(); oldG.disconnect(); } catch (e) {} }, (fade + 0.2) * 1000);
        this.bed = null; this.bedGain = null;
      }
      var src = this.ctx.createBufferSource(), g = this.ctx.createGain();
      src.buffer = this.noiseBuffer(type); src.loop = true;
      g.gain.setValueAtTime(0.0001, t);
      g.gain.linearRampToValueAtTime(level, t + fade);
      src.connect(g); g.connect(this.synthBus); src.start();
      this.bed = src; this.bedGain = g; this.bedType = type;
    },
    applySession: function (def, immediate) {
      var t = this.ctx.currentTime, glide = immediate ? 0.001 : 0.9;
      for (var i = 0; i < Math.max(2, def.tones.length); i++) {
        var tone = def.tones[i], slot = this.toneSlots[i];
        if (tone) {
          if (!slot) {
            slot = { oL: this.ctx.createOscillator(), oR: this.ctx.createOscillator(),
                     gL: this.ctx.createGain(), gR: this.ctx.createGain(),
                     pL: this.ctx.createStereoPanner ? this.ctx.createStereoPanner() : null,
                     pR: this.ctx.createStereoPanner ? this.ctx.createStereoPanner() : null };
            slot.oL.type = 'sine'; slot.oR.type = 'sine';
            slot.gL.gain.value = 0; slot.gR.gain.value = 0;
            if (slot.pL) { slot.pL.pan.value = -1; slot.pR.pan.value = 1; }
            slot.oL.connect(slot.gL); slot.oR.connect(slot.gR);
            if (slot.pL) { slot.gL.connect(slot.pL); slot.pL.connect(this.synthBus);
                           slot.gR.connect(slot.pR); slot.pR.connect(this.synthBus); }
            else { slot.gL.connect(this.synthBus); slot.gR.connect(this.synthBus); }
            slot.oL.start(); slot.oR.start();
            this.toneSlots[i] = slot;
            slot.oL.frequency.setValueAtTime(tone.l, t);
            slot.oR.frequency.setValueAtTime(tone.r, t);
          } else {
            slot.oL.frequency.cancelScheduledValues(t);
            slot.oR.frequency.cancelScheduledValues(t);
            slot.oL.frequency.setValueAtTime(slot.oL.frequency.value, t);
            slot.oR.frequency.setValueAtTime(slot.oR.frequency.value, t);
            slot.oL.frequency.exponentialRampToValueAtTime(tone.l, t + glide);
            slot.oR.frequency.exponentialRampToValueAtTime(tone.r, t + glide);
          }
          ['gL', 'gR'].forEach(function (k) {
            var g = slot[k], target = tone.amp;
            g.gain.cancelScheduledValues(t);
            g.gain.setValueAtTime(Math.max(g.gain.value, 0.0001), t);
            g.gain.linearRampToValueAtTime(target, t + (immediate ? 0.05 : 1.2));
          });
        } else if (slot) {
          ['gL', 'gR'].forEach(function (k) {
            var g = slot[k];
            g.gain.cancelScheduledValues(t);
            g.gain.setValueAtTime(Math.max(g.gain.value, 0.0001), t);
            g.gain.linearRampToValueAtTime(0.0001, t + 0.6);
          });
        }
      }
      if (this.bedType !== def.bed) this.startBed(def.bed, def.bedLevel, immediate);
      else if (this.bedGain) {
        this.bedGain.gain.cancelScheduledValues(t);
        this.bedGain.gain.setValueAtTime(this.bedGain.gain.value, t);
        this.bedGain.gain.linearRampToValueAtTime(def.bedLevel, t + (immediate ? 0.05 : 1.2));
      } else this.startBed(def.bed, def.bedLevel, immediate);
    },
    rampMaster: function (target, duration) {
      fadeGain(this.master.gain, target, this.ctx.currentTime, duration);
    },
    setVolume: function (percent) {
      this._vol = Math.pow(percent / 100, 1.6) * 0.9;
      if (this.playing && this.startedAt !== null) this.rampMaster(this._vol, 0.08);
    },
    position: function () {
      return this.elapsed + (this.startedAt === null ? 0 : Math.max(0, this.ctx.currentTime - this.startedAt));
    },
    previewLimit: function () {
      var range = selectedRange(), duration = durationSeconds(fileDuration);
      if (range && !range.error) return range.end;
      return generatedBeds.has(current) ? duration : Math.min(60, duration);
    },
    clearDeadline: function () {
      if (!this.deadline) return;
      this.deadline.onended = null;
      this.deadline.stop();
      this.deadline = null;
    },
    scheduleDeadline: function () {
      this.clearDeadline();
      if (!BACKGROUNDS[current] || !this.playing || this.startedAt === null) return;
      var t = this.ctx.currentTime, end = this.startedAt - this.elapsed + this.previewLimit(), remaining = end - t;
      if (this.timeline) this.timeline.setEnd(this.previewLimit());
      if (remaining <= 0) { this.finishPreview(); return; }
      var gain = this.loopBus.gain, fade = Math.min(selectedRange() ? 0.025 : 0.5, remaining / 2);
      // Timed on the audio clock: the mix reaches silence at its limit even in a hidden tab.
      fadeGain(gain, 1, t, Math.min(selectedRange() ? 0.025 : 0.14, remaining / 4));
      gain.setValueAtTime(1, end - fade);
      gain.linearRampToValueAtTime(0, end);
      var timer = this.ctx.createBufferSource(), self = this;
      timer.buffer = this.ctx.createBuffer(1, 1, this.ctx.sampleRate);
      timer.onended = function () { if (self.deadline === timer) self.finishPreview(); };
      this.deadline = timer;
      timer.start(end);
    },
    finishPreview: function () {
      this.pause();
      this.elapsed = this.previewLimit();
      updateStatus('Preview complete. Press play to restart.');
    },
    cancelLoad: function () {
      if (this.load) this.load.abort();
      this.load = null;
      this.loading = false;
    },
    // Fetch only the selected loops. With one bed and one cue active we retain
    // at most their decoded buffers, apart from a short outgoing crossfade.
    readLoop: async function (slot, key, seconds, signal) {
      if (!key || (this.loops[slot] && this.loops[slot].key === key)) return null;
      var filename = slot === 'bed' ? key + '.flac' : 'cue-' + key + '.wav';
      var response = await fetch('audio/' + filename, { signal: signal });
      if (!response.ok) throw new Error('audio unavailable');
      var samples = await this.ctx.decodeAudioData(await response.arrayBuffer());
      if (samples.numberOfChannels !== 2 || samples.duration + 0.001 < seconds) throw new Error('incomplete loop');
      return samples;
    },
    readBed: async function (key, restart, signal) {
      if (!generatedBeds.has(key)) return this.readLoop('bed', key, 60, signal);
      var bed = this.loops.bed, seed = selectedSeed();
      if (bed && bed.key === key && bed.seed === seed && !restart) return null;
      if (!window.EntrainSoundscapes) throw new Error('Sound generator unavailable');
      var self = this;
      var handle = await window.EntrainSoundscapes.prepare(this.ctx, {
        kind: key, position: this.position(), seed: seed, signal: signal,
        onerror: function () {
          if (current === key && self.loops.bed && self.loops.bed.handle === handle && self.playing) {
            self.pause();
            updateStatus('The sound generator stopped. Press play to retry or render with the local CLI.');
          }
        }
      });
      return handle;
    },
    removeBed: function (duration) {
      var old = this.loops.bed;
      if (!old) return;
      this.loops.bed = null;
      var t = this.ctx.currentTime;
      fadeGain(old.gain.gain, 0, t, duration);
      if (old.handle) {
        old.handle.onended = function () { old.gain.disconnect(); };
        old.handle.stop(t + duration);
      } else old.source.stop(t + duration);
    },
    replaceBed: function (key, prepared, restart, when) {
      var old = this.loops.bed, t = when ?? this.ctx.currentTime, generated = generatedBeds.has(key);
      if (old && old.key === key) {
        if (!restart && (!generated || old.seed === selectedSeed())) return;
        if (!generated) prepared = old.source.buffer;
      }
      this.removeBed(0.28);
      if (!key) return;
      var gain = this.ctx.createGain();
      gain.connect(this.loopBus);
      gain.gain.setValueAtTime(0, t);
      gain.gain.linearRampToValueAtTime(1, t + (selectedRange() ? 0.025 : 0.28));
      if (generated) {
        this.loops.bed = {key: key, seed: selectedSeed(), handle: prepared, gain: gain};
        prepared.start(gain, t);
        return;
      }
      var source = this.ctx.createBufferSource();
      source.buffer = prepared;
      source.loop = true;
      source.loopEnd = 60;
      source.connect(gain);
      source.onended = function () { source.disconnect(); gain.disconnect(); };
      source.start(t, this.position() % 60);
      this.loops.bed = {key: key, source: source, gain: gain};
    },
    setCueGain: function () {
      if (this.loops.cue) fadeGain(this.loops.cue.gain.gain, cueVolume / 100, this.ctx.currentTime, 0.08);
    },
    retuneCue: function () {
      if (this.retuneTimer !== null || !this.playing || !this.loops.cue) return;
      var self = this;
      // Coalesce pointer movement. The replacement itself is scheduled on
      // the audio clock at the next strike, never by this UI timer.
      this.retuneTimer = setTimeout(function () {
        self.retuneTimer = null;
        if (self.playing && !self.loading && self.loops.cue) self.scheduleCue(cue, null, false);
      }, 60);
    },
    clearCueWake: function (state) {
      if (!state || !state.wake) return;
      state.wake.onended = null;
      state.wake.stop();
      state.wake = null;
    },
    fillCues: function () {
      var state = this.loops.cue;
      if (!this.playing || !state) return;
      this.clearCueWake(state);
      var now = this.ctx.currentTime, rate = state.samples.sampleRate;
      var end = state.sessionOrigin + this.previewLimit();
      // Even a three-hour mix retains only two minutes of future strikes.
      // The audio-clock callback refills halfway through that window.
      var horizon = Math.min(end, now + 120);
      state.events = state.events.filter(function (hit) { return hit.end > now; });
      state.nextIndex = Math.max(state.nextIndex, Math.floor((now - state.origin) * 1000 / state.interval));
      for (;; state.nextIndex++) {
        var n = state.nextIndex, destFrame = Math.ceil(n * state.interval * rate / 1000);
        var onset = state.origin + destFrame / rate;
        if (onset >= horizon) break;
        var when = Math.max(now, state.notBefore, onset), strike = n % 16;
        var first = Math.ceil(strike * 625 * rate / 1000);
        var last = Math.ceil((strike + 1) * 625 * rate / 1000);
        var available = Math.min(last - first, Math.ceil((n + 1) * state.interval * rate / 1000) - destFrame);
        var skipped = Math.max(0, Math.round((when - onset) * rate));
        if (skipped >= available) continue;
        var source = this.ctx.createBufferSource(), length = (available - skipped) / rate;
        source.buffer = state.samples;
        source.connect(state.gain);
        state.remaining++;
        source.onended = function () {
          // Finished one-shot sources are released naturally. Disconnecting
          // each hit can rebuild a live graph mid-quantum in Chromium.
          if (--state.remaining === 0 && state.retired) state.gain.disconnect();
        };
        source.start(when, (first + skipped) / rate, length);
        state.events.push({source: source, when: when, end: when + length, index: n});
      }
      if (horizon < end) {
        var wake = this.ctx.createBufferSource(), self = this;
        wake.buffer = this.ctx.createBuffer(1, 1, this.ctx.sampleRate);
        wake.onended = function () {
          if (self.loops.cue !== state || state.wake !== wake) return;
          state.wake = null;
          self.fillCues();
        };
        state.wake = wake;
        wake.start(now + 60);
      }
    },
    scheduleCue: function (key, samples, restart, when) {
      var old = this.loops.cue, now = this.ctx.currentTime, pos = this.position();
      var sessionOrigin = this.startedAt === null ? (when ?? now) - pos : this.startedAt - this.elapsed;
      if (old && old.key === key) {
        if (!restart && old.interval === cueInterval) return;
        samples = old.samples;
      }
      // Finish the current strike and replace the next one on its original
      // side. Resume or start a new selection at the displayed position.
      var next = old && key && !restart ? old.events.find(function (hit) { return hit.when > now; }) : null;
      if (old) {
        this.clearCueWake(old);
        old.retired = true;
        if (old.remaining === 0) old.gain.disconnect();
        var cutoff = next ? next.when : now + 0.06;
        if (!next) fadeGain(old.gain.gain, 0, now, 0.06);
        old.events.forEach(function (hit) {
          if (hit.when >= cutoff) hit.source.stop(now);
          else if (hit.end > cutoff) hit.source.stop(cutoff);
        });
        this.loops.cue = null;
      }
      if (!key) return;
      var interval = cueInterval;
      var n = next ? next.index : Math.floor(1000 * pos / interval);
      var origin = next ? next.when - Math.ceil(n * interval * samples.sampleRate / 1000) / samples.sampleRate : sessionOrigin;
      var gain = this.ctx.createGain(), start = next ? next.when : (when ?? now);
      gain.connect(this.loopBus);
      gain.gain.setValueAtTime(0, start);
      gain.gain.linearRampToValueAtTime(cueVolume / 100, start + (next ? 0.005 : selectedRange() ? 0.025 : 0.06));
      this.loops.cue = {key: key, samples: samples, gain: gain, interval: interval, events: [],
        nextIndex: n, origin: origin, notBefore: start, sessionOrigin: sessionOrigin, remaining: 0, retired: false, wake: null};
      this.fillCues();
    },
    clearTimeline: function () {
      if (this.timeline) this.timeline.stop();
      this.timeline = null;
    },
    readCues: async function (keys, signal) {
      await Promise.all(keys.map(async key => {
        if (this.cueBuffers.has(key)) return;
        var response = await fetch('audio/cue-' + key + '.wav', {signal:signal});
        if (!response.ok) throw new Error('Cue unavailable');
        var buffer = await this.ctx.decodeAudioData(await response.arrayBuffer());
        if (buffer.numberOfChannels !== 2 || buffer.duration + 0.001 < 10) throw new Error('Incomplete cue');
        if (!signal.aborted) this.cueBuffers.set(key, buffer);
      }));
    },
    selectionReady: function (when) {
      if (this.startedAt === null) {
        this.startedAt = when ?? this.ctx.currentTime;
        this.rampMaster(this._vol, selectedRange() ? 0.035 : 1.2);
      }
      this.loading = false;
      this.scheduleDeadline();
      updateStatus();
    },
    applySelection: function () {
      this.cancelLoad();
      if (!this.playing) return;
      if (SESSIONS[current]) {
        this.replaceBed(null);
        this.clearTimeline();
        this.scheduleCue(null);
        if (this.activeSynth !== current) this.applySession(SESSIONS[current], !this.toneSlots.length);
        this.activeSynth = current;
        fadeGain(this.synthBus.gain, 1, this.ctx.currentTime, 0.28);
        this.selectionReady();
        return;
      }
      var self = this, controller = new AbortController(), bedKey = current;
      var timed = cue === 'sections', cueKey = timed || cue === 'off' ? null : cue;
      var sections = timed ? sectionEditor.validate() : null;
      if (timed && !sections) { this.pause(); return; }
      var needed = timed ? [...new Set(sections.filter(s => s.volume > 0).map(s => s.cue))] : cueKey ? [cueKey] : [];
      var preparedBed = null, restart = this.startedAt === null;
      this.load = controller;
      this.loading = true;
      updateStatus();
      Promise.all([
        this.readBed(bedKey, restart, controller.signal).then(function (bed) {
          preparedBed = bed;
          return bed;
        }),
        this.readCues(needed, controller.signal)
      ]).then(function (buffers) {
        if (controller !== self.load) return;
        self.load = null;
        // After a pause fade or a scene change, start both layers at the
        // transport position; changing the cue alone leaves the bed running.
        // One audio-clock anchor keeps the bed, strike index, section fades,
        // and displayed position aligned even if setting up the graph takes time.
        var offline = typeof OfflineAudioContext !== 'undefined' && self.ctx instanceof OfflineAudioContext;
        var when = self.ctx.currentTime + (restart && !offline ? 0.06 : 0);
        var origin = restart ? when - self.elapsed : self.startedAt - self.elapsed;
        self.replaceBed(bedKey, buffers[0], restart, when);
        preparedBed = null;
        self.clearTimeline();
        self.scheduleCue(cueKey, self.cueBuffers.get(cueKey), restart, when);
        if (timed && sections.length) {
          self.timeline = new EntrainEditor.Timeline(self.ctx, self.loopBus, sections, self.cueBuffers, self.position(), self.previewLimit(), origin);
        }
        fadeGain(self.synthBus.gain, 0, self.ctx.currentTime, 0.28);
        self.selectionReady(when);
      }).catch(function () {
        if (controller.signal.aborted) return;
        controller.abort();
        if (generatedBeds.has(bedKey) && preparedBed && self.loops.bed?.handle !== preparedBed) preparedBed.stop();
        self.pause();
        updateStatus(generatedBeds.has(bedKey)
          ? 'Could not start the generated sound. Press play to retry or render with the local CLI.'
          : 'Could not load this preview. Press play to retry.');
      });
    },
    play: function () {
      if (!this.ctx) this.init();
      clearTimeout(this.suspendTimer);
      this.suspendTimer = null;
      if (BACKGROUNDS[current] && (this.elapsed < previewStart() || this.elapsed >= this.previewLimit())) this.elapsed = previewStart();
      if (this.ctx.state === 'suspended') this.ctx.resume();
      this.playing = true;
      this.applySelection();
    },
    pause: function () {
      this.elapsed = this.position();
      this.startedAt = null;
      this.cancelLoad();
      this.clearDeadline();
      this.clearCueWake(this.loops.cue);
      this.clearTimeline();
      this.rampMaster(0, 0.5);
      if (this.loops.bed?.handle) this.removeBed(0.5);
      this.playing = false;
      var self = this;
      this.suspendTimer = setTimeout(function () {
        self.suspendTimer = null;
        if (!self.playing) self.ctx.suspend();
      }, 620);
      updateStatus();
    }

  };

  /* ============ UI wiring ============ */
  var playBtn = $('#play-btn'), statusLine = $('#status-line'), scopeState = $('#scope-state');
  function fmt(s) {
    var m = Math.floor(s / 60), r = Math.floor(s % 60);
    return (m < 10 ? '0' + m : m) + ':' + (r < 10 ? '0' + r : r);
  }
  function applyNativeScore(score) {
    var times = [score.durationMS, ...score.sections.flatMap(section => [section.start, section.length, section.fadeIn, section.fadeOut])];
    if (!Object.hasOwn(BACKGROUNDS, score.background) || times.some(ms => !Number.isSafeInteger(ms) || ms > 10800000)) {
      var err = new Error('Valid CLI session. This preview supports the five listed backgrounds and whole milliseconds up to 3h. Download the text to render it with the CLI.');
      err.name = 'UnsupportedPreview';
      throw err;
    }
    if (engine.playing) engine.pause();
    current = score.background;
    fileDuration = EntrainEditor.token(score.durationMS);
    $('#mix-duration').value = fileDuration;
    lengthChoice = durationPreset(fileDuration);
    syncDuration();
    seedText = score.seed ? String(score.seed) : '';
    $('#mix-seed').value = seedText;
    cue = score.sections.length ? 'sections' : score.cue === 'none' ? 'off' : score.cue;
    cueVolume = score.volume;
    cueInterval = score.interval;
    $('#cue-volume').value = cueVolume;
    $('#cue-interval').value = cueInterval;
    var time = EntrainEditor.token;
    sectionEditor.rows = score.sections.map(s => ({
      start:time(s.start), duration:time(s.length), fadeIn:time(s.fadeIn), fadeOut:time(s.fadeOut),
      cue:s.cue, interval:String(s.interval), volume:String(s.volume)
    }));
    sectionEditor.render();
    for (const [key, val] of Object.entries({session:current, duration:fileDuration, lengthChoice, seed:seedText,
      cue, cueVolume, cueInterval, sections:JSON.stringify(sectionEditor.rows)})) localStorage.setItem('entrain.' + key, val);
    if (selectedRange()?.error) $('#preview-excerpt').checked = false;
    engine.elapsed = previewStart();
    engine.startedAt = null;
    syncSessionUI();
  }
  var sessionDocument = new EntrainDocument(applyNativeScore);
  sessionDocument.applyFormat = buildSource;
  function filenameStem() { return lastBackground + (cue === 'off' ? '' : '-' + cue); }
  function buildSource() {
    var field = $('#mix-duration'), validTime = durationSeconds(field.value) > 0;
    var sections = cue === 'sections' ? sectionEditor.validate() : [];
    var seed = selectedSeed(lastBackground), valid = validTime && !!sections && Number.isFinite(seed);
    $('#mix-seed').setAttribute('aria-invalid', String(!Number.isFinite(seed)));
    $('#seed-hint').textContent = Number.isFinite(seed) ? 'Blank or 0 plays the original sound.' : 'Enter a whole number from 0 to 2147483647.';
    validControls = valid;
    field.setAttribute('aria-invalid', String(!validTime));
    $('#duration-hint').textContent = !validTime ? 'Enter a positive duration up to 3h, such as 45s or 1h30m.' :
      generatedBeds.has(current) ? 'The CLI and full preview use this duration.' : 'The CLI renders the full session. Preview plays up to 1 min unless you select an excerpt.';
    $('#source-feedback').textContent = '';
    var stem = filenameStem(), filename = stem + '.entrain';
    var cueTitles = {off: '', bell: ' with alternating bells', tone: ' with alternating tones', noise: ' with alternating noise bursts', sections: ' with timed cues'};
    var title = BACKGROUNDS[lastBackground] + cueTitles[cue];
    var lines = ['version = 1', 'title = "' + title + '"', 'output = "wav"', '',
      '[[voice]]', 'type = "soundscape"', 'background = "' + lastBackground + '"', 'duration = "' + EntrainEditor.token(EntrainEditor.timeMS(field.value)) + '"'];
    if (seed > 0) lines.push('seed = ' + seed);
    if (cue === 'sections' && sections) {
      sections.forEach(function (section) {
        var time = EntrainEditor.token;
        lines.push('', '[[voice.cue_section]]', 'start = "' + time(section.start) + '"', 'duration = "' + time(section.length) + '"',
          'cue = "' + section.cue + '"', 'cue_volume_percent = ' + section.volume, 'cue_interval_ms = ' + section.interval,
          'fade_in = "' + time(section.fadeIn) + '"', 'fade_out = "' + time(section.fadeOut) + '"');
      });
    } else if (cue !== 'off' && cue !== 'sections') lines.push('cue = "' + cue + '"', 'cue_volume_percent = ' + cueVolume, 'cue_interval_ms = ' + cueInterval);
    $('#mix-source').rows = Math.min(creatorPage ? 24 : 18, lines.length + 1);
    sessionDocument.update(valid ? lines.join('\n') + '\n' : '', valid);
    $('#source-name').textContent = filename;
    $('#mix-command').textContent = sessionDocument.draft ? './entrain render ' + filename :
      valid ? './entrain render ' + filename + ' -o ' + stem + '.wav' : '';
    var range = selectedRange();
    $('#excerpt-fields').hidden = !range;
    $('#excerpt-hint').textContent = !range ? '' : range.error ? 'Start must be inside the file and length must be positive.' :
      'Audition ' + EntrainEditor.token(range.startMS) + ' to ' + EntrainEditor.token(range.endMS) + '. The saved session stays full length.';
    $('#excerpt-command').textContent = range && !range.error && valid ? './entrain render ' + filename + ' --start ' +
      EntrainEditor.token(range.startMS) + ' -L ' + EntrainEditor.token(range.lengthMS) + (sessionDocument.draft ? '' : ' -o ' + stem + '.excerpt.wav') : '';
  }
  function syncSessionUI() {
    var def = SESSIONS[current], background = !def;
    $('.studio').dataset.builder = String(background);
    document.querySelectorAll('[data-background]').forEach(function (b) {
      b.setAttribute('aria-checked', String(b.dataset.background === current));
      b.tabIndex = b.dataset.background === (background ? current : 'pink') ? 0 : -1;
    });
    document.querySelectorAll('[data-session]').forEach(function (c) {
      c.setAttribute('aria-pressed', String(c.dataset.session === current));
    });
    if (background) {
      lastBackground = current;
      localStorage.setItem('entrain.background', current);
    }
    $('.source-caption').textContent = background ? (creatorPage ?
      'This is the score of your session. Edit it directly, or shape it with the controls.'
      : 'Edit the score and apply it to the preview, or download it for the CLI.')
      : 'While you audition this binaural preset, the editor keeps your ' + BACKGROUNDS[lastBackground].toLowerCase() + ' score. Apply the score to audition it.';
    $('#excerpt-panel').hidden = !background;
    $('#seed-field').hidden = !background || !generatedBeds.has(current);
    if (creatorPage && !$('#seed-field').hidden) $('#seed-field').open = true;
    $('#section-editor').hidden = !background || cue !== 'sections';
    $('#duration-field').hidden = !background;
    $('#cue-shape').disabled = !background;
    $('#cue-shape').value = background ? cue : 'off';
    $('#cue-tuning').hidden = !background || cue === 'off' || cue === 'sections';
    $('#cue-volume-read').value = cueVolume + '%';
    $('#cue-interval-read').value = cueInterval + ' ms';
    $('#cue-volume').setAttribute('aria-valuetext', cueVolume + ' percent');
    $('#cue-interval').setAttribute('aria-valuetext', cueInterval + ' milliseconds');
    $('#session-readouts').hidden = background;
    if (background) {
      $('#t-name').textContent = BACKGROUNDS[current];
      $('#t-sub').textContent = cue === 'sections' ? sectionEditor.rows.length + ' timed cue sections' : cue === 'off' ? 'Background only' : cueNames[cue] + ' · alternating every ' + cueInterval + ' ms';
    } else {
      $('#t-name').textContent = def.name;
      $('#t-sub').textContent = def.sub;
      $('#ro-l').textContent = def.tones[0].l.toFixed(2) + ' Hz';
      $('#ro-r').textContent = def.tones[0].r.toFixed(2) + ' Hz';
      $('#ro-beat').textContent = def.tones[0].beat.toFixed(1) + ' Hz ' + def.tones[0].band;
      $('#ro-bed').textContent = def.bed;
      $('#ro-out').textContent = def.out;
    }
    buildSource();
    localStorage.setItem('entrain.session', current);
    updateStatus();
  }
  function chooseSound(name) {
    if (current === name) return;
    current = name;
    engine.elapsed = previewStart();
    engine.startedAt = null;
    engine.clearDeadline();
    $('#session-details').open = !!SESSIONS[name];
    syncSessionUI();
    reapplyValid();
  }
  document.querySelectorAll('[data-session], [data-background]').forEach(function (button) {
    button.addEventListener('click', function () { chooseSound(button.dataset.session || button.dataset.background); });
  });
  $('.bed-picker').addEventListener('keydown', function (event) {
    var buttons = Array.from(this.querySelectorAll('button')), at = buttons.indexOf(event.target);
    if (at < 0 || !['ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home','End'].includes(event.key)) return;
    event.preventDefault();
    var next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 :
      (at + (['ArrowLeft','ArrowUp'].includes(event.key) ? -1 : 1) + buttons.length) % buttons.length;
    buttons[next].focus();
    buttons[next].click();
  });
  presets.forEach(function (button) {
    button.addEventListener('click', function () {
      var choice = button.dataset.length, field = $('#mix-duration');
      lengthChoice = choice;
      localStorage.setItem('entrain.lengthChoice', choice);
      syncDuration();
      if (choice === 'custom') { field.focus(); field.select(); return; }
      field.value = choice;
      field.dispatchEvent(new Event('input'));
    });
  });
  $('#cue-shape').addEventListener('change', function () {
    cue = this.value;
    localStorage.setItem('entrain.cue', cue);
    syncSessionUI();
    reapplyValid();
  });
  $('#cue-volume').addEventListener('input', function () {
    cueVolume = this.valueAsNumber;
    localStorage.setItem('entrain.cueVolume', String(cueVolume));
    if (engine.ctx) engine.setCueGain();
    syncSessionUI();
  });
  $('#cue-interval').addEventListener('input', function () {
    cueInterval = this.valueAsNumber;
    localStorage.setItem('entrain.cueInterval', String(cueInterval));
    engine.retuneCue();
    syncSessionUI();
  });
  $('#mix-duration').addEventListener('input', function () {
    if (lengthChoice !== 'custom' && durationPreset(this.value) !== lengthChoice) {
      lengthChoice = 'custom';
      localStorage.setItem('entrain.lengthChoice', lengthChoice);
      syncDuration();
    }
    if (durationSeconds(this.value)) {
      fileDuration = this.value;
      localStorage.setItem('entrain.duration', fileDuration);
      engine.scheduleDeadline();
      engine.fillCues();
    }
    buildSource();
    if (engine.playing && (!sourceValid() || selectedRange()?.error)) engine.pause();
    updateStatus();
  });
  function reapplyValid() {
    if (!engine.playing) return;
    if (BACKGROUNDS[current] && (!sourceValid() || selectedRange()?.error)) engine.pause();
    else engine.applySelection();
  }
  function onScheduleEdit() {
    syncSessionUI();
    reapplyValid();
  }
  function rangeChanged() {
    if (engine.playing) engine.pause();
    engine.elapsed = previewStart();
    engine.startedAt = null;
    syncSessionUI();
  }
  function selectSectionRange(row) {
    $('#preview-excerpt').checked = true;
    var entrance = EntrainEditor.timeMS(row.start), total = EntrainEditor.timeMS($('#mix-duration').value);
    var first = Math.max(0, entrance - 5000), last = Math.min(total, entrance + 25000);
    $('#preview-start').value = EntrainEditor.token(first);
    $('#preview-length').value = EntrainEditor.token(last - first);
    rangeChanged();
    playBtn.focus();
  }
  $('#preview-excerpt').addEventListener('change', rangeChanged);
  $('#preview-start').addEventListener('input', rangeChanged);
  $('#preview-length').addEventListener('input', rangeChanged);
  function seedChanged() {
    seedText = $('#mix-seed').value;
    localStorage.setItem('entrain.seed', seedText);
    if (engine.playing) engine.pause();
    syncSessionUI();
  }
  $('#mix-seed').addEventListener('input', seedChanged);
  $('#new-seed').addEventListener('click', function () {
    $('#mix-seed').value = String(1 + (crypto.getRandomValues(new Uint32Array(1))[0] % 2147483647));
    seedChanged();
  });
  $('#mix-copy').addEventListener('click', async function () {
    var source = $('#mix-source');
    try {
      await navigator.clipboard.writeText(source.value);
      $('#source-feedback').textContent = 'Session copied.';
    } catch (err) {
      source.focus();
      source.select();
      $('#source-feedback').textContent = 'Session selected. Press Ctrl+C or Command+C to copy.';
    }
  });
  $('#mix-download').addEventListener('click', function () {
    var url = URL.createObjectURL(new Blob([$('#mix-source').value], {type: 'text/plain;charset=utf-8'}));
    var a = document.createElement('a');
    a.href = url;
    a.download = filenameStem() + '.entrain';
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
    $('#source-feedback').textContent = 'Download started for ' + a.download + '.';
    if (creatorPage) {
      var strip = $('#download-confirm');
      strip.innerHTML = '<strong>' + a.download + ' saved.</strong> Render it: <code>' + $('#mix-command').textContent + '</code>';
      strip.hidden = false;
    }
  });
  function updateStatus(message) {
    var playing = engine.playing, background = !!BACKGROUNDS[current];
    playBtn.setAttribute('aria-pressed', String(playing));
    playBtn.setAttribute('aria-label', playing ? 'Pause session' : 'Play session');
    playBtn.disabled = background && !playing && (!sourceValid() || !!selectedRange()?.error);
    scopeState.classList.toggle('live', playing && !engine.loading);
    scopeState.innerHTML = '<span class="dot"></span>' + (engine.loading ? 'LOADING' : playing ? 'PLAYING' : 'STANDBY');
    statusLine.textContent = message || (engine.loading ? 'Loading the selected sounds…' : playing ?
      (background ? (selectedRange() ? 'Playing excerpt · session position ' + fmt(engine.position()) : generatedBeds.has(current) ? 'Playing the generated mix · ' + fileDuration : 'Playing a stereo preview · up to ' + engine.previewLimit() + ' seconds') : 'Synthesizing stereo audio in your browser') :
      (background && engine.elapsed >= engine.previewLimit() ? 'Preview complete. Press play to restart.' :
      engine.elapsed > previewStart() ? 'Paused. Press play to resume.' : 'Press play to begin.'));
    $('#elapsed').textContent = fmt(engine.position());
  }
  playBtn.addEventListener('click', function () {
    if (engine.playing) engine.pause();
    else {
      try { engine.play(); } catch (err) {
        if (engine.playing) engine.pause();
        updateStatus('Audio playback is unavailable in this browser.');
      }
    }
  });
  var vol = $('#volume');
  vol.addEventListener('input', function () {
    $('#vol-read').textContent = vol.value + '%';
    localStorage.setItem('entrain.vol', vol.value);
    engine.setVolume(vol.valueAsNumber);
  });
  var savedVol = localStorage.getItem('entrain.vol');
  if (savedVol !== null) { vol.value = savedVol; $('#vol-read').textContent = savedVol + '%'; }
  var handoffChip = $('#handoff-chip');
  if (creatorPage && arriving && handoffChip) {
    $('#handoff-text').innerHTML = handoff && handoff.brown ?
      'Continuing from the Studio — <strong>sleep-delta</strong> renders with brown noise offline, so the preview uses pink.' :
      handoff ? 'Continuing from the Studio — the bed behind <strong>' + handoff.preset + '</strong>.' :
      'Continuing <strong>' + BACKGROUNDS[current] + '</strong> from the Studio.';
    handoffChip.hidden = false;
    $('#handoff-reset').addEventListener('click', function () {
      current = 'pink';
      $('#mix-duration').value = '6m';
      lengthChoice = '6m';
      syncDuration();
      $('#mix-duration').dispatchEvent(new Event('input'));
      handoffChip.hidden = true;
      syncSessionUI();
    });
  }
  setInterval(function () {
    var track = document.querySelector('#section-timeline .tl-track');
    if (track) {
      var head = track.querySelector('.tl-playhead');
      var total = EntrainEditor.timeMS($('#mix-duration').value);
      if (engine.playing && total > 0) {
        head.hidden = false;
        head.style.left = Math.min(100, engine.position() * 1000 / total * 100) + '%';
      } else head.hidden = true;
    }
    if (engine.playing) $('#elapsed').textContent = fmt(engine.position());
  }, 250);
  syncSessionUI();

  /* ============ Scope visualizer ============ */
  var scope = $('#scope-canvas'), sctx = scope.getContext('2d');
  var freqData, waveData;
  var fitCanvas = window.entrainFitCanvas;
  var NBARS = 56;
  function drawScope() {
    var dpr = fitCanvas(scope);
    if (!dpr) return;
    var W = scope.width, H = scope.height, mid = H / 2;
    sctx.clearRect(0, 0, W, H);
    /* grid */
    sctx.strokeStyle = 'rgba(25,23,19,0.10)'; sctx.lineWidth = 1;
    for (var gi = 1; gi < 5; gi++) { var gy = (H / 5) * gi; sctx.beginPath(); sctx.moveTo(0, gy); sctx.lineTo(W, gy); sctx.stroke(); }
    var live = engine.playing && engine.analyser;
    var i, x, bw = W / NBARS;
    if (live) {
      if (!freqData) { freqData = new Uint8Array(engine.analyser.frequencyBinCount); waveData = new Uint8Array(engine.analyser.fftSize); }
      engine.analyser.getByteFrequencyData(freqData);
      var sr = engine.ctx.sampleRate, binHz = sr / engine.analyser.fftSize;
      var fMin = 30, fMax = 600;
      for (i = 0; i < NBARS; i++) {
        var f = fMin * Math.pow(fMax / fMin, i / (NBARS - 1));
        var f2 = fMin * Math.pow(fMax / fMin, (i + 1) / (NBARS - 1));
        var b0 = Math.max(1, Math.floor(f / binHz)), b1 = Math.max(b0 + 1, Math.floor(f2 / binHz));
        var peak = 0;
        for (var b = b0; b < b1 && b < freqData.length; b++) peak = Math.max(peak, freqData[b]);
        var h = (peak / 255) * (H * 0.86);
        var g = sctx.createLinearGradient(0, H - h, 0, H);
        g.addColorStop(0, 'rgba(194,65,12,0.95)'); g.addColorStop(1, 'rgba(194,65,12,0.28)');
        sctx.fillStyle = g;
        sctx.fillRect(i * bw + bw * 0.18, H - h, bw * 0.64, h);
      }
      engine.analyser.getByteTimeDomainData(waveData);
      sctx.beginPath();
      var step = waveData.length / W;
      for (i = 0; i < W; i++) {
        var v = waveData[Math.floor(i * step)] / 128 - 1;
        var y = mid + v * H * 0.32;
        i === 0 ? sctx.moveTo(i, y) : sctx.lineTo(i, y);
      }
      sctx.strokeStyle = 'rgba(25,23,19,0.65)'; sctx.lineWidth = 1.2 * dpr; sctx.stroke();
    } else {
      /* idle: baseline + faint carrier ticks */
      sctx.strokeStyle = 'rgba(111,105,90,0.4)'; sctx.lineWidth = 1 * dpr;
      sctx.beginPath(); sctx.moveTo(0, mid); sctx.lineTo(W, mid); sctx.stroke();
      sctx.fillStyle = 'rgba(194,65,12,0.35)';
      for (i = 0; i < NBARS; i++) sctx.fillRect(i * bw + bw * 0.18, mid - 1, bw * 0.64, 2);
    }
  }
  function loopScope() { drawScope(); requestAnimationFrame(loopScope); }
  if (!reducedMotion) requestAnimationFrame(loopScope); else drawScope();

  if (reducedMotion) window.addEventListener('resize', drawScope);
})();

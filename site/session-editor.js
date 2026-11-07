// Support the same sample clock and cue-section boundaries as the native renderer.
(() => {
  'use strict';
  const maxMS = 10800000;
  function timeMS(value) {
    const text = String(value).trim();
    if (!text) return NaN;
    if (/^\d+(?:\.\d+)?$/.test(text)) return bounded(Number(text) * 1000);
    let at = 0, total = 0;
    const units = {h: 3600000, m: 60000, s: 1000, ms: 1};
    for (const part of text.matchAll(/(\d+(?:\.\d+)?)(ms|[hms])/g)) {
      if (part.index !== at) return NaN;
      total += Number(part[1]) * units[part[2]];
      at += part[0].length;
    }
    return at === text.length ? bounded(total) : NaN;
  }
  function bounded(ms) {
    return ms <= maxMS && Math.abs(ms - Math.round(ms)) < 0.000001 ? Math.round(ms) : NaN;
  }
  function token(ms) {
    if (ms === 0) return '0s';
    if (ms % 3600000 === 0) return ms / 3600000 + 'h';
    if (ms % 60000 === 0) return ms / 60000 + 'm';
    if (ms % 1000 === 0) return ms / 1000 + 's';
    return ms + 'ms';
  }
  const keys = ['start','duration','cue','volume','interval','fadeIn','fadeOut'];
  // The proportional timeline is a creator-only visual; the Studio keeps its player focus.
  const timelineTrack = () => document.body.dataset.page === 'session-creator' ?
    document.querySelector('#section-timeline .tl-track') : null;
  class Sections {
    constructor(change, audition, duration) {
      this.change = change;
      this.audition = audition;
      this.duration = duration;
      this.list = document.getElementById('section-list');
      this.add = document.getElementById('add-section');
      this.track = timelineTrack();
      let stored;
      try { stored = JSON.parse(localStorage.getItem('entrain.sections') || '[]'); } catch { /* Start with an empty schedule. */ }
      this.rows = Array.isArray(stored) ? stored.filter(row => row && keys.every(k => typeof row[k] === 'string')) : [];
      this.render();
      this.add.addEventListener('click', () => {
        const previous = this.rows.at(-1);
        const start = previous ? timeMS(previous.start) + timeMS(previous.duration) : 0;
        const length = Math.min(15000, this.duration() - start);
        if (!(length > 0)) return;
        const fade = token(Math.min(250, Math.floor(length / 2)));
        this.rows.push({start:token(start), duration:token(length), cue:'bell', volume:'100', interval:'625', fadeIn:fade, fadeOut:fade});
        this.render();
        this.changed();
        this.list.lastElementChild.querySelector('input').focus();
      });
      this.list.addEventListener('input', event => {
        const field = event.target.dataset.field;
        if (!field) return;
        this.rows[Number(event.target.closest('fieldset').dataset.index)][field] = event.target.value;
        this.changed();
      }, true);
      this.list.addEventListener('click', event => {
        const action = event.target.dataset.action;
        if (!action) return;
        const i = Number(event.target.closest('fieldset').dataset.index);
        if (action === 'audition') { this.audition(this.rows[i]); return; }
        this.rows.splice(i, 1);
        this.render();
        this.changed();
        (this.list.children[i]?.querySelector('input') || this.add).focus();
      });
    }
    changed() {
      localStorage.setItem('entrain.sections', JSON.stringify(this.rows));
      this.change();
    }
    render() {
      this.list.replaceChildren();
      this.rows.forEach((row, i) => {
        const fragment = document.getElementById('cue-row-template').content.cloneNode(true);
        fragment.querySelector('fieldset').dataset.index = i;
        fragment.querySelector('legend').textContent = 'Section ' + (i + 1);
        for (const el of fragment.querySelectorAll('[data-field]')) el.value = row[el.dataset.field];
        this.list.append(fragment);
      });
      this.renderTimeline();
    }
    renderTimeline() {
      if (!this.track) return;
      const wrap = this.track.closest('#section-timeline');
      const total = this.duration();
      wrap.classList.toggle('empty', !this.rows.length || !(total > 0));
      for (const el of this.track.querySelectorAll('.tl-block')) el.remove();
      if (!(total > 0)) return;
      this.rows.forEach((row, i) => {
        const start = timeMS(row.start), length = timeMS(row.duration);
        if (!(start >= 0) || !(length > 0)) return;
        const block = document.createElement('div');
        block.className = 'tl-block';
        block.style.left = Math.min(100, start / total * 100) + '%';
        block.style.width = Math.max(1.2, Math.min(100, length / total * 100)) + '%';
        block.title = 'Section ' + (i + 1) + ' · ' + row.cue;
        block.textContent = String(i + 1);
        block.addEventListener('click', () => this.focusRow(i));
        this.track.append(block);
      });
    }
    focusRow(i) {
      const fields = this.list.children[i];
      if (!fields) return;
      fields.scrollIntoView({block: 'nearest', behavior: 'smooth'});
      const first = fields.querySelector('input');
      if (first) first.focus({preventScroll: true});
    }
    validate() {
      const result = [], errors = [];
      let end = 0, fileMS = this.duration();
      this.rows.forEach((row, i) => {
        const fields = this.list.children[i];
        fields.querySelectorAll('[data-field]').forEach(el => el.removeAttribute('aria-invalid'));
        function error(field, reason) {
          fields.querySelector('[data-field="' + field + '"]').setAttribute('aria-invalid', 'true');
          errors.push('Section ' + (i + 1) + ': ' + reason);
        }
        const start = timeMS(row.start), length = timeMS(row.duration);
        const fadeIn = timeMS(row.fadeIn), fadeOut = timeMS(row.fadeOut);
        const volume = row.volume.trim() ? Number(row.volume) : NaN;
        const interval = row.interval.trim() ? Number(row.interval) : NaN;
        if (!(start >= 0)) error('start', 'enter a start time such as 0s or 1m15s.');
        else if (start < end) error('start', 'starts before the previous section ends at ' + token(end) + '.');
        if (!(length > 0)) error('duration', 'duration must be at least 1ms.');
        else if (start + length > fileMS) error('duration', 'ends after the file duration.');
        if (!['bell','tone','noise'].includes(row.cue)) error('cue', 'choose a cue sound.');
        if (!(volume >= 0 && volume <= 200)) error('volume', 'volume must be between 0 and 200%.');
        if (!Number.isInteger(interval) || interval < 350 || interval > 2000) error('interval', 'interval must be 350–2000 whole milliseconds.');
        if (!(fadeIn >= 0)) error('fadeIn', 'enter a nonnegative fade duration.');
        if (!(fadeOut >= 0)) error('fadeOut', 'enter a nonnegative fade duration.');
        if (fadeIn + fadeOut > length) error('fadeOut', 'the two fades must fit inside this section.');
        end = start + length;
        result.push({start, length, fadeIn, fadeOut, cue:row.cue, volume, interval});
      });
      this.add.disabled = !!errors.length || !(fileMS > end);
      for (const button of this.list.querySelectorAll('[data-action="audition"]')) button.disabled = !!errors.length;
      this.renderTimeline();
      document.getElementById('section-message').textContent = errors.join(' ') ||
        (!this.rows.length ? 'No sections yet. The background plays alone.' : end >= fileMS ? 'The final section reaches the end of the session.' : '');
      return errors.length ? null : result;
    }
  }

  // Queue a bounded window of strikes. Section boundaries and cosine tapers
  // are rounded to the same first sample at/after an instant as the Go code.
  class Timeline {
    constructor(ctx, destination, sections, samples, position, end, origin) {
      this.ctx = ctx;
      this.sections = sections;
      this.samples = samples;
      this.origin = origin;
      this.notBefore = origin + position;
      this.end = end;
      this.gain = ctx.createGain();
      this.gain.connect(destination);
      this.channels = sections.map(section => {
        const level = ctx.createGain();
        level.gain.value = section.volume / 100;
        level.connect(this.gain);
        return level;
      });
      this.events = new Set();
      this.next = sections.map(s => Math.max(0, Math.floor((position * 1000 - s.start) / s.interval)));
      this.fill();
    }
    setEnd(end) {
      this.end = end;
      this.fill();
    }
    fill() {
      this.cancelWake();
      const now = this.ctx.currentTime, horizon = Math.min(this.origin + this.end, now + 120);
      this.sections.forEach((sec, i) => {
        const buffer = this.samples.get(sec.cue);
        if (!buffer || sec.volume === 0) return;
        const rate = buffer.sampleRate, frame = ms => Math.ceil(ms * rate / 1000);
        const start = frame(sec.start), end = frame(sec.start + sec.length);
        const fi = frame(sec.fadeIn), fo = frame(sec.fadeOut);
        let n = this.next[i];
        for (;; n++) {
          const rel = frame(n * sec.interval), at = start + rel, onset = this.origin + at / rate;
          if (at >= end || onset >= horizon) break;
          const first = frame(n % 16 * 625), last = frame((n % 16 + 1) * 625);
          const count = Math.min(last - first, end - at, frame((n+1) * sec.interval) - rel);
          const when = Math.max(now, this.notBefore, onset);
          const skipped = Math.max(0, Math.round((when - onset) * rate));
          if (skipped >= count) continue;
          let clip = buffer, offset = first + skipped;
          if (rel + skipped < fi || end - at - count < fo) {
            clip = this.ctx.createBuffer(2, count - skipped, rate);
            for (let ch = 0; ch < 2; ch++) {
              const input = buffer.getChannelData(ch), out = clip.getChannelData(ch);
              for (let f = 0; f < out.length; f++) {
                const elapsed = rel + skipped + f, remaining = end - at - skipped - f - 1;
                const x = elapsed < fi ? elapsed / fi : remaining < fo ? remaining / fo : 1;
                out[f] = input[offset + f] * (0.5 - 0.5 * Math.cos(Math.PI * x));
              }
            }
            offset = 0;
          }
          const source = this.ctx.createBufferSource();
          source.buffer = clip;
          source.connect(this.channels[i]);
          this.events.add(source);
          source.onended = () => this.events.delete(source);
          source.start(when, offset / rate, (count - skipped) / rate);
        }
        this.next[i] = n;
      });
      if (horizon < this.origin + this.end) {
        this.wake = this.ctx.createBufferSource();
        this.wake.buffer = this.ctx.createBuffer(1, 1, this.ctx.sampleRate);
        this.wake.onended = () => { this.wake = null; this.fill(); };
        this.wake.start(now + 60);
      }
    }
    cancelWake() {
      if (!this.wake) return;
      this.wake.onended = null; this.wake.stop(); this.wake = null;
    }
    stop() {
      this.cancelWake();
      const now = this.ctx.currentTime, end = now + .03;
      this.gain.gain.setValueAtTime(this.gain.gain.value, now);
      this.gain.gain.linearRampToValueAtTime(0, end);
      for (const node of this.events) node.stop(end);
      const cleanup = this.ctx.createBufferSource();
      cleanup.buffer = this.ctx.createBuffer(1, 1, this.ctx.sampleRate);
      cleanup.onended = () => { this.gain.disconnect(); this.channels.forEach(level => level.disconnect()); };
      cleanup.start(end);
    }
  }
  window.EntrainEditor = {timeMS, token, Sections, Timeline};
})();

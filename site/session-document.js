// Control-generated scores follow the form. Once the listener edits the text,
// the draft owns its formatting until they explicitly return to the controls.
class EntrainDocument {
  constructor(apply) {
    this.apply = apply;
    this.source = document.getElementById('mix-source');
    this.status = document.getElementById('document-status');
    this.confirm = document.getElementById('source-confirm');
    this.key = 'entrain.sourceDocument.v1';
    let saved;
    try { saved = JSON.parse(localStorage.getItem(this.key)); } catch { /* A new draft starts from the form. */ }
    this.draft = saved && typeof saved.text === 'string' ? saved : null;
    this.revision = 0;
    this.source.addEventListener('input', () => {
      this.revision++;
      this.draft = {...this.draft, text:this.source.value};
      this.error = '';
      this.confirm.hidden = true;
      this.persist();
      this.applyFormat?.();
      this.refresh();
    });
    document.getElementById('apply-source').addEventListener('click', () => this.applyText());
    document.getElementById('use-controls').addEventListener('click', () => {
      this.confirm.hidden = false;
      document.getElementById('cancel-control-source').focus();
    });
    document.getElementById('cancel-control-source').addEventListener('click', () => {
      this.confirm.hidden = true;
      document.getElementById('use-controls').focus();
    });
    document.getElementById('confirm-control-source').addEventListener('click', () => {
      this.revision++;
      this.draft = null;
      this.error = '';
      this.confirm.hidden = true;
      this.persist();
      this.update(this.canonical, this.valid);
      this.applyFormat?.();
      this.source.focus();
    });
  }
  persist() {
    try {
      if (this.draft) localStorage.setItem(this.key, JSON.stringify(this.draft));
      else localStorage.removeItem(this.key);
      this.storageError = false;
    } catch { this.storageError = true; }
  }
  update(canonical, valid) {
    this.canonical = canonical;
    this.valid = valid;
    const text = this.draft ? this.draft.text : canonical;
    if (this.source.value !== text) this.source.value = text;
    this.refresh();
  }
  refresh() {
    const draft = this.draft;
    document.getElementById('document-tools').hidden = !draft;
    document.getElementById('use-controls').disabled = !this.valid;
    document.getElementById('apply-source').disabled = !draft || !!this.applying || !this.source.value.trim();
    const disabled = draft ? !this.source.value.length : !this.valid;
    document.getElementById('mix-download').disabled = disabled;
    document.getElementById('mix-copy').disabled = disabled;
    this.source.setAttribute('aria-invalid', String(!!this.error && !this.unsupported));
    const message = this.error || (this.applying ? 'Checking source…' : !draft ? '' :
      draft.text !== draft.applied ? 'Edited source. Apply to update the preview. Copy and Download use the text shown.' :
      draft.anchor !== this.canonical ? 'Controls differ from this source. Apply to restore its settings, or use the controls to replace the text.' :
      'Source applied. Press Play to audition it.');
    this.status.textContent = message + (this.storageError ? ' Local storage is full. Download to keep this draft.' : '');
    this.status.hidden = !this.status.textContent;
  }
  async applyText() {
    const revision = this.revision, text = this.source.value;
    this.applying = true;
    this.error = '';
    this.unsupported = false;
    this.refresh();
    try {
      if (!window.EntrainSoundscapes?.parseSession) throw new Error('The local validator is unavailable. Check the CLI command below.');
      const score = await window.EntrainSoundscapes.parseSession(text);
      if (revision !== this.revision) return;
      this.apply(score);
      this.draft = {text, applied:text, anchor:this.canonical};
      this.persist();
      // Rebuild displayed render instructions after learning the output format.
      this.applyFormat?.();
    } catch (error) {
      if (revision !== this.revision) return;
      this.unsupported = error.name === 'UnsupportedPreview';
      this.error = this.unsupported ? error.message : 'Could not apply source. ' + error.message;
    } finally {
      this.applying = false;
      this.refresh();
    }
  }
}

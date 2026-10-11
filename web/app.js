(() => {
  const input = document.querySelector('#file-input');
  const zone = document.querySelector('#drop-zone');
  const browse = document.querySelector('#browse');
  const button = document.querySelector('#convert');
  const form = document.querySelector('#convert-form');
  const error = document.querySelector('#error');
  const deleteButton = document.querySelector('#delete-generated');
  const cleanupMessage = document.querySelector('#cleanup-message');
  let file = null;
  let openURL = '';

  function resetConvertButton() {
    button.type = 'submit';
    button.dataset.mode = 'convert';
    button.textContent = 'Convert to HTML';
    openURL = '';
  }

  function choose(candidate) {
    error.hidden = true;
    if (!candidate) return;
    resetConvertButton();
    document.querySelector('#result').hidden = true;
    if (!candidate.name.toLowerCase().endsWith('.txt')) {
      file = null; button.disabled = true; error.textContent = 'Please choose a .txt file.'; error.hidden = false; return;
    }
    if (candidate.size > 5 * 1024 * 1024) {
      file = null; button.disabled = true; error.textContent = 'The file must be 5 MB or smaller.'; error.hidden = false; return;
    }
    file = candidate; button.disabled = false;
    document.querySelector('#file-name').textContent = `${candidate.name} · ${(candidate.size / 1024).toFixed(1)} KB`;
  }

  browse.addEventListener('click', () => input.click());
  zone.addEventListener('click', e => { if (e.target !== browse) input.click(); });
  zone.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); input.click(); } });
  input.addEventListener('change', () => choose(input.files[0]));
  button.addEventListener('click', e => {
    if (button.dataset.mode === 'open' && openURL) {
      e.preventDefault();
      window.open(openURL, '_blank', 'noopener');
    }
  });
  for (const name of ['dragenter', 'dragover']) zone.addEventListener(name, e => { e.preventDefault(); zone.classList.add('dragover'); });
  for (const name of ['dragleave', 'drop']) zone.addEventListener(name, e => { e.preventDefault(); zone.classList.remove('dragover'); });
  zone.addEventListener('drop', e => choose(e.dataTransfer.files[0]));

  deleteButton.addEventListener('click', async () => {
    if (!confirm('Delete all generated HTML files? This cannot be undone.')) return;
    deleteButton.disabled = true; cleanupMessage.textContent = 'Deleting…';
    try {
      const response = await fetch('/delete-generated', { method: 'POST' });
      if (!response.ok) throw new Error('Could not delete generated files.');
      const result = await response.json();
      document.querySelector('#result').hidden = true;
      resetConvertButton();
      button.disabled = !file;
      cleanupMessage.textContent = `Deleted ${result.deleted} generated HTML file${result.deleted === 1 ? '' : 's'}.`;
    } catch (err) {
      cleanupMessage.textContent = err.message;
    } finally {
      deleteButton.disabled = false;
    }
  });

  form.addEventListener('submit', async e => {
    e.preventDefault(); if (!file) return;
    resetConvertButton();
    button.disabled = true; error.hidden = true; document.querySelector('#result').hidden = true;
    button.dataset.mode = 'converting';
    setProgress(0);
    const data = new FormData(); data.append('file', file, file.name);
    data.append('encoding', document.querySelector('#encoding').value);
    let jobId;
    try {
      const xhr = new XMLHttpRequest();
      const upload = new Promise((resolve, reject) => {
        xhr.open('POST', '/convert');
        xhr.upload.onprogress = ev => { if (ev.lengthComputable) setProgress(Math.min(14, Math.round(ev.loaded / ev.total * 14))); };
        xhr.onload = () => xhr.status >= 200 && xhr.status < 300 ? resolve(JSON.parse(xhr.responseText)) : reject(new Error(xhr.responseText || 'Upload failed.'));
        xhr.onerror = () => reject(new Error('Network error while uploading.'));
        xhr.send(data);
      });
      ({ jobId } = await upload);
      await poll(jobId);
    } catch (err) {
      resetConvertButton();
      error.textContent = err.message.replace(/^.*?:\s*/, '').replace(/[<>]/g, ''); error.hidden = false; button.disabled = false;
    }
  });

  function setProgress(value) {
    button.textContent = `Converting ${Math.max(0, Math.min(99, value))}%`;
  }
  async function poll(id) {
    const response = await fetch(`/status/${encodeURIComponent(id)}`);
    if (!response.ok) throw new Error('Could not read conversion status.');
    const state = await response.json();
    if (state.error) throw new Error(state.error);
    if (state.done) {
      openURL = `/result/${encodeURIComponent(id)}/open`;
      button.type = 'button';
      button.dataset.mode = 'open';
      button.textContent = 'Open converted HTML';
      button.disabled = false;
      document.querySelector('#result').hidden = false;
      document.querySelector('#download-link').href = `/result/${encodeURIComponent(id)}/download`;
      return;
    }
    setProgress(state.progress);
    return setTimeout(() => poll(id).catch(showError), 300);
  }
  function showError(err) {
    resetConvertButton();
    error.textContent = err.message;
    error.hidden = false;
    button.disabled = false;
  }
})();

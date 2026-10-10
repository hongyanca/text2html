(() => {
  const input = document.querySelector('#file-input');
  const zone = document.querySelector('#drop-zone');
  const browse = document.querySelector('#browse');
  const button = document.querySelector('#convert');
  const form = document.querySelector('#convert-form');
  const error = document.querySelector('#error');
  const progress = document.querySelector('#progress-panel');
  const deleteButton = document.querySelector('#delete-generated');
  const cleanupMessage = document.querySelector('#cleanup-message');
  let file = null;

  function choose(candidate) {
    error.hidden = true;
    if (!candidate) return;
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
      cleanupMessage.textContent = `Deleted ${result.deleted} generated HTML file${result.deleted === 1 ? '' : 's'}.`;
    } catch (err) {
      cleanupMessage.textContent = err.message;
    } finally {
      deleteButton.disabled = false;
    }
  });

  form.addEventListener('submit', async e => {
    e.preventDefault(); if (!file) return;
    button.disabled = true; error.hidden = true; document.querySelector('#result').hidden = true;
    progress.hidden = false; setProgress(0, 'Uploading', 'Uploading your file…');
    const data = new FormData(); data.append('file', file, file.name);
    let jobId;
    try {
      const xhr = new XMLHttpRequest();
      const upload = new Promise((resolve, reject) => {
        xhr.open('POST', '/convert');
        xhr.upload.onprogress = ev => { if (ev.lengthComputable) setProgress(Math.min(14, Math.round(ev.loaded / ev.total * 14)), 'Uploading', 'Uploading your file…'); };
        xhr.onload = () => xhr.status >= 200 && xhr.status < 300 ? resolve(JSON.parse(xhr.responseText)) : reject(new Error(xhr.responseText || 'Upload failed.'));
        xhr.onerror = () => reject(new Error('Network error while uploading.'));
        xhr.send(data);
      });
      ({ jobId } = await upload);
      await poll(jobId);
    } catch (err) {
      progress.hidden = true; error.textContent = err.message.replace(/^.*?:\s*/, '').replace(/[<>]/g, ''); error.hidden = false; button.disabled = false;
    }
  });

  function setProgress(value, stage, message) {
    document.querySelector('#bar').style.width = `${value}%`;
    document.querySelector('#percent').textContent = `${value}%`;
    document.querySelector('#stage').textContent = stage;
    document.querySelector('#status-message').textContent = message;
  }
  async function poll(id) {
    const response = await fetch(`/status/${encodeURIComponent(id)}`);
    if (!response.ok) throw new Error('Could not read conversion status.');
    const state = await response.json();
    setProgress(state.progress, state.stage, state.error || (state.done ? 'Conversion complete.' : `${state.stage}…`));
    if (state.error) throw new Error(state.error);
    if (!state.done) return setTimeout(() => poll(id).catch(showError), 300);
    document.querySelector('#open-link').href = `/result/${encodeURIComponent(id)}/open`;
    document.querySelector('#download-link').href = `/result/${encodeURIComponent(id)}/download`;
    document.querySelector('#result').hidden = false;
  }
  function showError(err) { error.textContent = err.message; error.hidden = false; button.disabled = false; }
})();

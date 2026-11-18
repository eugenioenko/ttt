import { installFilesystem } from './filesystem.js';

const loader = document.getElementById('loader');
const status = document.getElementById('loading-status');
const progress = document.getElementById('loading-progress');
const retry = document.getElementById('retry');

globalThis.tttDemoError = (error) => {
  status.textContent = 'The demo could not start. Please try again.';
  document.getElementById('loading-detail').textContent = String(error?.message ?? error);
  loader.hidden = false;
  loader.dataset.failed = 'true';
  progress.hidden = true;
  retry.hidden = false;
  console.error(error);
};
globalThis.tttDemoReady = () => {
  loader.hidden = true;
  document.getElementById('terminal').focus();
};
retry.addEventListener('click', () => location.reload());

async function downloadAsset(name, onProgress = () => {}) {
  const response = await fetch(name);
  if (!response.ok) throw new Error(`Download failed (${response.status})`);
  const total = response.headers.get('content-encoding') ? 0 : Number(response.headers.get('content-length'));
  const reader = response.body.getReader();
  const chunks = [];
  let received = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    received += value.length;
    onProgress(received, total);
  }
  const blob = new Blob(chunks);
  // Some static servers decode .gz responses through Content-Encoding themselves.
  const magic = new Uint8Array(await blob.slice(0, 2).arrayBuffer());
  return magic[0] === 0x1f && magic[1] === 0x8b
    ? new Response(blob.stream().pipeThrough(new DecompressionStream('gzip'))).arrayBuffer()
    : blob.arrayBuffer();
}

globalThis.tttStart = async (go) => {
  try {
    const [wasm, files] = await Promise.all([
      downloadAsset('main.wasm.gz', (received, total) => {
        if (total > 0) {
          progress.value = received / total;
          status.textContent = `Loading editor… ${Math.round(received / total * 100)}%`;
        } else {
          status.textContent = `Loading editor… ${(received / 1048576).toFixed(1)} MB`;
        }
      }),
      downloadAsset('workspace.json.gz').then((data) => JSON.parse(new TextDecoder().decode(data))),
    ]);
    status.textContent = 'Preparing editor…';
    progress.removeAttribute('value');
    await installFilesystem(files);
    go.argv = ['ttt-web'];
    go.env = { HOME: '/home/demo', PWD: '/ttt', TERM: 'xterm-256color' };
    go.exit = (code) => globalThis.tttDemoError(`The editor stopped (code ${code}). Reset to start a new session.`);
    const result = await WebAssembly.instantiate(wasm, go.importObject);
    await go.run(result.instance);
  } catch (error) {
    globalThis.tttDemoError(error);
  }
};

import('./tcell.js').catch(globalThis.tttDemoError);

window.addEventListener('keydown', (event) => {
  if (event.key === 'F6') {
    event.preventDefault();
    event.stopImmediatePropagation();
    window.parent.postMessage({ type: 'ttt-focus-docs' }, location.origin);
  }
}, true);

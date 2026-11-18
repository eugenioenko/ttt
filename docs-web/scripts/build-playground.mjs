import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';
import { build } from 'esbuild';

const docs = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const root = resolve(docs, '..');
const output = join(docs, 'public/playground');
const copy = (source, target) => writeFileSync(target, readFileSync(source));
mkdirSync(output, { recursive: true });
const go = (args, options = {}) => execFileSync('go', args, { cwd: root, ...options });
go(['build', '-trimpath', '-ldflags=-s -w', '-o', join(output, 'main.wasm'), './cmd/ttt-web'], {
  env: { ...process.env, GOOS: 'js', GOARCH: 'wasm', CGO_ENABLED: '0' },
  stdio: 'inherit',
});
const wasm = readFileSync(join(output, 'main.wasm'));
writeFileSync(join(output, 'main.wasm.gz'), gzipSync(wasm, { level: 9 }));
rmSync(join(output, 'main.wasm'));
const goroot = go(['env', 'GOROOT'], { encoding: 'utf8' }).trim();
const tcell = go(['list', '-m', '-f', '{{.Dir}}', 'github.com/gdamore/tcell/v3'], { encoding: 'utf8' }).trim();
copy(join(goroot, 'lib/wasm/wasm_exec.js'), join(output, 'wasm_exec.js'));
mkdirSync(join(output, 'ghostty-web'), { recursive: true });
for (const name of ['ghostty-web.js', 'ghostty-vt.wasm', 'LICENSE']) {
  writeFileSync(join(output, 'ghostty-web', name), readFileSync(join(tcell, 'webfiles/ghostty-web', name)));
}
copy(join(tcell, 'webfiles/beep.wav'), join(output, 'beep.wav'));
copy(join(tcell, 'LICENSE'), join(output, 'TCELL-LICENSE'));
let frontend = readFileSync(join(tcell, 'webfiles/tcell.js'), 'utf8');
const startup = frontend.lastIndexOf('const go = new Go();');
if (startup < 0) throw new Error('The tcell browser startup changed; update the playground adapter.');
frontend = frontend.slice(0, startup) + 'await globalThis.tttStart(new Go());\n';
const fontHook = `  fontFamily: '"Menlo", "Andale Mono", "Courier New", monospace',`;
const terminalHook = 'const term = new Terminal({';
if (!frontend.includes(fontHook) || !frontend.includes(terminalHook)) throw new Error('The tcell browser font setup changed; update the playground adapter.');
frontend = frontend.replace(fontHook, `  fontFamily: '"JetBrains Mono", monospace',`);
frontend = frontend.replace(terminalHook, `await Promise.all(['normal 400', 'normal 700', 'italic 400', 'italic 700'].map(
  style => document.fonts.load(style + ' 14px "JetBrains Mono"')
));
${terminalHook}`);
const fitHook = '  fit.fit();';
if (!frontend.includes(fitHook)) throw new Error('The tcell browser sizing changed; update the playground adapter.');
frontend = frontend.replace(fitHook, `  const metrics = term.renderer?.getMetrics?.();
  if (!metrics?.width || !metrics?.height) return;
  const maxCols = positiveInt(container.dataset.maxCols) ?? Infinity;
  const maxRows = positiveInt(container.dataset.maxRows) ?? Infinity;
  const cols = Math.max(2, Math.min(Math.floor(container.clientWidth / metrics.width), maxCols));
  const rows = Math.max(1, Math.min(Math.floor(container.clientHeight / metrics.height), maxRows));
  if (term.cols !== cols || term.rows !== rows) term.resize(cols, rows);
  if (Number.isFinite(maxCols)) {
    window.parent.postMessage({ type: 'ttt-demo-width', width: maxCols * metrics.width }, location.origin);
  }`);
frontend = frontend.replace(/function showFatal\(error\) \{[\s\S]*?\n\}/, 'function showFatal(error) {\n  globalThis.tttDemoError(error);\n}');
writeFileSync(join(output, 'tcell.js'), frontend);

const files = {};
const names = execFileSync('git', ['ls-files', '-z'], { cwd: root, encoding: 'utf8' }).split('\0').filter(Boolean);
for (const name of names) {
  if (!/\.(go|lua|json|toml|ya?ml|mdx?|[cm]?js|ts|astro|css|mod|sum|txt)$/.test(name) && !['LICENSE', 'Makefile', '.gitignore'].includes(name)) continue;
  const data = readFileSync(join(root, name));
  if (data.length <= 512 * 1024 && !data.includes(0)) files[name] = data.toString('utf8');
}
writeFileSync(join(output, 'workspace.json.gz'), gzipSync(JSON.stringify(files), { level: 9 }));
for (const name of ['index.html', 'playground.css']) copy(join(docs, 'playground', name), join(output, name));
await build({
  entryPoints: [join(docs, 'playground/host.js')],
  outfile: join(output, 'host.js'),
  bundle: true,
  format: 'esm',
  platform: 'browser',
  external: ['./tcell.js'],
  minify: true,
  legalComments: 'linked',
});
console.log(`Playground: ${Object.keys(files).length} source files; ${(wasm.length / 1048576).toFixed(1)} MiB WASM before compression.`);

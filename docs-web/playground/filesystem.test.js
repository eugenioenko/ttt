import { test } from 'node:test';
import assert from 'node:assert/strict';
import { installFilesystem } from './filesystem.js';

test('Go filesystem bridge reads directories and saves Unicode edits using byte offsets', async () => {
  const original = { fs: globalThis.fs, path: globalThis.path, process: globalThis.process };
  try {
    await installFilesystem({ 'README.md': 'hello', 'internal/example.go': 'package example\n' });
    const { fs } = globalThis;
    const call = (name, ...args) => new Promise((resolve, reject) => {
      fs[name](...args, (error, value) => error ? reject(error) : resolve(value));
    });
    const folder = await call('open', '/ttt', fs.constants.O_RDONLY, 0o755);
    assert.ok(folder > 2);
    const stat = await call('fstat', folder);
    assert.ok(stat.isDirectory());
    assert.deepEqual((await call('readdir', '/ttt')).sort(), ['README.md', 'internal']);
    await call('close', folder);
    const fd = await call('open', '/ttt/README.md', fs.constants.O_RDWR | fs.constants.O_TRUNC, 0o644);
    const text = new TextEncoder().encode('你好, editor!\n');
    assert.equal(await call('write', fd, text, 0, text.length, 0), text.length);
    const read = new Uint8Array(text.length);
    assert.equal(await call('read', fd, read, 0, read.length, 0), read.length);
    assert.equal(new TextDecoder().decode(read), '你好, editor!\n');
    await call('close', fd);
    assert.equal(globalThis.path.resolve('internal/example.go'), '/ttt/internal/example.go');
    await assert.rejects(call('open', '/ttt/missing.go', fs.constants.O_RDONLY, 0o644), { code: 'ENOENT' });
  } finally {
    Object.assign(globalThis, original);
  }
});

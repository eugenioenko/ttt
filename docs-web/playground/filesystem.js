import { configure, InMemory, fs } from '@zenfs/core';
import { dirname, resolve } from '@zenfs/core/path';

export async function installFilesystem(files) {
  await configure({ mounts: { '/': InMemory } });
  fs.mkdirSync('/ttt', { recursive: true });
  fs.mkdirSync('/home/demo/.config/ttt', { recursive: true });
  fs.mkdirSync('/tmp', { recursive: true });
  fs.writeFileSync('/tmp/stdio', '');
  // Go reserves descriptors 0–2 for stdio; ZenFS starts allocating at 0.
  for (let i = 0; i < 3; i++) fs.openSync('/tmp/stdio', 'r+');
  for (const [name, contents] of Object.entries(files)) {
    const path = resolve('/ttt', name);
    if (!path.startsWith('/ttt/')) throw new Error(`Invalid workspace path: ${name}`);
    fs.mkdirSync(dirname(path), { recursive: true });
    fs.writeFileSync(path, contents);
  }
  let cwd = '/ttt';
  globalThis.path = { resolve: (...parts) => resolve(cwd, ...parts) };
  globalThis.process = {
    argv0: 'ttt-web', pid: 1, ppid: 0,
    getuid: () => 0, getgid: () => 0, geteuid: () => 0, getegid: () => 0,
    getgroups: () => [], umask: () => 0o022,
    cwd: () => cwd,
    chdir: (path) => {
      const next = resolve(cwd, path);
      if (!fs.statSync(next).isDirectory()) throw new Error('Not a directory');
      cwd = next;
    },
  };
  globalThis.fs = { ...fs,
    writeSync: (fd, data, ...args) => {
      if (fd > 2) return fs.writeSync(fd, data, ...args);
      console.log(new TextDecoder().decode(data));
      return data.length;
    },
    write: (fd, data, offset, length, position, callback) => {
      if (fd > 2) return fs.write(fd, data, offset, length, position, callback);
      globalThis.fs.writeSync(fd, data.subarray(offset, offset + length));
      callback(null, length);
    },
  };
}

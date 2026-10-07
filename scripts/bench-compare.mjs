#!/usr/bin/env node
// Compares BenchmarkEditor between a base checkout (BASE_DIR) and this one on
// the same machine, alternating passes, and writes a Markdown report.
//
//   BASE_DIR=../ttt-main node scripts/bench-compare.mjs

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const baseDir = path.resolve(process.env.BASE_DIR ?? '');
const count = Number(process.env.COUNT ?? 3);
const benchTime = process.env.BENCH_TIME ?? '5x';
const marker = '<!-- ttt-editor-benchmarks -->';
const benchFile = 'tests/e2e/editor_bench_test.go';
const fixtures = 'tests/e2e/testdata/bench';

if (!process.env.BASE_DIR) throw new Error('BASE_DIR must name the base checkout');
if (!Number.isInteger(count) || count < 1) throw new Error(`COUNT must be a positive integer, got ${count}`);

// A base that predates the benchmarks is measured with the PR's scenarios.
const overlaid = !fs.existsSync(path.join(baseDir, benchFile));
if (overlaid) {
  fs.copyFileSync(path.join(root, benchFile), path.join(baseDir, benchFile));
  fs.cpSync(path.join(root, fixtures), path.join(baseDir, fixtures), { recursive: true });
}

const temporaryDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ttt-bench-'));
const sides = {
  base: { dir: baseDir, binary: path.join(temporaryDir, 'base.test'), results: {} },
  head: { dir: root, binary: path.join(temporaryDir, 'head.test'), results: {} },
};

function compile(side) {
  execFileSync('go', ['test', '-c', '-o', side.binary, './tests/e2e'], { cwd: side.dir, stdio: ['ignore', 'inherit', 'inherit'] });
}

function run(side) {
  const output = execFileSync(
    side.binary,
    ['-test.run', '^$', '-test.bench', '^BenchmarkEditor$', '-test.benchmem', '-test.benchtime', benchTime, '-test.short'],
    { cwd: path.join(side.dir, 'tests/e2e'), encoding: 'utf8', maxBuffer: 64 << 20 },
  );
  for (const line of output.split('\n')) {
    const match = line.match(/^BenchmarkEditor\/(\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns\/op.*?([\d.]+) allocs\/op/);
    if (!match) continue;
    const [, name, ns, allocs] = match;
    const previous = side.results[name];
    if (!previous || Number(ns) < previous.ns) side.results[name] = { ns: Number(ns), allocs: Number(allocs) };
  }
}

const duration = (ns) => ns >= 1e6 ? `${(ns / 1e6).toFixed(1)} ms` : `${(ns / 1e3).toFixed(0)} µs`;
const percent = (next, previous) => {
  if (!previous) return 'n/a';
  const change = ((next / previous) - 1) * 100;
  if (Math.abs(change) < 0.05) return '0.0%';
  return `${change >= 0 ? '+' : ''}${change.toFixed(1)}%`;
};

try {
  console.error('building base and PR benchmark binaries...');
  compile(sides.base);
  compile(sides.head);
  for (let pass = 0; pass < count; pass++) {
    console.error(`benchmark pass ${pass + 1}/${count}...`);
    const order = pass % 2 === 0 ? [sides.base, sides.head] : [sides.head, sides.base];
    for (const side of order) run(side);
  }

  const names = Object.keys(sides.head.results).filter((name) => sides.base.results[name]);
  if (names.length === 0) throw new Error('no benchmark ran on both sides');
  const ratios = names.map((name) => sides.head.results[name].ns / sides.base.results[name].ns);
  const geomean = (Math.exp(ratios.reduce((sum, ratio) => sum + Math.log(ratio), 0) / ratios.length) - 1) * 100;
  const short = (sha, fallback) => (sha ? sha.slice(0, 12) : fallback);

  const lines = [
    marker,
    '## Editor benchmarks',
    '',
    `Comparing \`${short(process.env.BASE_SHA, 'base')}\` with \`${short(process.env.HEAD_SHA, 'PR')}\` on the same runner: ` +
      `fastest of ${count} alternating passes at ${benchTime} each, \`-short\` (2,000-line large file).` +
      (overlaid ? ' The base predates these benchmarks, so it was measured with the PR\'s scenarios.' : ''),
    '',
    `Geometric-mean time change: **${geomean >= 0 ? '+' : ''}${geomean.toFixed(1)}%**.`,
    '',
    '| Benchmark | Base | PR | Time change | Base allocs | PR allocs | Allocation change |',
    '| --- | ---: | ---: | ---: | ---: | ---: | ---: |',
  ];
  for (const name of names) {
    const before = sides.base.results[name];
    const after = sides.head.results[name];
    lines.push(
      `| ${name} | ${duration(before.ns)} | ${duration(after.ns)} | ${percent(after.ns, before.ns)} | ` +
        `${before.allocs} | ${after.allocs} | ${percent(after.allocs, before.allocs)} |`,
    );
  }
  const added = Object.keys(sides.head.results).filter((name) => !sides.base.results[name]);
  if (added.length) lines.push('', `New in this PR: ${added.map((name) => `\`${name}\``).join(', ')}.`);
  lines.push('', '_Timing changes are informational because shared CI runners are noisy; a benchmark error still fails the job._', '');

  const report = lines.join('\n');
  console.log(report);
  if (process.env.REPORT_FILE) fs.writeFileSync(process.env.REPORT_FILE, report);
  if (process.env.GITHUB_STEP_SUMMARY) fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, report);
} finally {
  fs.rmSync(temporaryDir, { recursive: true, force: true });
}

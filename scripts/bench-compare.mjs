#!/usr/bin/env node
// Compares BenchmarkEditor and BenchmarkDiff between a base checkout (BASE_DIR)
// and this one on the same machine, alternating passes, and writes a Markdown
// report with one table per suite.
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
const suites = [
  { name: 'Editor', title: 'Editor', file: 'tests/e2e/editor_bench_test.go' },
  { name: 'Diff', title: 'Diff views', file: 'tests/e2e/diff_bench_test.go' },
];
const harnessFile = 'tests/e2e/harness_test.go';
const fixtures = 'tests/e2e/testdata/bench';

if (!process.env.BASE_DIR) throw new Error('BASE_DIR must name the base checkout');
if (!Number.isInteger(count) || count < 1) throw new Error(`COUNT must be a positive integer, got ${count}`);

// A base that predates a suite is measured with the PR's scenarios for it. A
// base without the editor suite also takes the PR's harness and fixtures.
const overlaid = suites.filter((suite) => !fs.existsSync(path.join(baseDir, suite.file)));
for (const suite of overlaid) {
  fs.copyFileSync(path.join(root, suite.file), path.join(baseDir, suite.file));
}
if (overlaid.some((suite) => suite.name === 'Editor')) {
  fs.copyFileSync(path.join(root, harnessFile), path.join(baseDir, harnessFile));
  fs.cpSync(path.join(root, fixtures), path.join(baseDir, fixtures), { recursive: true });
}

const temporaryDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ttt-bench-'));
const sides = {
  base: { dir: baseDir, binary: path.join(temporaryDir, 'base.test'), results: {} },
  head: { dir: root, binary: path.join(temporaryDir, 'head.test'), results: {} },
};
const pattern = `^Benchmark(${suites.map((suite) => suite.name).join('|')})$`;
const resultLine = new RegExp(`^Benchmark(${suites.map((suite) => suite.name).join('|')})\\/(\\S+?)(?:-\\d+)?\\s+\\d+\\s+([\\d.]+) ns\\/op.*?([\\d.]+) allocs\\/op`);
const results = (side, suite) => {
  side.results[suite] ??= {};
  return side.results[suite];
};

function compile(side) {
  execFileSync('go', ['test', '-c', '-o', side.binary, './tests/e2e'], { cwd: side.dir, stdio: ['ignore', 'inherit', 'inherit'] });
}

function run(side) {
  const output = execFileSync(
    side.binary,
    ['-test.run', '^$', '-test.bench', pattern, '-test.benchmem', '-test.benchtime', benchTime, '-test.short'],
    { cwd: path.join(side.dir, 'tests/e2e'), encoding: 'utf8', maxBuffer: 64 << 20 },
  );
  for (const line of output.split('\n')) {
    const match = line.match(resultLine);
    if (!match) continue;
    const [, suite, name, ns, allocs] = match;
    const suiteResults = results(side, suite);
    const previous = suiteResults[name];
    if (!previous || Number(ns) < previous.ns) suiteResults[name] = { ns: Number(ns), allocs: Number(allocs) };
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

  const short = (sha, fallback) => (sha ? sha.slice(0, 12) : fallback);
  const lines = [
    marker,
    '## Editor and diff benchmarks',
    '',
    `Comparing \`${short(process.env.BASE_SHA, 'base')}\` with \`${short(process.env.HEAD_SHA, 'PR')}\` on the same runner: ` +
      `fastest of ${count} alternating passes at ${benchTime} each, \`-short\` (2,000-line large file).` +
      (overlaid.length
        ? ` The base predates ${overlaid.map((suite) => `\`Benchmark${suite.name}\``).join(' and ')}, so it was measured with the PR's scenarios.`
        : ''),
  ];
  let compared = 0;
  for (const suite of suites) {
    const base = sides.base.results[suite.name] ?? {};
    const head = sides.head.results[suite.name] ?? {};
    const names = Object.keys(head).filter((name) => base[name]);
    const added = Object.keys(head).filter((name) => !base[name]);
    if (names.length === 0 && added.length === 0) continue;
    compared += names.length;
    lines.push('', `### ${suite.title}`, '');
    if (names.length) {
      // Geometric mean of PR/base ratios: every benchmark weighs the same however
      // long it runs. Zero on both sides is no change; zero on one side has no ratio.
      const geomean = (field) => {
        const ratios = names
          .map((name) => [base[name][field], head[name][field]])
          .filter(([before, after]) => (before > 0) === (after > 0))
          .map(([before, after]) => (before > 0 ? after / before : 1));
        if (ratios.length === 0) return '**n/a**';
        const mean = Math.exp(ratios.reduce((sum, ratio) => sum + Math.log(ratio), 0) / ratios.length);
        return `**${percent(mean, 1)}**`;
      };
      lines.push(
        `Geometric-mean change: time ${geomean('ns')}, memory (allocations) ${geomean('allocs')}.`,
        '',
        '<details>',
        '<summary>Click here to see benchmark details</summary>',
        '',
        '| Benchmark | Base | PR | Time change | Base allocs | PR allocs | Allocation change |',
        '| --- | ---: | ---: | ---: | ---: | ---: | ---: |',
      );
      for (const name of names) {
        const before = base[name];
        const after = head[name];
        lines.push(
          `| ${name} | ${duration(before.ns)} | ${duration(after.ns)} | ${percent(after.ns, before.ns)} | ` +
            `${before.allocs} | ${after.allocs} | ${percent(after.allocs, before.allocs)} |`,
        );
      }
      lines.push('', '</details>');
    }
    if (added.length) lines.push('', `New in this PR: ${added.map((name) => `\`${name}\``).join(', ')}.`);
  }
  if (compared === 0) throw new Error('no benchmark ran on both sides');
  lines.push('', '_Timing changes are informational because shared CI runners are noisy; a benchmark error still fails the job._', '');

  const report = lines.join('\n');
  console.log(report);
  if (process.env.REPORT_FILE) fs.writeFileSync(process.env.REPORT_FILE, report);
  if (process.env.GITHUB_STEP_SUMMARY) fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, report);
} finally {
  fs.rmSync(temporaryDir, { recursive: true, force: true });
}

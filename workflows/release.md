# Release Prep Workflow

Run before cutting a new release. Stop and fix any failure before continuing to the next step — do not proceed on a red run.

## 1. Full test suite (single pass)

```sh
make test                          # unit + e2e: go test ./...
cd tests/functional && pnpm test   # functional (drives bin/ttt via --exec)
cd tests/integration && pnpm test  # integration (drives bin/ttt via real PTY)
```

`make build` first if the functional/integration suites need a fresh `bin/ttt`.

## 2. Go tests x100 (normal)

```sh
for i in $(seq 1 100); do
  go test ./... || { echo "FAILED on iteration $i"; break; }
done
```

## 3. Go tests x100 (race detector)

```sh
for i in $(seq 1 100); do
  go test -race ./... || { echo "FAILED on iteration $i"; break; }
done
```

## 4. Chaos monkey — 1,000,000 events

Chaos tests execute arbitrary random commands and must run in Docker — never run this on the host.

```sh
docker build -t ttt-chaos -f tests/chaos/Dockerfile .
mkdir -p chaos-output
docker run --rm \
  -e CHAOS_ITERATIONS=2000 \
  -e CHAOS_EVENTS=500 \
  -v "$(pwd)/chaos-output:/output" \
  --entrypoint /chaos-test ttt-chaos \
  -test.run TestChaosMonkey -test.v -test.timeout 0
```

`CHAOS_ITERATIONS x CHAOS_EVENTS = 2000 x 500 = 1,000,000` total events. Rebuild the image first (`docker build`) so it picks up the current source tree, not a stale one. This can take a long time — `-test.timeout 0` disables the Go test timeout so it isn't killed mid-run; let it finish or run it in the background.

Any crash is written to `chaos-output/crash-<seed>-<iteration>.json`. Replay one with:

```sh
CHAOS_REPLAY=chaos-output/crash-<seed>-<iter>.json make chaos-replay
```

# C debugger fixture

Run from the ttt checkout on Linux x86-64 with Docker, Go, and the GitHub CLI:

```sh
make build
context=$(mktemp -d)
gh release download v1.12.3 -R vadimcn/codelldb -p codelldb-linux-x64.vsix -D "$context"
docker build -t ttt-dap-native -f tests/debugger-native/Dockerfile "$context"
docker run --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
  -v "$PWD/bin/ttt:/usr/local/bin/ttt:ro" \
  -v "$PWD/tests/debugger-native/check.sh:/check.sh:ro" ttt-dap-native sh /check.sh
```

The container compiles one C program with symbols. Each debugger launches it through ttt, stops at the configured source breakpoint, reads a local variable, steps to change its value, and continues to exit. The adapter commands and target configuration are in `check.sh`.

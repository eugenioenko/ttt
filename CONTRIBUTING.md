# Contributing to TTT

Thanks for your interest in contributing! Read the [README](README.md) for an overview of the project.

## Getting started

Prerequisites: [Go](https://go.dev/) 1.25+, [Git](https://git-scm.com/), [ripgrep](https://github.com/BurntSushi/ripgrep).

```sh
make build   # builds to bin/ttt
make test    # go test ./...
make lint    # golangci-lint run
```

## How to contribute

1. Features need an accepted issue first; bug fixes, docs, and small cleanups can go straight to a PR. Comment on the issue you are picking up so work is not duplicated.
2. Make your change with tests, and run it in the real binary. `bin/ttt --exec "..."` scripts input and captures screenshots; `bin/ttt --listen` lets you drive a running editor.
3. Make sure `make test` and `make lint` pass, then open a PR against `main`.

[AGENTS.md](AGENTS.md) is the detailed guide for humans and AI agents alike: architecture constraints, test layers, the debug harness, and PR expectations. [ARCHITECTURE.md](ARCHITECTURE.md) covers package boundaries.

## Code style

Run `make fmt` before committing. Add a comment only when missing it would cause a bug or misuse. AI-generated code tends to over-comment, so strip those comments before opening a PR.

## What makes a good PR

- Small and focused, one concern per PR
- Title using conventional commits: `type(scope): description`
- The *why* in the PR body, plus a screenshot for visible changes

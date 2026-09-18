# Contributing to TTT

Thanks for your interest in contributing! Read the [README](README.md) for an overview of the project.

## Getting started

Prerequisites: [Go](https://go.dev/) 1.18+, [Git](https://git-scm.com/), [ripgrep](https://github.com/BurntSushi/ripgrep).

```sh
make build   # builds to bin/ttt
make test    # go test ./...
make lint    # golangci-lint run
```

## How to contribute

1. Fork the repo and create a branch
2. Make your changes
3. Make sure `make test` and `make lint` pass
4. Open a PR against `main`

## Code style

Run `make fmt` before committing. The linter (`make lint`) and `go vet` catch the rest.

Minimize comments. Only add a comment when the *why* is non-obvious — a hidden constraint, a subtle invariant, a workaround for a specific bug. Don't comment what the code does; well-named identifiers handle that.

## Testing

`make test` must pass for all PRs. If your change touches an area with existing tests, update them. If you're adding new behavior, add tests for it.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for package boundaries and dependency zones. [AGENTS.md](AGENTS.md) summarizes constraints for AI-assisted contributors.

## What makes a good PR

- Small and focused — one concern per PR
- Clear title using conventional commits: `type(scope): description`
- Explain the *why* in the PR body, not just the *what*

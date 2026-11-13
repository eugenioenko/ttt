# Contributing to TTT

Thanks for your interest in contributing! Read the [README](README.md) for an overview of the project.

## Getting started

Prerequisites: [Go](https://go.dev/) 1.25+, [Git](https://git-scm.com/), [ripgrep](https://github.com/BurntSushi/ripgrep).

```sh
make build   # builds to bin/ttt
make test    # go test ./...
make lint    # gofmt check + golangci-lint run
```

## How to contribute

1. Bug fixes, docs, and small cleanups can go straight to a PR. New features need an accepted issue first (see [Proposing a feature](#proposing-a-feature)). If you are working from an existing issue, see [Picking up an issue](#picking-up-an-issue).
2. Make your change with tests, and run it in the real binary. `bin/ttt --exec "..."` scripts input and captures screenshots; `bin/ttt --listen` lets you drive a running editor.
3. Make sure `make test` and `make lint` pass, then open a PR against `main`.
4. Follow up on your PR: fix each CodeRabbit finding or reply explaining why it does not apply, and push fixes until CI passes and CodeRabbit has reviewed your latest commit ([guidance](AGENTS.md#following-up-on-a-pull-request)).

[AGENTS.md](AGENTS.md) is the detailed guide for humans and AI agents alike: architecture constraints, test layers, the debug harness, and PR expectations. [ARCHITECTURE.md](ARCHITECTURE.md) covers package boundaries.

## Proposing a feature

TTT is close to feature parity with the editors it is modeled on, so most remaining work is polish and fixes rather than new surface area. Before building a new feature, open an issue describing what you want and why, and wait for the maintainer and community to agree on scope and design. This keeps the editor coherent and avoids a finished PR that cannot be merged.

## Picking up an issue

These are suggestions, not rules. They help avoid duplicated and wasted effort.

- **Check first.** Read the issue's comments and linked PRs to make sure nobody is already working on it. If someone claimed it a while ago and there is no PR or update, ask in the thread before taking it over.
- **Say you're on it.** Leave a short comment such as "I'll take this" so others know it is taken.
- **One at a time.** Work on one issue, and wait until its PR is merged before picking up the next. Reviews go faster, and other contributors get a chance at the good first issues.
- **Reproduce first.** For bugs, start with a failing test on the latest `main` that shows the problem, then fix it.
- **Address review feedback.** When CodeRabbit reviews your PR, go through each finding: fix it, or reply explaining why it does not apply. Push fixes until CI passes and CodeRabbit has reviewed your latest commit, so the maintainer reviews a PR that is already clean.

## Code style

Run `make fmt` before committing. Add a comment only when missing it would cause a bug or misuse. AI-generated code tends to over-comment, so strip those comments before opening a PR.

## What makes a good PR

- Small and focused, one concern per PR
- Title using conventional commits: `type(scope): description`
- The *why* in the PR body, plus before and after [screenshots](AGENTS.md#screenshots) for visible changes

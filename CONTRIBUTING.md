# Contributing to gitdeck

Thanks for your interest in contributing! Here's everything you need to get started.

## Getting started

1. Fork the repository and clone your fork:

```bash
git clone https://github.com/<your-user>/gitdeck.git
cd gitdeck
```

2. Make sure you have **Go 1.24+** installed:

```bash
go version
```

3. Build and run:

```bash
go build -o gitdeck ./cmd/gitdeck
./gitdeck
```

4. Run the tests:

```bash
go test ./...
```

## How to contribute

### Reporting bugs

Open an [issue](https://github.com/waabox/gitdeck/issues) with:

- What you expected to happen.
- What actually happened.
- Steps to reproduce.
- OS, Go version, and gitdeck version (`gitdeck --version` or commit hash).

### Suggesting features

Open an [issue](https://github.com/waabox/gitdeck/issues) describing the use case and why it would be useful. We'll discuss it before any code is written.

### Submitting a pull request

1. **Open an issue first** to discuss the change — this avoids wasted effort.
2. Create a feature branch from `main`:

```bash
git checkout -b feature/my-change
```

3. Make your changes. Keep commits focused and atomic.
4. Make sure tests pass:

```bash
go test ./...
```

5. Push and open a pull request against `main`.

## Project structure

```
cmd/gitdeck/       # Application entry point
internal/          # Private packages (TUI, providers, config, etc.)
assets/            # Logo and demo assets
.github/workflows/ # CI and release pipelines
```

## Code guidelines

- Follow standard Go conventions (`gofmt`, `go vet`).
- Keep functions small and focused.
- Write tests for new functionality.
- Commit messages should be clear and explain **what** and **why** — not implementation details.

## CI

Every pull request runs the CI workflow (`.github/workflows/ci.yml`). Make sure it passes before requesting a review.

## License

By contributing you agree that your contributions will be licensed under the [MIT License](LICENSE).

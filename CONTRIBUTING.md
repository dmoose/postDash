# Contributing to eventrelay

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/dmoose/eventrelay.git
cd eventrelay
make build    # Build the binary
make test     # Run tests with race detector
```

### Prerequisites

- Go 1.26+

## Making Changes

1. Fork the repo and create a feature branch from `main`
2. Make your changes
3. Run `make fmt` and `make lint`
4. Run `make test` to ensure all tests pass
5. Submit a pull request

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Wrap errors with context: `fmt.Errorf("doing something: %w", err)`
- Write table-driven tests where appropriate
- Use `t.TempDir()` for tests that need filesystem access

## What to Contribute

- Bug fixes with test cases
- Documentation improvements
- New notification targets
- SDK improvements or new language SDKs
- Performance improvements with benchmarks

## Reporting Issues

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Go version and OS

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

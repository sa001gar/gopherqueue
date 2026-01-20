# Contributing to GopherQueue

Thank you for your interest in contributing to GopherQueue! This document provides guidelines and information for contributors.

## Code of Conduct

Please be respectful and considerate of others. We aim to maintain a welcoming and inclusive community.

## How to Contribute

### Reporting Issues

- Check existing issues to avoid duplicates
- Use the issue templates when available
- Provide clear reproduction steps
- Include Go version and operating system

### Submitting Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Run linter: `golangci-lint run`
6. Commit with clear messages
7. Push and create a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/gopherqueue.git
cd gopherqueue

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./...
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Write meaningful comments for exported types and functions
- Keep functions focused and small
- Write tests for new functionality

### Testing

- Write unit tests for new features
- Ensure existing tests pass
- Aim for good test coverage
- Use table-driven tests where appropriate

### Documentation

- Update README if adding features
- Add godoc comments for public APIs
- Update CHANGELOG for notable changes

## Questions?

Open an issue with the "question" label, and we'll be happy to help!

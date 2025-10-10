---
title: Contributing
description: How to contribute to Dagu in a few clear steps.
outline: [2, 3]
---

Thank you for considering to help improve Dagu! We welcome contributions from anyone on the internet.

## Quick Start

- Browse [`good first issue`](https://github.com/dagu-org/dagu/labels/good%20first%20issue) or [`help wanted`](https://github.com/dagu-org/dagu/labels/help%20wanted) labels and comment to claim.
- Join the [Discord server](https://discord.gg/gpahPUjGRk) for questions or to share progress.

## Getting Started

- Fork the repository and clone it locally
- Look for any issue that interests you
- Make your changes and test them
- Ask questions if anything is unclear

## How to Contribute

We welcome contributions of all kinds, including:

- Help other users by answering questions and providing support
- Suggest new features or improvements
- Improve documentation and examples, or provide use cases
- Refactor code for better readability and maintainability
- Fix bugs or add missing tests
- Add new features based on issue discussions
- Review and provide feedback on PRs

## Development

Prerequisites:

- [Go (latest stable)](https://go.dev/doc/install)
- [Node.js](https://nodejs.org/en/download/)
- [pnpm](https://pnpm.io/installation)

Building frontend assets:

```bash
make ui
```

Building binary:

```bash
make bin
```

Running tests:

```bash
make lint
make test
```

Running test with coverage:

```bash
make test-coverage
```

## Frontend

Starting the backend server on port 8080:

```bash
DAGU_PORT=8080 make
```

Starting the development server:

```bash
cd ui
pnpm install
pnpm dev
```

Navigate to [http://localhost:8081](http://localhost:8081) to view hot-reloading frontend.

### Code Standards

- Write unit tests for any new functionality
- Aim for good test coverage on new code
- Test error conditions and edge cases

### Pull Requests

Before submitting:

- Tests pass (`make test`)
- Linter passes (`make lint`)
- New code includes tests
- Documentation updated if applicable
- Commit messages following the [Go Commit Message Guidelines](https://go.dev/wiki/CommitMessage)

### Review Process

- All PRs are reviewed by [maintainers](https://github.com/dagu-org/dagu/graphs/contributors).
- Community members are encouraged to review and provide feedback.

## Issues

### Bug Reports

When reporting bugs, please include:

- Operating system and version
- Steps to reproduce the issue (example DAG yaml is very helpful)
- Expected behavior
- Actual behavior
- Relevant logs or error messages

### Feature Requests

When requesting features, please describe:

- Clearly describe the feature and its use case
- Explain why it would be valuable
- Consider backward compatibility
- Provide examples if possible

## License

By contributing to Dagu, you agree that your contributions will be licensed under the **GNU General Public License v3.0**.

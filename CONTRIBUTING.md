# Contributing to GitScars (`scars`)

Thank you for your interest in contributing to `git-scars`! We welcome contributions from the community.

## Getting Started

1. **Prerequisites**:
   - Go 1.22 or later
   - Git

2. **Clone and Branch**:
   ```bash
   git clone https://github.com/Pavanspoojary/scars.git
   cd scars
   git checkout -b feat/your-feature-name
   ```


## Development Workflow

- Follow standard Go formatting conventions (`gofmt -s -w .`).
- Write focused unit tests for any new features or bug fixes.
- Verify your changes locally before submitting:
  ```bash
  gofmt -l .
  go vet ./...
  go test -race ./...
  ```

## Commits & Pull Requests

- Use concise, descriptive Conventional Commit messages (`feat: ...`, `fix: ...`, `docs: ...`).
- Open a pull request against the `main` branch with a clear description of the problem solved and testing performed.
- Ensure automated CI checks pass.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Be respectful, collaborative, and constructive in all discussions, code reviews, and issue threads. Report unacceptable behavior through a private security advisory, as described in the code of conduct.

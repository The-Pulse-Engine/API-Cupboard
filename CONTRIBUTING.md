# Contributing to API Cupboard

Thank you for your interest. This file tells you how to report problems and send changes.

## Before you start

- Join the [PulseEngine Discord](https://discord.gg/VU8RUByEeG) to see open issues on the commission board and to ask questions. To claim an issue, comment on it here on GitHub.
- Read [`docs/project-plan.md`](docs/project-plan.md). It defines the scope.
- Features that are out of scope (tray app, key rotation, rate limits, macOS, sync) go in a `v2-ideas` issue. Do not send a pull request for them.
- For security problems, do not open a public issue. Read [`SECURITY.md`](SECURITY.md).

## Report a bug

Open an issue and include:

1. Your OS and version (for example, Windows 11 or Arch Linux).
2. The output of `cupboard --version`.
3. The steps to reproduce the problem.
4. What you expected and what happened.

Do not paste API keys, tokens or the contents of your keychain.

## Set up the project

Requirements:

- Go (the version in `go.mod`)
- Git
- On Linux: a running Secret Service (for example `gnome-keyring` or `kwallet`)

```sh
git clone https://github.com/<owner>/api-cupboard.git
cd api-cupboard
go build ./...
go test ./...
```

## Send a change

1. Fork the repository and create a branch from `main`.
2. Keep each pull request to one change.
3. Run these commands before you push:

   ```sh
   gofmt -l .
   go vet ./...
   go test ./...
   ```

4. Add tests for new behavior. Changes to `internal/proxy` or `internal/ui` must include tests for the security checks they touch.
5. Write a clear pull request title and describe what changed and why.

## Code rules

- Use the Go standard library first. Add a dependency only if there is a strong reason, and explain it in the pull request.
- Keep functions small and names clear.
- Keep comments short.
- Never log, print or return secrets in errors.
- Do not weaken a security check to make a test pass.

## Commit messages

Use a short summary line in the imperative mood, for example:

```text
proxy: reject requests with unknown Host header
```

## License

When you contribute, you agree that your contribution is licensed under the [MIT License](LICENSE).

# Contributing to Chatwire

Ideas, bug reports, questions and pull requests are all welcome: open an
[issue](https://github.com/PeterStoica/chatwire/issues) for anything. For a change bigger than a small fix, open an
issue first so we can agree on the approach.

## Ground rules

- Chatwire is written from scratch. Do not copy code from other WhatsApp clients or libraries, or from WhatsApp's own
  apps and web client, and do not paste their source into issues or pull requests. Describe the behaviour instead.
- Never put real phone numbers, names, messages or keys in code, tests, issues or logs. The tests use made-up numbers
  such as +40 700 000 000.

## Working on the code

- You need Go 1.27 or newer: `go build ./cmd/chatwire` and `go test ./...`.
- Pull requests go to `main`, and the maintainer reviews and approves every change. A release is a version tag on
  `main`: it is built, checked by installing it on Linux, macOS and Windows, and only then published.
- Every pull request runs gofmt, go vet, the tests on Linux, macOS and Windows, the race detector, govulncheck and
  gitleaks. Before you push, run `gofmt -l .`, `go vet ./...` and `go test -race ./...`.
- Tests describe what Chatwire does, not how, need no phone and pass without a network: `internal/testkit` has a
  fake WhatsApp server, phone and media CDN, and timing tests use `testing/synctest`.
- The code has no comments; explain why in the commit message. Commit messages follow Conventional Commits, for
  example `fix(link): answer WhatsApp's pings while linking`.

## Sign-off

Add a `Signed-off-by` line to each commit (`git commit -s`). It confirms you wrote the change and may submit it, as
described in the Developer Certificate of Origin (https://developercert.org). Contributions are licensed under the
Apache License 2.0, like the rest of Chatwire.

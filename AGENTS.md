# Repository Guidelines

## Project Structure & Module Organization

PDG is a Go CLI for connecting independently hosted websites to ATProto, with a small TypeScript web package.

- `cmd/pdg/` contains the executable entrypoint.
- `internal/cli/`, `config/`, `scan/`, `publish/`, and `identity/` contain application layers.
- `internal/protocol/atproto/` contains identity, DID/PDS, OAuth, and Standard.site protocol code.
- `internal/shared/` contains reusable filesystem and HTTP helpers.
- `web/src/` contains framework-independent Web Component code; `web/src/components/` holds individual components.
- `examples/static-site/` contains scan fixtures and demonstration HTML.
- Go tests live beside their implementation files as `*_test.go`.

## Build, Test, and Development Commands

- `make build` builds `bin/pdg`.
- `make run` runs the CLI through `go run`.
- `make test` runs all Go tests (`go test ./...`).
- `make vet` runs `go vet ./...`.
- `make fmt` formats Go files with `gofmt`.
- `make check` formats, vets, and tests the Go code.
- `make web-install` installs web dependencies with pnpm.
- `make web-build` type-checks and builds the web package.
- Example: `go run ./cmd/pdg scan ./examples/static-site`.

## Coding Style & Naming Conventions

Use standard `gofmt` formatting and idiomatic Go: mixed-case identifiers, concise package names, early returns, wrapped errors with operation context, and `context.Context` as the first parameter. Keep protocol-independent models separate from ATProto implementation details. TypeScript uses the project’s existing formatting and strict compiler settings; use descriptive kebab-free filenames and camelCase identifiers.

## Testing Guidelines

Use Go’s standard `testing` package with table-driven tests where cases share setup. Name tests `TestSubject` or `TestSubjectCondition`; keep protocol and parser edge cases covered, especially malformed metadata, invalid endpoints, and OAuth failures. Run `make check` before submitting. Web tests, when added, should use Vitest via `cd web && pnpm test`.

## Commit & Pull Request Guidelines

Use short, imperative Conventional Commit-style subjects, matching history such as `feat(atproto): add ...` and `chore: ...`. Keep commits focused. Pull requests should explain the behavior and rationale, identify relevant commands/tests run, call out protocol or configuration changes, and include rendered UI screenshots when web components visibly change. Do not commit credentials, OAuth tokens, generated binaries, or local site-specific `pdg.yaml` files.

## Security & Configuration

Treat identity and OAuth data as sensitive. Validate external URLs and DID documents at boundaries, preserve context cancellation, and avoid logging tokens or authorization codes. `pdg init` writes `pdg.yaml`; inspect it before committing and keep site-specific values local.

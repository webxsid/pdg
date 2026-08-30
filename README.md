# Project DG

**Project Digital Garden (PDG)** is an experiment in connecting independent websites to the open social web.

PDG aims to let developers keep their existing website, framework, domain, and hosting while adding open social capabilities through protocols such as ATProto.

The website remains yours. The social data remains on the open protocol. PDG is the bridge.

> [!IMPORTANT]
> PDG is currently experimental and under active development. There is no stable release or API yet.

## What PDG is exploring

The initial version focuses exclusively on existing websites.

PDG will provide tooling to:

- scan existing static websites for publishable content
- connect a website to an ATProto identity
- publish compatible content using standards such as Standard.site
- associate existing webpages with their ATProto records
- provide framework-independent Web Components for open social interactions

The intended workflow is roughly:

```text
Existing Website
       │
       ▼
      PDG
       │
       ├──── publish ────► ATProto
       │
       ◄──── interact ─── Web Components
       │
       ▼
Existing Website
```

PDG does not host or replace the website.

## Current status

PDG is in early v0 development.

The current implementation can scan static HTML output and identify publishable documents using existing web metadata.

### Planned V1 work includes:

* project configuration
* ATProto identity and OAuth
* Standard.site publishing
* deterministic webpage ↔ ATProto record association
* profile, follow, reaction, and conversation Web Components


### Development

Requirements

* Go 1.25+
* pnpm
* Node.js

#### Build
```
make build
```

The PDG CLI will be written to:
```txt
bin/pdg 
```

#### Test
```
make test
```

#### Run Locally
```
go run ./cmd/pdg --help
```

**For Example**
```go 
go run ./cmd/pdg scan ./examples/static-site
```

### Project Structure
```
.
├── cmd/pdg/              # CLI entrypoint
├── internal/
│   ├── cli/
│   ├── config/
│   ├── scan/
│   ├── publish/
│   ├── identity/
│   └── protocol/
│       └── atproto/
├── web/                  # Framework-independent web package
├── examples/
└── docs/
```

The core PDG models are intended to remain protocol-independent. 
ATProto is the first supported open social protocol.

### Scope

PDG V1 is intentionally constrained.

It is not currently:

* a hosting provider
* a website generator
* a CMS
* a social network
* an ATProto PDS
* an ATProto AppView

The first goal is much smaller:

> Take an existing website and make it a participant in the open social web.

License

PDG is licensed under the MIT License. See [LICENSE](LICENSE)

## Authentication Storage

PDG stores OAuth credentials in the operating system's secure credential
store: macOS Keychain, Windows Credential Manager, or Linux Secret Service.
PDG does not fall back to plaintext credential files, encrypted files, or
environment variables when a native store is unavailable. Linux headless
environments need an active Secret Service/D-Bus provider.

Non-secret account metadata is stored separately under the platform user
configuration directory in `pdg/auth/accounts.json`; it contains no access
tokens, refresh tokens, authorization codes, or DPoP private keys.

To manually rotate the active ATProto session's access and refresh tokens, run
`pdg atproto auth refresh`. The existing DPoP key is reused, and rotated
credentials are written back to the native credential store. `pdg atproto auth
status` never refreshes automatically; when a known access-token expiry has
passed, it recommends the refresh command.

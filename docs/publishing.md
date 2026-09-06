# PDG V0 Publishing Model

PDG V0 operates on generated static HTML. The framework and source format are
not part of the scanner contract:

```text
site source → framework build → generated static HTML → PDG scan
```

## Configuration

The generated directory is supplied to `pdg scan`, or defaults to the
Standard.site `public_dir`. Publication paths belong to the Standard.site
integration, not to Bluesky:

```yaml
site:
  url: https://example.com

integrations:
  standard_site:
    enabled: true
    identity: did:plc:...
    public_dir: ./dist
    paths:
      - path: blog
        publish: all
      - path: notes
        publish: explicit
  bluesky:
    enabled: true
    identity: did:plc:...
```

Scan the configured Standard.site public directory:

```sh
pdg scan
```

An explicit directory overrides the configured public directory:

```sh
pdg scan ./dist
```

Paths are relative to the project root and cannot escape it. `/` means the
project root. `public_dir` is resolved safely within the project root.

Initialize and attach integrations separately:

```sh
pdg init
pdg add standard-site
pdg add bluesky
pdg configure standard-site
```

The Standard.site publication URI is managed state in `.pdg/state.json`.
Credentials remain in the secure authentication store and are never written to
`pdg.yaml` or `.pdg`.

## Routing

The only document publication target is `standard-site`. Bluesky is an explicit
social action and is not controlled by scan paths.

For a Standard.site path configured as `publish: all`, matching pages are
selected unless they explicitly use:

```html
<meta name="pdg:targets" content="none">
```

`none` takes precedence over `publish: all` and must be used by itself. For a
path configured as `publish: explicit`, a page must opt in with:

```html
<meta name="pdg:targets" content="standard-site">
```

Unknown publication targets, including `bluesky`, are rejected. Bluesky posting
is initiated explicitly with `pdg post <path>`.

## Metadata

General document metadata uses target-neutral names:

```html
<meta name="pdg:title" content="A title">
<meta name="pdg:description" content="A description">
<meta name="pdg:published-at" content="2026-08-30T10:00:00Z">
<meta name="pdg:updated-at" content="2026-08-30T12:00:00Z">
<meta name="pdg:tag" content="atproto">
```

Optional Bluesky copy is separate from publication routing:

```html
<meta name="pdg:bluesky" content="Optional copy for the social post.">
```

The reserved future namespace is `pdg:<target>:<field>`; arbitrary
target-specific fields are not parsed in V0.

## Scan, publish, and inject

The normal workflow is:

```text
build → pdg scan → pdg publish standard-site → pdg inject standard-site → deploy
```

`pdg scan [directory]` discovers generated HTML and records durable, non-secret
document state in `.pdg/state.json`. With no directory argument it scans
`integrations.standard_site.public_dir`. It never authenticates, performs remote operations,
or modifies generated HTML.

`pdg publish` consumes `.pdg/state.json`; it does not rescan HTML. Standard.site
documents require a published time, so intended publication pages should
provide `pdg:published-at` or supported published-time metadata. Bluesky posts
are not part of `pdg publish`.

`pdg inject standard-site` materializes the Standard.site publication discovery
file and document verification links in generated HTML. It is local-only,
uses `.pdg/state.json` as URI authority, and is safe to repeat after a clean
site build.

To explicitly create a Bluesky post for a published Standard.site document:

```sh
pdg post /blog/example
pdg post /blog/example --dry-run
pdg post /blog/example --content "Custom copy"
```

## Verification

`pdg verify` checks local generated artifacts. `pdg verify --live` checks the
deployed Standard.site publication file and document verification links over
HTTP. Live verification is read-only and does not query the PDS or repair the
deployed site.

`pdg repair` restores safely reconstructable local Standard.site artifacts.

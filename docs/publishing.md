# PDG V0 Publishing Model

PDG V0 operates on generated static HTML. The framework and source format are
not part of the scanner contract:

```text
site source → framework build → generated static HTML → PDG scan
```

Configure the generated directory and publication targets in `pdg.yaml`:

```yaml
[scan]
dist = "./dist"

[[scan.scope]]
path = "blog"
targets = ["standard-site", "bluesky"]
```

Standard.site setup also records the public directory used for the verification
file and the project-relative paths that should be scanned:

```yaml
integrations:
  standard_site:
    enabled: true
    public_dir: public
    paths:
      - path: /
        publish: all
      - path: blog
        publish: explicit
  bluesky:
    enabled: true
    identity: did:plc:...
    paths:
      - path: blog
        publish: all
```

`/` means the project root. Paths are never allowed to escape the project
root. Both integrations use the same project-relative path semantics; only
Standard.site uses `public_dir` for its verification file. The optional `scan`
section can provide more advanced scan settings; it is not emitted by
`pdg init` unless configured explicitly.

Supported targets are `standard-site` and `bluesky`. A page can replace its
scope targets with:

```html
<meta name="pdg:targets" content="standard-site">
```

Use `content="none"` to explicitly route a page to no targets. Target
selection is separate from whether an integration is enabled in project
configuration.

General metadata uses target-neutral names such as `pdg:title`,
`pdg:description`, `pdg:published-at`, `pdg:updated-at`, and repeated
`pdg:tag` elements. The reserved future namespace is
`pdg:<target>:<field>`; target-specific fields are not parsed in V0.

`pdg scan` records durable, non-secret discovery state in `.pdg/state.json`.
The state distinguishes content changes from target routing changes and keeps
history for missing, unpublished, and previously selected targets. Scanning
does not create, update, or delete remote records and never modifies generated
HTML. Standard.site documents and Bluesky posts will be handled by a future
`pdg publish` workflow.

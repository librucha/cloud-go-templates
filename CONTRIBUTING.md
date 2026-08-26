# Contributing

Thanks for taking the time. This is a small library; the bar is "a reviewer
can tell what changed and why, and a test proves it".

## Getting set up

You need Go (the version in [`go.mod`](go.mod); [asdf](https://asdf-vm.com/) or
[mise](https://mise.jdx.dev/) users get it from [`.tool-versions`](.tool-versions))
and [Task](https://taskfile.dev/installation/).

```sh
git clone https://github.com/librucha/cloud-go-templates.git
cd cloud-go-templates
task install-tools   # golangci-lint, govulncheck
task check           # fmt, vet, lint, test
```

`task --list` shows everything else: `test-coverage`, `vuln`, `doc`, `tidy`,
`upgrade`.

## Tests never touch a cloud

Every test in this repository runs offline, with no Azure credentials and no
network. That is not a convention to be polite about - CI has no subscription
to talk to, so a test that dials out is a test that fails.

Use `azure.WithCredential` with a fake token credential and `azure.WithTransport`
with a stub transport. `azure/testing_test.go` has the fakes the existing tests
share; start there.

## What a change should look like

- **A test that fails without the change.** For a bug fix, write the failing
  test first and put it in the same commit or an earlier one.
- **Doc comments on everything exported.** `revive`'s `exported` rule is on,
  so lint catches the omission, but write them for the reader, not the linter.
- **The README stays true.** If you add or change a template function, update
  the function table and any prose that describes its behaviour.
- **Deliberate quirks stay documented.** Some behaviour here is odd on purpose
  (`azKey` emits a public modulus under a `PRIVATE KEY` header, PKCS12 handles
  RSA only) because changing it would change rendered output for existing
  users. If you touch one of those, say in the PR why the break is worth it.

## Commit messages

Short imperative subject, prefixed with the kind of change:

```
fix: return an error when a storage account has no keys
add: azTableEntity template function
docs: describe the provider's context lifetime
test: cover the concurrent client-build path
```

The release notes are generated from commits and PR titles, so the subject
line is what users end up reading.

## Compatibility

Exported API is a promise. Renaming or removing an exported symbol, changing a
signature, or changing what a template function renders is a breaking change,
and for a Go module that means a new major version - a heavy cost for a
consumer. Prefer adding a new function or a new option over changing an
existing one.

## Releasing

Maintainers only:

```sh
task release -- v1.2.3
git push origin v1.2.3
```

The tag push runs the release workflow: it re-verifies the tagged commit,
creates the GitHub release with generated notes, and asks the module proxy for
the new version so it appears on pkg.go.dev.

## Reporting security problems

Do not open an issue. See [SECURITY.md](SECURITY.md).

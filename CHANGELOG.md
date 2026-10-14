# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Because this is a Go module, a breaking change to an exported symbol means a
new major version and a new import path.

## [Unreleased]

### Added

- `entraAppClientId <displayName>` template function, aliased as
  `entraApplicationClientId`: the application (client) ID of an Entra ID app
  registration, looked up by display name through Microsoft Graph.

- Project infrastructure for public release: Taskfile, golangci-lint config,
  CI matrix (Linux/macOS/Windows), CodeQL and govulncheck workflows, tag-driven
  release workflow, Dependabot, issue and PR templates, contributor docs.

## [0.1.0] - Unreleased

Initial extraction of the Azure template functions from
[krmgen](https://github.com/librucha/krmgen) into a standalone library.

### Added

- `azure.Provider` with per-provider Azure clients and cache, safe for
  concurrent use.
- Options: `WithCredential`, `WithTransport`, `WithSubscriptionID`.
- Template functions: `azSec`, `toPem`, `azPfxKey`, `azPfxCrt`, `azCert`,
  `azKey`, `azStoreKey`, `azUserIdentityClientId`.

[Unreleased]: https://github.com/librucha/cloud-go-templates/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/librucha/cloud-go-templates/releases/tag/v0.1.0

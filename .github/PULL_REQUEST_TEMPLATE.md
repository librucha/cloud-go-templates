# What this changes

<!-- One or two sentences. Link the issue it closes, if any. -->

Closes #

## Why

<!-- What was wrong or missing. Skip if the title already says it. -->

## Checklist

- [ ] `task check` passes locally (fmt, vet, lint, test)
- [ ] New behaviour is covered by a test that fails without the change
- [ ] Tests use a stub transport (`azure.WithTransport`) and never touch a live cloud
- [ ] Exported symbols have doc comments
- [ ] The README table and docs match the code, if functions changed
- [ ] Breaking change? Say so here and explain the migration:

<!--
A breaking change to an exported API means a new major version for consumers.
Flag it explicitly rather than letting a reviewer discover it.
-->

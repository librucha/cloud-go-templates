# Security Policy

## Supported versions

The latest released minor version is supported. Fixes are not backported.

## Reporting a vulnerability

Report privately through GitHub:
[**Report a vulnerability**](https://github.com/librucha/cloud-go-templates/security/advisories/new).

Do not open a public issue, and do not include real secrets, certificates or
tokens in the report - a redacted reproduction is enough.

Expect an acknowledgement within a week. If a fix is warranted, it ships in a
patch release together with a published advisory crediting you, unless you ask
otherwise.

## What is in scope

This library reads secrets, certificates and keys out of Azure and hands the
values to Go's `text/template`. In scope, for example:

- a value leaking somewhere it should not (logs, error messages, the cache)
- the cache serving a value under the wrong key
- authentication or credential handling that does not do what it documents
- a parsing bug in the PKCS12 handling that is exploitable with a crafted bundle

## What is not in scope

- **Rendered output is not escaped.** `text/template`, not `html/template`, is
  the target. A template that renders a secret into an HTML page will produce
  unescaped output; that is the caller's responsibility.
- **Secrets in rendered artifacts.** The whole point of the library is to put
  cloud secrets into rendered files. Where those files end up is the caller's
  problem.
- **Vulnerabilities in the Azure SDK or `golang.org/x/crypto`.** Report those
  upstream. If one affects this library, an issue here noting the advisory is
  welcome.
- **`azKey`'s PEM header.** It emits the RSA modulus - public material - under
  a `"... PRIVATE KEY"` header. This is a documented, deliberately preserved
  quirk, not a leak: no private key material is exposed.

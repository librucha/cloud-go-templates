# cloud-go-templates

Go template functions that read configuration out of public clouds, in the
spirit of [sprig](https://github.com/Masterminds/sprig): you build a `FuncMap`
and hand it to `text/template`.

Today only Azure is implemented.

## Usage

```go
provider, err := azure.New(ctx)
if err != nil {
	return err
}

tmpl := template.New("example").Funcs(provider.FuncMap())
```

A `Provider` owns its Azure clients and its cache. Two providers share
nothing, so a test or a second render can have its own.

Credentials come from `azidentity.NewDefaultAzureCredential` unless you pass
`azure.WithCredential`. Nothing touches the network until the first function
call.

## Functions

| Function | Arity | Returns |
|---|---|---|
| `azSec <vault> <name> [version]` | 2–3 | Key Vault secret value |
| `toPem <type> <data>` | 2 | The data wrapped in a PEM block |
| `azPfxKey <vault> <secret> [version]` | 2–3 | Private key extracted from a PKCS12 secret, PEM-encoded |
| `azPfxCrt <vault> <secret> [version]` | 2–3 | Certificate chain extracted from a PKCS12 secret, PEM-encoded |

Omitting the version selects the most recently created enabled version whose
`notBefore` has passed.

## Caching

Every lookup is cached for the lifetime of the provider and keyed by the
function's own arguments. Nothing is invalidated mid-render.

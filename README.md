# cloud-go-templates

Go template functions that read configuration out of public clouds, in the
spirit of [sprig](https://github.com/Masterminds/sprig): you build a `FuncMap`
and hand it to `text/template`.

Today only Azure is implemented.

## Install

```sh
go get github.com/librucha/cloud-go-templates
```

Import path: `github.com/librucha/cloud-go-templates/azure`.

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

### The context you pass to `New`

`azure.New(ctx)` stores `ctx` on the `Provider` and reuses it for every Azure
call the provider ever makes. This is deliberate: Go template functions
receive only the arguments the template passes them, so there is no per-call
site to thread a context through - the context has to come from somewhere,
and construction time is the only place left.

The practical consequence: the context's lifetime **is** the provider's
lifetime. Pass a context that lives as long as you intend to keep using the
provider, or every call after it expires or is cancelled fails silently from
the caller's point of view (each function returns `ctx.Err()`-flavoured
errors, but nothing about the failure says "your context expired").

- For a CLI run (krmgen's use case): `context.Background()`. The process
  lives exactly as long as the render does, so there is nothing to expire.
- For a server handling many renders: build one `Provider` per render (or per
  request) with that render's own context, not one long-lived `Provider`
  fed a request-scoped context from its first caller - the second caller's
  render would run against a context that isn't its own. If you want a
  provider that outlives any single request (to keep its cache warm), give it
  its own long-lived context - `context.Background()` again, not
  `context.WithTimeout` or a context derived from a request - and accept that
  cancellation is then your responsibility rather than "whichever request
  happened to construct it".

### Testing without the network

`azure.WithTransport` supplies the HTTP transport every Azure client is
built with. This is how a consumer tests its own template code that calls
`azSec`, `azCert`, and so on without touching a live Key Vault or needing
Azure credentials:

```go
type stubTransport struct{}

func (stubTransport) Do(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"value":"stubbed-secret"}`)),
	}, nil
}

provider, err := azure.New(ctx,
	azure.WithCredential(fakeCredential{}), // any azcore.TokenCredential; never dials out
	azure.WithTransport(stubTransport{}),
)
```

See `azure/testing_test.go` in this repository for the fuller fake credential
and mock sender this package's own tests use.

## Functions

| Function | Arity | Returns |
|---|---|---|
| `azSec <vault> <name> [version]` | 2–3 | Key Vault secret value |
| `toPem <type> <data>` | 2 | The data wrapped in a PEM block |
| `azPfxKey <vault> <secret> [version]` | 2–3 | Private key extracted from a PKCS12 secret, PEM-encoded |
| `azPfxCrt <vault> <secret> [version]` | 2–3 | Certificate chain extracted from a PKCS12 secret, PEM-encoded |
| `azCert <vault> <name> [version]` | 2–3 | Key Vault certificate, PEM-encoded |
| `azKey <vault> <name> [version]` | 2–3 | Key Vault key's RSA modulus, PEM-wrapped under an `"... PRIVATE KEY"` header - **not actually a private key**; see below |
| `azStoreKey <subscription> <resourceGroup> <account>` | 3 | Storage account's first access key |
| `azUserIdentityClientId <resourceGroup> <name>` | 2 | Client ID of a user-assigned managed identity |

Omitting the version selects the most recently created enabled version whose
`notBefore` has passed.

`azKey` has emitted the RSA modulus (the public part of the key pair) under a
`"<KTY> PRIVATE KEY"` PEM header since its earliest version. That mismatch
between header and payload is preserved deliberately - fixing it would change
rendered output for anyone already depending on this function.

`azStoreKey` takes its subscription as an explicit argument. `azUserIdentityClientId`
has none to pass, so it resolves one itself: `azure.WithSubscriptionID`, else
the `AZURE_SUBSCRIPTION_ID` environment variable, else the first subscription
the credential can see. `azStoreKey` returns an error naming the account if
it has no keys, rather than panicking.

`azPfxKey` and `azPfxCrt` are built on `golang.org/x/crypto/pkcs12`, whose own
doc comment says it is [frozen](https://go.dev/wiki/Frozen) and not accepting
new features, pointing instead at `software.sslmate.com/src/go-pkcs12` for
new code. This package has not moved off `golang.org/x/crypto/pkcs12`. On top
of that, this package's own PKCS8 conversion, `convertToPkcs8`, handles
**RSA keys only** - a PFX bundle carrying an EC key parses fine as PKCS12 but
fails at the PKCS1-to-PKCS8 conversion step. Both are inherited from the code
this package was ported from, not introduced by the port.

## Caching

Every lookup is cached for the lifetime of the provider and keyed by the
function's own arguments. Nothing is invalidated mid-render.

There is no in-flight request deduplication (no singleflight): two goroutines
racing on the same uncached key each pass the cache-miss check and each issue
their own network call; both then write the same value back under the same
key. That is redundant traffic, not a correctness problem, but it is not "one
network call" - if you are counting on a single lookup per key under
concurrent access, you don't get it. A single caller making two *sequential*
references to the same key does get one network call, since the second finds
what the first one's write left behind.

## Concurrency

A `Provider` is safe for concurrent use: call its functions - including
different functions, against different vaults - from as many goroutines as
you like without additional locking. Building a client for one vault or
resource kind never blocks building a client for another; only two callers
racing to build the *same* client wait on each other, and both then share the
client the first one built.

`go test -race ./...` catches a data race in this code if one is introduced,
but a race-free implementation is not the same claim as "unrelated client
kinds don't serialize" - the previous design (one mutex shared by every
client kind) was already race-free, it just serialized every build behind
that one lock, which is a liveness defect the race detector cannot see.
`TestProvider_UnrelatedClientKindsDoNotSerialize` is what actually backs that
claim: it is a timing test that fails if building one client kind blocks
building another.

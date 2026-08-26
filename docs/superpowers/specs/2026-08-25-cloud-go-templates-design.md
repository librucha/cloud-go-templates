# cloud-go-templates — design

*Date: 2026-08-25 · Status: describes v0.1.0 as built*

## 1. What this is

A Go `template.FuncMap` for reading configuration out of public clouds, in the
spirit of [sprig](https://github.com/Masterminds/sprig): you construct a
provider, take its `FuncMap`, and hand that to `text/template`. Templates can
then reach for a Key Vault secret or a managed identity's client ID the same
way they reach for `upper` or `trimSuffix`.

Today only Azure is implemented.

```go
provider, err := azure.New(ctx)
if err != nil {
    return err
}
tmpl := template.New("example").Funcs(provider.FuncMap())
```

### Why it exists

The functions were extracted from a CLI that held its Azure clients and lookup
results in **ten package-level maps**. That is tolerable in a single-shot
binary and unacceptable in a library: the state cannot be reset, cannot be
isolated between callers, and races under concurrent use. Anyone embedding
those functions would have inherited a global they could neither see nor
control.

The whole shape of this library follows from removing that: a `Provider`
instance owns everything, and two providers share nothing.

### Non-goals

- **Not an Azure SDK wrapper.** It exposes template functions, not a general
  client library. Anything a template cannot express is out of scope.
- **Not a secret manager.** It reads at render time; it never creates,
  rotates or writes back.
- **Not multi-cloud yet.** The name is an ambition. AWS and GCP would arrive
  as sibling functions under their own prefixes (`awsSec`, `gcpSec`), not by
  overloading the Azure ones.
- **No Entra ID.** Group and application lookups were considered and
  deliberately dropped, which also keeps `msgraph-sdk-go` out of the
  dependency graph.

## 2. Architecture

One package, `azure`, exposing one type.

```
azure.New(ctx, opts...) → *Provider ── FuncMap() → template.FuncMap
                             │
                             ├── credential   (lazy, sync.Once)
                             ├── subscription (lazy, sync.Once)
                             ├── clients      (sync.Map of per-key sync.Once)
                             └── cache        (map guarded by a mutex)
```

**Why one package rather than one per resource kind.** The code this came from
was split into six packages — secrets, certificates, keys, storage, identity,
commons — but that split existed only so each could own its globals. With an
instance holding the state, six packages would mean six constructors and six
`FuncMap`s for a consumer to merge by hand, for no gain. The files inside the
package stay split by resource kind.

### Components

| File | Responsibility |
|---|---|
| `provider.go` | `Provider`, `New`, `FuncMap`, lazy credential, per-key client construction |
| `options.go` | `Option`, `WithCredential`, `WithSubscriptionID`, `WithTransport` |
| `cache.go` | the result cache: a map behind a mutex |
| `secrets.go` | `azSec`, `toPem`, and version resolution |
| `pkcs12.go` | `azPfxKey`, `azPfxCrt` |
| `certificates.go` | `azCert` |
| `keys.go` | `azKey` |
| `storage.go` | `azStoreKey` |
| `identity.go` | `azUserIdentityClientId` |
| `subscription.go` | which subscription to use, resolved once |
| `testing_test.go` | the one mock transport every test shares |

## 3. Public API

```go
func New(ctx context.Context, opts ...Option) (*Provider, error)

func WithCredential(cred azcore.TokenCredential) Option
func WithSubscriptionID(id string) Option
func WithTransport(t policy.Transporter) Option

func (p *Provider) FuncMap() template.FuncMap
func ToPem(blockType, text string) (string, error)
```

`New` performs **no I/O**. Credentials, subscription and clients are all
resolved on first use, so constructing a provider cannot fail on a network
problem, and a program that never renders an Azure function never talks to
Azure.

`WithTransport` exists so a consumer can point every client at a stub and test
templates without a network. The library's own tests use nothing else.

### The context is stored

`Provider` keeps the `context.Context` given to `New` and uses it for every
later call. Storing a context in a struct is normally poor Go style; here
there is no alternative, because a template function receives only its string
arguments — there is no call site to thread a context through.

The consequence belongs in the caller's head: **a provider is only as
long-lived as its context.** Pass `context.Background()` for a CLI run. In a
server, build a provider per request rather than passing a request-scoped
context to a long-lived one.

### Template functions

`FuncMap()` returns exactly these eight entries, and that set is the library's
public contract:

| Function | Arity | Returns |
|---|---|---|
| `azSec <vault> <name> [version]` | 2–3 | Key Vault secret value |
| `toPem <type> <data>` | 2 | the data wrapped in a PEM block |
| `azPfxKey <vault> <secret> [version]` | 2–3 | private key from a PKCS12 secret |
| `azPfxCrt <vault> <secret> [version]` | 2–3 | certificate chain from a PKCS12 secret |
| `azCert <vault> <name> [version]` | 2–3 | Key Vault certificate, PEM |
| `azKey <vault> <name> [version]` | 2–3 | see the note below |
| `azStoreKey <subscription> <group> <account>` | 3 | first storage account key |
| `azUserIdentityClientId <group> <name>` | 2 | client ID of a user-assigned managed identity |

A test asserts the map's exact contents **and** that each name is bound to the
function it claims — a swap between two entries would otherwise pass every
other test in the suite.

## 4. Behaviour worth pinning

### Secret version resolution

`azSec` with no version selects the most recently created **enabled** version
whose `notBefore` has passed. Disabled versions and versions dated into the
future are skipped; if none qualifies, it is an error rather than a fallback.

That lookup costs a version listing, so the result is cached twice: under the
version-less key and under the version it resolved to. A later call naming
that version hits the cache instead of listing again.

`azCert` and `azKey` do **not** do this. A version-less and an explicit-version
reference to the same certificate are two separate cache entries and two
network calls.

### `azKey` does not return a private key

`azKey` PEM-wraps the RSA **modulus** under a header reading
`"<KTY> PRIVATE KEY"`. It is not a private key and never was; Key Vault does
not export private key material this way.

This is inherited behaviour, preserved deliberately because changing it would
change what existing templates render. It is documented at the function, in a
test named for it, and here. **Do not "fix" it without treating it as a
breaking change.**

### Managed identities: user-assigned only

`azUserIdentityClientId` resolves user-assigned identities, addressed by
resource group and name. System-assigned identities are addressed by the ARM
scope of their host resource and go through a different SDK client, so they
would need their own function — `azSystemIdentityClientId <scope>` — rather
than an overload of this one.

In practice the user-assigned form is what manifests need: the
`azure.workload.identity/client-id` annotation requires a federated credential,
and federated credentials exist only on user-assigned identities.

### Subscription resolution

For the functions that need a subscription, it is taken from, in order:

1. `WithSubscriptionID`
2. the `AZURE_SUBSCRIPTION_ID` environment variable
3. the first subscription the credential can see, asked of Azure

Resolved once per provider and memoised, including the failure.

### Errors, not panics

Every path that could dereference something absent in an unusual Azure
response returns an error naming the resource: a storage account with no keys,
a secret with no value, an identity with no client ID, a key with no material
or no key type. The code this was extracted from panicked in those cases and
took the process with it.

## 5. Caching and concurrency

**A provider is safe for concurrent use.**

**Client construction is deduplicated per key.** Each client kind and target
(`"secrets:myvault"`, `"certificates:myvault"`) gets its own entry behind its
own `sync.Once`, so two goroutines building the same client wait for and share
one build, while building different clients never blocks across keys. A failed
construction is cached too — every build function is a pure SDK constructor, so
a failure is deterministic and retrying could not help.

**Results are cached, requests are not.** The cache is a map behind a mutex.
`get` and `put` are two independent operations, and the network call between
them runs outside any lock. There is **no in-flight deduplication**: two
goroutines racing on the same uncached key each issue their own call to Azure
and each store the result. A single caller making two sequential references
causes one call.

**Cache keys are always built from the calling function's own arguments**,
never from the resource ID Azure returns. This is not an implementation detail
worth forgetting: the code this came from keyed writes on the returned ID and
reads on a locally built one, so every entry was written and never found, and
every reference re-fetched. Any new function must follow the same rule.

Entries live for the lifetime of the provider and are never invalidated or
evicted. Nothing persists between processes.

## 6. Testing strategy

- **No test touches the network or needs credentials.** Every test builds a
  provider with `WithCredential(fakeCredential{})` and `WithTransport(...)`
  pointed at a stub that answers with canned JSON.
- **One shared harness**, in `testing_test.go`. The code this came from
  duplicated the same mock transport five times, once per package; collapsing
  the packages collapsed the harness with them.
- **Cache assertions count requests.** A test that merely checks two calls
  return the same value proves nothing; these assert the stub was called once.
- **The concurrency guarantee is tested by observation, not by the race
  detector.** This is worth spelling out, because it is easy to get wrong: the
  design being replaced used a single shared mutex, which is *race-free* and
  merely serialises. `-race` cannot tell it from per-key locking. The guarantee
  is therefore proven by a test that blocks one client's construction and
  asserts a different kind still gets through, with a timeout so a regression
  fails rather than hangs.
- Coverage is 87.3%. What remains uncovered is the real-credential path, which
  by definition cannot be exercised offline.

## 7. Known limitations

| Limitation | Consequence |
|---|---|
| `golang.org/x/crypto/pkcs12` is frozen upstream | no fixes are coming; a move to `software.sslmate.com/src/go-pkcs12` is the eventual answer |
| `convertToPkcs8` handles RSA only | a PFX carrying an EC key fails |
| No in-flight request deduplication | a concurrent burst on one cold key multiplies calls to Azure |
| Azure SDK versions are pinned low | they were matched to the consumer this was extracted from, so the extraction could not smuggle in an upgrade; raising them is a deliberate, separate change |
| System-assigned managed identities unsupported | see section 4 |

## 8. Open questions

1. **Multi-cloud naming.** `awsSec`/`gcpSec` as sibling functions is the
   assumption. An alternative — one `sec` function taking a cloud prefix in a
   coordinate string — was considered and rejected: it loses arity checking and
   collides with values containing the separator.
2. **Is a second provider per process ever wanted?** Nothing prevents it, but
   nothing needs it yet. If it stays unused, the option surface could shrink.
3. **Go version floor.** `go.mod` currently names the newest toolchain, which
   forces the same on every consumer. It should be lowered to the oldest
   version the code actually needs.
4. **API stability.** At v0.1.0 the `FuncMap` names are treated as a contract
   but the Go surface is not. Before v1, `apidiff` or `gorelease` in CI would
   make breaking changes visible rather than accidental.

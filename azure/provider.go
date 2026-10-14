// Package azure provides Go template functions that read secrets,
// certificates, keys and identifiers from Azure.
//
// A Provider owns its clients and its cache, so two providers never share
// state. Construct one, take its FuncMap, and hand that to text/template.
package azure

import (
	"context"
	"errors"
	"sync"
	"text/template"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// Provider resolves Azure resources for template functions.
type Provider struct {
	// ctx is stored deliberately. Go template functions receive only their
	// arguments, so there is no call site to thread a context through; the
	// lifetime of a render is the lifetime of the provider.
	ctx context.Context
	cfg config

	credOnce sync.Once
	cred     azcore.TokenCredential
	credErr  error

	subIDOnce sync.Once
	subID     string
	subIDErr  error

	// clients holds one *clientEntry per client kind (keyed by e.g.
	// "secrets:myvault"). Each entry has its own sync.Once, so building the
	// certificates client for one vault never blocks building the secrets
	// client for another - only concurrent builds under the *same* key wait
	// on each other, and that wait resolves to the same cached client.
	clients sync.Map

	cache *cache
}

// clientEntry lazily builds and caches a single Azure client. A failed
// build is cached too - sync.Once runs build at most once regardless of
// outcome, so every later caller for that key gets the same error rather
// than retrying. All build funcs are pure SDK constructors with no I/O
// (credential resolution is separately memoized in credOnce), so this is
// benign in practice, but it is a behaviour change from the previous
// design, which retried construction on the next call.
type clientEntry struct {
	once sync.Once
	val  any
	err  error
}

// New builds a Provider. It performs no I/O: credentials and clients are
// created on first use.
func New(ctx context.Context, opts ...Option) (*Provider, error) {
	if ctx == nil {
		return nil, errors.New("azure: New requires a non-nil context")
	}
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Provider{
		ctx:   ctx,
		cfg:   cfg,
		cache: newCache(),
	}, nil
}

// FuncMap returns the template functions this provider exposes, keyed by the
// name templates call them under.
func (p *Provider) FuncMap() template.FuncMap {
	return template.FuncMap{
		"azSec":    p.SecretFunc,
		"toPem":    ToPem,
		"azPfxKey": p.PfxKeyFunc,
		"azPfxCrt": p.PfxCertFunc,
		"azCert":   p.CertificateFunc,
		"azKey":    p.KeyFunc,

		"azStoreKey":             p.StorageKeyFunc,
		"azUserIdentityClientId": p.UserIdentityClientIDFunc,

		"entraAppClientId":         p.EntraAppClientIDFunc,
		"entraApplicationClientId": p.EntraAppClientIDFunc,
	}
}

// credential resolves the credential once, lazily.
func (p *Provider) credential() (azcore.TokenCredential, error) {
	p.credOnce.Do(func() {
		if p.cfg.credential != nil {
			p.cred = p.cfg.credential
			return
		}
		p.cred, p.credErr = azidentity.NewDefaultAzureCredential(nil)
	})
	return p.cred, p.credErr
}

// client returns the client stored under key, or builds one with build and
// stores it. Clients are reused for the provider's lifetime.
//
// Construction happens under a per-key sync.Once, not a lock shared by every
// client kind: two goroutines building different clients (say, secrets for
// one vault and certificates for another) never wait on each other. Two
// goroutines racing to build the *same* client do wait, but only until the
// first build finishes, and both then get the one client it produced.
func (p *Provider) client(key string, build func(azcore.TokenCredential) (any, error)) (any, error) {
	v, ok := p.clients.Load(key)
	if !ok {
		v, _ = p.clients.LoadOrStore(key, &clientEntry{})
	}
	entry := v.(*clientEntry)
	entry.once.Do(func() {
		cred, err := p.credential()
		if err != nil {
			entry.err = err
			return
		}
		entry.val, entry.err = build(cred)
	})
	return entry.val, entry.err
}

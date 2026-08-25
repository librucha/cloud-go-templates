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

	clientsMu sync.Mutex
	clients   map[string]any

	cache *cache
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
		ctx:     ctx,
		cfg:     cfg,
		clients: make(map[string]any, 8),
		cache:   newCache(),
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
func (p *Provider) client(key string, build func(azcore.TokenCredential) (any, error)) (any, error) {
	p.clientsMu.Lock()
	defer p.clientsMu.Unlock()
	if c, ok := p.clients[key]; ok {
		return c, nil
	}
	cred, err := p.credential()
	if err != nil {
		return nil, err
	}
	c, err := build(cred)
	if err != nil {
		return nil, err
	}
	p.clients[key] = c
	return c, nil
}

package azure

import (
	"encoding/pem"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
)

// CertificateFunc is the azCert template function: azCert <vault> <name> [version].
func (p *Provider) CertificateFunc(vaultName string, args ...string) (any, error) {
	switch len(args) {
	case 1:
		return p.certificate(vaultName, args[0], "")
	case 2:
		return p.certificate(vaultName, args[0], args[1])
	default:
		return nil, fmt.Errorf("wrong arguments count for function %q: expected 1 or 2 arguments after the vault name, got %d", "azCert", len(args))
	}
}

// certificate reads one certificate and PEM-encodes it.
func (p *Provider) certificate(vaultName, name, version string) (string, error) {
	key := cacheKey("certificate", vaultName, name, version)
	if v, ok := p.cache.get(key); ok {
		return v.(string), nil
	}

	client, err := p.certificatesClient(vaultName)
	if err != nil {
		return "", err
	}

	cert, err := client.GetCertificate(p.ctx, name, version, nil)
	if err != nil {
		return "", err
	}

	pemCert := wrapCert(cert.CER)
	p.cache.put(key, pemCert)
	return pemCert, nil
}

func (p *Provider) certificatesClient(vaultName string) (*azcertificates.Client, error) {
	c, err := p.client("certificates:"+vaultName, func(cred azcore.TokenCredential) (any, error) {
		return azcertificates.NewClient(vaultURL(vaultName), cred, &azcertificates.ClientOptions{
			ClientOptions: azcore.ClientOptions{Transport: p.cfg.transport},
		})
	})
	if err != nil {
		return nil, err
	}
	return c.(*azcertificates.Client), nil
}

// wrapCert PEM-encodes the raw x509 certificate bytes Key Vault returns.
func wrapCert(data []byte) string {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: data,
	}
	return string(pem.EncodeToMemory(block))
}

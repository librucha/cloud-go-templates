package azure

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/pkcs12"
)

// PfxKeyFunc is the azPfxKey template function: azPfxKey <vault> <secret> [version].
// It reads a secret that is a base64 PKCS12 bundle and returns its private
// key, converted to PKCS8 and PEM-encoded.
func (p *Provider) PfxKeyFunc(vaultName string, args ...string) (any, error) {
	secret, err := p.pfxSecret("azPfxKey", vaultName, args...)
	if err != nil {
		return nil, err
	}
	return extractKey(secret)
}

// PfxCertFunc is the azPfxCrt template function: azPfxCrt <vault> <secret> [version].
// It reads a secret that is a base64 PKCS12 bundle and returns its
// certificate chain, PEM-encoded.
func (p *Provider) PfxCertFunc(vaultName string, args ...string) (any, error) {
	secret, err := p.pfxSecret("azPfxCrt", vaultName, args...)
	if err != nil {
		return nil, err
	}
	return extractCert(secret)
}

// pfxSecret resolves the base64 PKCS12 secret behind a Pfx function, with the
// same arity contract as SecretFunc: one argument names the secret, two name
// the secret and a version.
func (p *Provider) pfxSecret(funcName, vaultName string, args ...string) (string, error) {
	switch len(args) {
	case 1:
		return p.Secret(vaultName, args[0], "")
	case 2:
		return p.Secret(vaultName, args[0], args[1])
	default:
		return "", fmt.Errorf("wrong arguments count for function %q: expected 1 or 2 arguments after the vault name, got %d", funcName, len(args))
	}
}

// extractKey pulls the private key out of a base64 PKCS12 bundle, converts
// it from PKCS1 to PKCS8, and PEM-encodes it.
func extractKey(b64Content string) (any, error) {
	pfxData, err := base64.StdEncoding.DecodeString(b64Content)
	if err != nil {
		return nil, err
	}
	blocks, err := pkcs12.ToPEM(pfxData, "")
	if err != nil {
		return nil, err
	}

	for _, block := range blocks {
		if strings.Contains(block.Type, "KEY") {
			block.Headers = nil
			pkcs8, err := convertToPkcs8(block.Bytes)
			if err != nil {
				return nil, err
			}
			block.Bytes = pkcs8
			return string(pem.EncodeToMemory(block)), nil
		}
	}
	return nil, fmt.Errorf("no KEY block found")
}

// convertToPkcs8 converts a PKCS1 RSA private key to PKCS8.
func convertToPkcs8(pkcs1 []byte) ([]byte, error) {
	key, err := x509.ParsePKCS1PrivateKey(pkcs1)
	if err != nil {
		return nil, err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pkcs8, nil
}

// extractCert pulls every non-key block out of a base64 PKCS12 bundle and
// PEM-encodes the certificate chain.
func extractCert(b64Content string) (any, error) {
	pfxData, err := base64.StdEncoding.DecodeString(b64Content)
	if err != nil {
		return nil, err
	}
	blocks, err := pkcs12.ToPEM(pfxData, "")
	if err != nil {
		return nil, err
	}

	var certs strings.Builder
	for i := range blocks {
		block := blocks[i]
		if !strings.Contains(block.Type, "KEY") {
			block.Headers = nil
			certs.Write(pem.EncodeToMemory(block))
			certs.WriteRune('\n')
		}
	}

	if strings.TrimSpace(certs.String()) == "" {
		return nil, fmt.Errorf("no certificate block found")
	}
	return certs.String(), nil
}

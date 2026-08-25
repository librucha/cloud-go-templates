package azure

import (
	"encoding/pem"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

// KeyFunc is the azKey template function: azKey <vault> <name> [version].
func (p *Provider) KeyFunc(vaultName string, args ...string) (any, error) {
	switch len(args) {
	case 1:
		return p.key(vaultName, args[0], "")
	case 2:
		return p.key(vaultName, args[0], args[1])
	default:
		return nil, fmt.Errorf("wrong arguments count for function %q: expected 1 or 2 arguments after the vault name, got %d", "azKey", len(args))
	}
}

// key reads one key and PEM-encodes it via wrapKey.
func (p *Provider) key(vaultName, name, version string) (string, error) {
	key := cacheKey("key", vaultName, name, version)
	if v, ok := p.cache.get(key); ok {
		return v.(string), nil
	}

	client, err := p.keysClient(vaultName)
	if err != nil {
		return "", err
	}

	resp, err := client.GetKey(p.ctx, name, version, nil)
	if err != nil {
		return "", err
	}

	pemKey, err := wrapKey(&resp.KeyBundle)
	if err != nil {
		return "", fmt.Errorf("key %q in vault %q: %w", name, vaultName, err)
	}
	p.cache.put(key, pemKey)
	return pemKey, nil
}

func (p *Provider) keysClient(vaultName string) (*azkeys.Client, error) {
	c, err := p.client("keys:"+vaultName, func(cred azcore.TokenCredential) (any, error) {
		return azkeys.NewClient(vaultURL(vaultName), cred, &azkeys.ClientOptions{
			ClientOptions: azcore.ClientOptions{Transport: p.cfg.transport},
		})
	})
	if err != nil {
		return nil, err
	}
	return c.(*azkeys.Client), nil
}

// wrapKey PEM-wraps the RSA modulus (key.Key.N) under a header that reads
// "<KTY> PRIVATE KEY". This is NOT a private key - N is the public modulus,
// the same value that appears in the public key. That mismatch between the
// header and the payload has been krmgen's behaviour all along; it is
// preserved here unchanged because correcting it would change rendered
// output, which this port must not do.
//
// An unusual Key Vault response - no key material at all, or a key kind Key
// Vault omitted - is reported as an error instead of dereferencing a nil
// pointer and killing the process.
func wrapKey(key *azkeys.KeyBundle) (string, error) {
	if key == nil || key.Key == nil {
		return "", fmt.Errorf("key vault response has no key material")
	}
	if key.Key.Kty == nil {
		return "", fmt.Errorf("key vault response has no key type")
	}
	block := &pem.Block{
		Type:  fmt.Sprintf("%s PRIVATE KEY", *key.Key.Kty),
		Bytes: key.Key.N,
	}
	return string(pem.EncodeToMemory(block)), nil
}

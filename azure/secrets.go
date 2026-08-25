package azure

import (
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// SecretFunc is the azSec template function: azSec <vault> <name> [version].
func (p *Provider) SecretFunc(vaultName string, args ...string) (any, error) {
	switch len(args) {
	case 1:
		return p.Secret(vaultName, args[0], "")
	case 2:
		return p.Secret(vaultName, args[0], args[1])
	default:
		return nil, fmt.Errorf("wrong arguments count for function %q: expected 1 or 2 arguments after the vault name, got %d", "azSec", len(args))
	}
}

// Secret reads one secret. An empty version selects the most recently created
// enabled version whose notBefore is not in the future.
func (p *Provider) Secret(vaultName, name, version string) (string, error) {
	key := cacheKey("secret", vaultName, name, version)
	if v, ok := p.cache.get(key); ok {
		return v.(string), nil
	}

	client, err := p.secretsClient(vaultName)
	if err != nil {
		return "", err
	}

	if version == "" {
		resolved, err := p.latestActiveSecretVersion(client, vaultName, name)
		if err != nil {
			return "", err
		}
		version = resolved
	}

	secret, err := client.GetSecret(p.ctx, name, version, nil)
	if err != nil {
		return "", err
	}
	if secret.Value == nil {
		return "", fmt.Errorf("secret %q in vault %q has no value", name, vaultName)
	}

	// Cache under the version that was asked for and under the one that was
	// resolved, so a later call naming that version hits too.
	p.cache.put(key, *secret.Value)
	p.cache.put(cacheKey("secret", vaultName, name, version), *secret.Value)
	return *secret.Value, nil
}

func (p *Provider) secretsClient(vaultName string) (*azsecrets.Client, error) {
	c, err := p.client("secrets:"+vaultName, func(cred azcore.TokenCredential) (any, error) {
		return azsecrets.NewClient(vaultURL(vaultName), cred, &azsecrets.ClientOptions{
			ClientOptions: azcore.ClientOptions{Transport: p.cfg.transport},
		})
	})
	if err != nil {
		return nil, err
	}
	return c.(*azsecrets.Client), nil
}

// latestActiveSecretVersion returns the newest enabled version whose
// notBefore has passed.
func (p *Provider) latestActiveSecretVersion(client *azsecrets.Client, vaultName, name string) (string, error) {
	now := time.Now().UTC()
	var best *azsecrets.SecretProperties

	pager := client.NewListSecretPropertiesVersionsPager(name, nil)
	for pager.More() {
		page, err := pager.NextPage(p.ctx)
		if err != nil {
			return "", fmt.Errorf("listing versions of secret %q in vault %q: %w", name, vaultName, err)
		}
		for _, item := range page.Value {
			if item == nil || item.Attributes == nil || item.ID == nil {
				continue
			}
			if item.Attributes.Enabled != nil && !*item.Attributes.Enabled {
				continue
			}
			if item.Attributes.NotBefore != nil && item.Attributes.NotBefore.UTC().After(now) {
				continue
			}
			if best == nil || newerSecret(item, best) {
				best = item
			}
		}
	}
	if best == nil {
		return "", fmt.Errorf("no active version found for secret %q in vault %q", name, vaultName)
	}
	return best.ID.Version(), nil
}

func newerSecret(a, b *azsecrets.SecretProperties) bool {
	if a.Attributes.Created == nil {
		return false
	}
	if b.Attributes.Created == nil {
		return true
	}
	return a.Attributes.Created.After(*b.Attributes.Created)
}

// ToPem is the toPem template function: it wraps bytes in a PEM block of the
// given type. It needs no Azure access, so it is a plain function.
func ToPem(blockType, text string) (string, error) {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: []byte(text)})), nil
}

func vaultURL(vaultName string) string {
	return fmt.Sprintf("https://%v.vault.azure.net", vaultName)
}

// cacheKey builds a key from the caller's own arguments. Never key on an ID
// returned by Azure - see cache.go.
func cacheKey(kind string, parts ...string) string {
	return kind + ":" + strings.Join(parts, "/")
}

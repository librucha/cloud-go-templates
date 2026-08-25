package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// StorageKeyFunc is the azStoreKey template function:
// azStoreKey <subscription> <resourceGroup> <account>.
func (p *Provider) StorageKeyFunc(subscriptionID, resourceGroup, account string) (string, error) {
	key := cacheKey("storage", subscriptionID, resourceGroup, account)
	if v, ok := p.cache.get(key); ok {
		return v.(string), nil
	}

	client, err := p.accountsClient(subscriptionID)
	if err != nil {
		return "", err
	}

	keys, err := client.ListKeys(p.ctx, resourceGroup, account, nil)
	if err != nil {
		return "", err
	}
	// The version of this code that shipped in krmgen indexed Keys[0]
	// without checking the length, so an account with no keys panicked and
	// took the whole process down. This is the one deliberate behaviour
	// change in this phase: name the account in an error instead.
	if len(keys.Keys) == 0 || keys.Keys[0].Value == nil {
		return "", fmt.Errorf("storage account %q in resource group %q has no keys", account, resourceGroup)
	}

	value := *keys.Keys[0].Value
	p.cache.put(key, value)
	return value, nil
}

func (p *Provider) accountsClient(subscriptionID string) (*armstorage.AccountsClient, error) {
	c, err := p.client("storage:"+subscriptionID, func(cred azcore.TokenCredential) (any, error) {
		return armstorage.NewAccountsClient(subscriptionID, cred, &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{Transport: p.cfg.transport},
		})
	})
	if err != nil {
		return nil, err
	}
	return c.(*armstorage.AccountsClient), nil
}

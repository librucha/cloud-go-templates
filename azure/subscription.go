package azure

import (
	"errors"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

// subscriptionID resolves the subscription to use: the one passed to
// WithSubscriptionID, else the AZURE_SUBSCRIPTION_ID environment variable,
// else the first subscription the credential can see.
func (p *Provider) subscriptionID() (string, error) {
	if p.cfg.subscriptionID != "" {
		return p.cfg.subscriptionID, nil
	}
	if id := os.Getenv("AZURE_SUBSCRIPTION_ID"); id != "" {
		return id, nil
	}

	client, err := p.subscriptionsClient()
	if err != nil {
		return "", err
	}

	pager := client.NewListPager(nil)
	if pager.More() {
		page, err := pager.NextPage(p.ctx)
		if err != nil {
			return "", err
		}
		if len(page.Value) > 0 && page.Value[0].SubscriptionID != nil {
			return *page.Value[0].SubscriptionID, nil
		}
	}

	return "", errors.New("no subscriptions found: pass WithSubscriptionID or set AZURE_SUBSCRIPTION_ID")
}

func (p *Provider) subscriptionsClient() (*armsubscriptions.Client, error) {
	c, err := p.client("subscriptions", func(cred azcore.TokenCredential) (any, error) {
		return armsubscriptions.NewClient(cred, &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{Transport: p.cfg.transport},
		})
	})
	if err != nil {
		return nil, err
	}
	return c.(*armsubscriptions.Client), nil
}

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
//
// The result is memoized behind a sync.Once, the same pattern credential()
// uses. Without it, every cache-missed call that needs a subscription ID
// (chiefly UserIdentityClientIDFunc) and finds neither WithSubscriptionID nor
// AZURE_SUBSCRIPTION_ID set would call NewListPager again - the original
// resolved this once per client construction; this restores that.
func (p *Provider) subscriptionID() (string, error) {
	p.subIDOnce.Do(func() {
		p.subID, p.subIDErr = p.resolveSubscriptionID()
	})
	return p.subID, p.subIDErr
}

func (p *Provider) resolveSubscriptionID() (string, error) {
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

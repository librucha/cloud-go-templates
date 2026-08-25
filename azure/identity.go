package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
)

// UserIdentityClientIDFunc is the azUserIdentityClientId template function:
// azUserIdentityClientId <resourceGroup> <name>. It resolves user-assigned
// identities only; system-assigned ones are addressed by scope and would
// need a different function.
func (p *Provider) UserIdentityClientIDFunc(resourceGroup, name string) (string, error) {
	key := cacheKey("identity", resourceGroup, name)
	if v, ok := p.cache.get(key); ok {
		return v.(string), nil
	}

	subscriptionID, err := p.subscriptionID()
	if err != nil {
		return "", err
	}

	client, err := p.userAssignedIdentitiesClient(subscriptionID)
	if err != nil {
		return "", err
	}

	identity, err := client.Get(p.ctx, resourceGroup, name, nil)
	if err != nil {
		return "", err
	}
	// Properties.ClientID is a *string in the SDK response, so returning a
	// plain string requires a nil check somewhere. krmgen's version let a
	// nil pointer flow into the "any" it returned, relying on text/template
	// to print it as "<nil>"; here the check happens in the function itself
	// instead, and reports which identity was missing a client ID.
	if identity.Properties == nil || identity.Properties.ClientID == nil {
		return "", fmt.Errorf("identity %q in resource group %q has no client ID", name, resourceGroup)
	}

	clientID := *identity.Properties.ClientID
	p.cache.put(key, clientID)
	return clientID, nil
}

func (p *Provider) userAssignedIdentitiesClient(subscriptionID string) (*armmsi.UserAssignedIdentitiesClient, error) {
	c, err := p.client("identity:"+subscriptionID, func(cred azcore.TokenCredential) (any, error) {
		return armmsi.NewUserAssignedIdentitiesClient(subscriptionID, cred, &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{Transport: p.cfg.transport},
		})
	})
	if err != nil {
		return nil, err
	}
	return c.(*armmsi.UserAssignedIdentitiesClient), nil
}

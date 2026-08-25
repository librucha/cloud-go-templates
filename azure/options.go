package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// Option configures a Provider.
type Option func(*config)

type config struct {
	credential     azcore.TokenCredential
	subscriptionID string
	transport      policy.Transporter
}

// WithCredential supplies the credential every Azure client will use. Without
// it the provider falls back to azidentity.NewDefaultAzureCredential, resolved
// lazily on first use so that constructing a Provider never touches the
// network or the filesystem.
func WithCredential(cred azcore.TokenCredential) Option {
	return func(c *config) { c.credential = cred }
}

// WithSubscriptionID pins the subscription used by the functions that need
// one. Without it the provider reads AZURE_SUBSCRIPTION_ID, and failing that
// asks Azure for the first subscription the credential can see.
func WithSubscriptionID(id string) Option {
	return func(c *config) { c.subscriptionID = id }
}

// withTransport is unexported: it exists so the package's own tests can point
// every client at a stub. Consumers configure transports through their
// credential or environment instead.
func withTransport(t policy.Transporter) Option {
	return func(c *config) { c.transport = t }
}

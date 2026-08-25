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

// WithTransport supplies the HTTP transport every Azure client is built
// with. This is the hook for testing a consumer's own code without touching
// the network or needing Azure credentials: point it at a stub that answers
// requests in-process (see the package's own testing_test.go for an example)
// and every client the Provider builds - secrets, certificates, keys,
// storage, identities, subscriptions - uses it instead of dialing out.
//
// A single Transporter is shared by every client kind. If you need
// per-service transport options (proxies, retry policy, telemetry) beyond a
// single RoundTripper-style hook, wrap your Transporter to dispatch on the
// request instead; the Provider does not expose per-client-kind
// configuration.
func WithTransport(t policy.Transporter) Option {
	return func(c *config) { c.transport = t }
}

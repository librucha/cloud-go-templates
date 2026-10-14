package azure

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// graphEndpoint and graphScope address Microsoft Graph in the public cloud.
// Graph is called over plain REST through an azcore pipeline rather than
// through msgraph-sdk-go: one GET does not justify that SDK's dependency
// graph, and the pipeline still honours WithTransport and the credential.
const (
	graphEndpoint = "https://graph.microsoft.com/v1.0"
	graphScope    = "https://graph.microsoft.com/.default"
)

// EntraAppClientIDFunc is the entraAppClientId template function, also
// exposed as entraApplicationClientId: entraAppClientId <displayName>. It
// returns the appId (the client ID) of the Entra ID application registration
// with that display name.
//
// Display names are not unique in Entra ID, so a name matching no
// application or more than one is an error rather than a guess. The
// credential needs Application.Read.All (or must own the application) on
// Microsoft Graph.
func (p *Provider) EntraAppClientIDFunc(displayName string) (string, error) {
	key := cacheKey("entraApp", displayName)
	if v, ok := p.cache.get(key); ok {
		return v.(string), nil
	}

	pl, err := p.graphPipeline()
	if err != nil {
		return "", err
	}

	// OData string literals escape a single quote by doubling it. Spaces go
	// out as %20: QueryEscape's "+" is not a space to every OData parser.
	filter := "displayName eq '" + strings.ReplaceAll(displayName, "'", "''") + "'"
	endpoint := graphEndpoint + "/applications?$filter=" +
		strings.ReplaceAll(url.QueryEscape(filter), "+", "%20") + "&$select=appId"

	req, err := runtime.NewRequest(p.ctx, http.MethodGet, endpoint)
	if err != nil {
		return "", err
	}
	resp, err := pl.Do(req)
	if err != nil {
		return "", err
	}
	// NewResponseError and UnmarshalAsJSON both drain and close the body;
	// this covers any path that returns before either runs.
	defer func() { _ = resp.Body.Close() }()
	if !runtime.HasStatusCode(resp, http.StatusOK) {
		return "", runtime.NewResponseError(resp)
	}

	var page struct {
		Value []struct {
			AppID *string `json:"appId"`
		} `json:"value"`
	}
	if err := runtime.UnmarshalAsJSON(resp, &page); err != nil {
		return "", err
	}
	switch {
	case len(page.Value) == 0:
		return "", fmt.Errorf("no Entra application is named %q", displayName)
	case len(page.Value) > 1:
		return "", fmt.Errorf("%d Entra applications are named %q, want exactly one", len(page.Value), displayName)
	case page.Value[0].AppID == nil || *page.Value[0].AppID == "":
		return "", fmt.Errorf("the Entra application named %q has no appId", displayName)
	}

	clientID := *page.Value[0].AppID
	p.cache.put(key, clientID)
	return clientID, nil
}

func (p *Provider) graphPipeline() (runtime.Pipeline, error) {
	c, err := p.client("graph", func(cred azcore.TokenCredential) (any, error) {
		return runtime.NewPipeline("cloud-go-templates", "", runtime.PipelineOptions{
			PerRetry: []policy.Policy{runtime.NewBearerTokenPolicy(cred, []string{graphScope}, nil)},
		}, &policy.ClientOptions{
			Transport: p.cfg.transport,
			// There is no SDK module version to report; a User-Agent naming
			// one would be invented.
			Telemetry: policy.TelemetryOptions{Disabled: true},
		}), nil
	})
	if err != nil {
		return runtime.Pipeline{}, err
	}
	return c.(runtime.Pipeline), nil
}

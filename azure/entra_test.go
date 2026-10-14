package azure

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

const entraAppResponse = `{"value":[{"appId":"22222222-2222-2222-2222-222222222222"}]}`

// scopeRecordingCredential is fakeCredential that also remembers the scopes
// it was asked for, so a test can prove Graph is called with a Graph token
// rather than an ARM one.
type scopeRecordingCredential struct {
	mu     sync.Mutex
	scopes []string
}

func (c *scopeRecordingCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.mu.Lock()
	c.scopes = append(c.scopes, opts.Scopes...)
	c.mu.Unlock()
	return azcore.AccessToken{Token: "fake_token", ExpiresOn: time.Now().Add(time.Hour).UTC()}, nil
}

// TestEntraAppClientIDFunc_ReturnsAppID also pins the request: a Graph
// applications query filtered on the display name, selecting only appId,
// authorized with a token for the Graph scope.
func TestEntraAppClientIDFunc_ReturnsAppID(t *testing.T) {
	var got *http.Request
	sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
		got = r
		return mockResponse(200, entraAppResponse), nil
	}}
	cred := &scopeRecordingCredential{}
	p := newTestProvider(t, sender, WithCredential(cred))

	clientID, err := p.EntraAppClientIDFunc("my app")
	if err != nil {
		t.Fatalf("EntraAppClientIDFunc() error = %v", err)
	}
	if clientID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("got %q, want the appId from the response", clientID)
	}

	if got.URL.Host != "graph.microsoft.com" || got.URL.Path != "/v1.0/applications" {
		t.Errorf("request went to %s%s, want graph.microsoft.com/v1.0/applications", got.URL.Host, got.URL.Path)
	}
	if f := got.URL.Query().Get("$filter"); f != "displayName eq 'my app'" {
		t.Errorf("$filter = %q, want %q", f, "displayName eq 'my app'")
	}
	if strings.Contains(got.URL.RawQuery, "+") {
		t.Errorf("raw query %q encodes a space as +, want %%20", got.URL.RawQuery)
	}
	if s := got.URL.Query().Get("$select"); s != "appId" {
		t.Errorf("$select = %q, want appId", s)
	}
	if a := got.Header.Get("Authorization"); a != "Bearer fake_token" {
		t.Errorf("Authorization = %q, want the credential's bearer token", a)
	}
	if len(cred.scopes) != 1 || cred.scopes[0] != "https://graph.microsoft.com/.default" {
		t.Errorf("token scopes = %v, want only the Graph scope", cred.scopes)
	}
}

// TestEntraAppClientIDFunc_EscapesQuotes proves a display name containing a
// single quote cannot end the OData string literal early.
func TestEntraAppClientIDFunc_EscapesQuotes(t *testing.T) {
	var filter string
	sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
		filter = r.URL.Query().Get("$filter")
		return mockResponse(200, entraAppResponse), nil
	}}
	p := newTestProvider(t, sender)

	if _, err := p.EntraAppClientIDFunc("o'brien's app"); err != nil {
		t.Fatalf("EntraAppClientIDFunc() error = %v", err)
	}
	if want := "displayName eq 'o''brien''s app'"; filter != want {
		t.Errorf("$filter = %q, want %q", filter, want)
	}
}

func TestEntraAppClientIDFunc_NoMatchIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"value":[]}`), nil
	}}
	p := newTestProvider(t, sender)

	_, err := p.EntraAppClientIDFunc("missing")
	if err == nil {
		t.Fatal("EntraAppClientIDFunc() error = nil, want an error when no application has the name")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to name the display name", err)
	}
}

// Display names are not unique in Entra ID. Picking the first match would
// render whichever application Graph happened to list first.
func TestEntraAppClientIDFunc_AmbiguousNameIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"value":[{"appId":"a"},{"appId":"b"}]}`), nil
	}}
	p := newTestProvider(t, sender)

	_, err := p.EntraAppClientIDFunc("dup")
	if err == nil {
		t.Fatal("EntraAppClientIDFunc() error = nil, want an error when two applications share the name")
	}
	if !strings.Contains(err.Error(), "2") || !strings.Contains(err.Error(), "dup") {
		t.Errorf("error = %v, want it to name the match count and the display name", err)
	}
}

func TestEntraAppClientIDFunc_MissingAppIDIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"value":[{}]}`), nil
	}}
	p := newTestProvider(t, sender)

	if _, err := p.EntraAppClientIDFunc("app"); err == nil {
		t.Fatal("EntraAppClientIDFunc() error = nil, want an error when the application has no appId")
	}
}

func TestEntraAppClientIDFunc_PropagatesFailure(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(http.StatusForbidden, `{"error":{"code":"Authorization_RequestDenied","message":"Insufficient privileges to complete the operation."}}`), nil
	}}
	p := newTestProvider(t, sender)

	_, err := p.EntraAppClientIDFunc("app")
	if err == nil {
		t.Fatal("EntraAppClientIDFunc() error = nil, want the Graph failure to propagate")
	}
	if !strings.Contains(err.Error(), "Authorization_RequestDenied") {
		t.Errorf("error = %v, want it to carry Graph's error code", err)
	}
}

func TestEntraAppClientIDFunc_CachesRepeatedLookups(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, entraAppResponse), nil
	}}
	p := newTestProvider(t, sender)

	first, err := p.EntraAppClientIDFunc("app")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := p.EntraAppClientIDFunc("app")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if first != second {
		t.Errorf("got %v then %v, want the same client id", first, second)
	}
	if sender.calls != 1 {
		t.Errorf("Graph was queried %d times, want once", sender.calls)
	}
}

// TestEntraAppClientIDFunc_BothTemplateNamesRender renders the function
// through text/template under its name and its alias, sharing one cache.
func TestEntraAppClientIDFunc_BothTemplateNamesRender(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, entraAppResponse), nil
	}}
	p := newTestProvider(t, sender)

	tmpl := template.Must(template.New("t").Funcs(p.FuncMap()).Parse(
		`{{ entraAppClientId "app" }} {{ entraApplicationClientId "app" }}`))
	var out strings.Builder
	if err := tmpl.Execute(&out, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "22222222-2222-2222-2222-222222222222 22222222-2222-2222-2222-222222222222"
	if out.String() != want {
		t.Errorf("rendered %q, want %q", out.String(), want)
	}
	if sender.calls != 1 {
		t.Errorf("Graph was queried %d times, want once", sender.calls)
	}
}

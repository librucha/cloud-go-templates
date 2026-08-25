package azure

import (
	"net/http"
	"strings"
	"testing"
)

// TestUserIdentityClientIDFunc_ReturnsClientID ports krmgen's
// TestGetClientId_ReturnsClientIdAndCaches.
func TestUserIdentityClientIDFunc_ReturnsClientID(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id","properties":{"clientId":"11111111-1111-1111-1111-111111111111"}}`), nil
	}}
	p := newTestProvider(t, sender, WithSubscriptionID("sub"))

	got, err := p.UserIdentityClientIDFunc("rg", "id")
	if err != nil {
		t.Fatalf("UserIdentityClientIDFunc() error = %v", err)
	}
	if got != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("got %v, want the client id from the response", got)
	}
	if sender.calls != 1 {
		t.Errorf("the identity was queried %d times, want once", sender.calls)
	}
}

// TestUserIdentityClientIDFunc_CachesRepeatedLookups proves a second lookup
// for the same resource group/name is served from the cache rather than the
// identity.
func TestUserIdentityClientIDFunc_CachesRepeatedLookups(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id","properties":{"clientId":"11111111-1111-1111-1111-111111111111"}}`), nil
	}}
	p := newTestProvider(t, sender, WithSubscriptionID("sub"))

	first, err := p.UserIdentityClientIDFunc("rg", "id")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := p.UserIdentityClientIDFunc("rg", "id")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if first != second {
		t.Errorf("got %v then %v, want the same client id", first, second)
	}
	if sender.calls != 1 {
		t.Errorf("the identity was queried %d times, want once", sender.calls)
	}
}

// TestUserIdentityClientIDFunc_NilClientIDIsAnError proves an identity whose
// Properties has no ClientID is reported as an error. krmgen's original code
// let a nil pointer flow into the "any" it returned, relying on
// text/template to print it as "<nil>"; this is a real behaviour change (see
// docs/specification.md in krmgen), the only one of the four that turns a
// previously succeeding render into a failure.
func TestUserIdentityClientIDFunc_NilClientIDIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id","properties":{}}`), nil
	}}
	p := newTestProvider(t, sender, WithSubscriptionID("sub"))

	_, err := p.UserIdentityClientIDFunc("rg", "id")
	if err == nil {
		t.Fatal("UserIdentityClientIDFunc() error = nil, want an error when the identity has no client ID")
	}
	if !strings.Contains(err.Error(), "id") || !strings.Contains(err.Error(), "rg") {
		t.Errorf("error = %v, want it to name the identity and resource group", err)
	}
}

// TestUserIdentityClientIDFunc_NilPropertiesIsAnError proves an identity
// response with no Properties block at all is reported as an error rather
// than a nil-pointer dereference.
func TestUserIdentityClientIDFunc_NilPropertiesIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"id":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id"}`), nil
	}}
	p := newTestProvider(t, sender, WithSubscriptionID("sub"))

	if _, err := p.UserIdentityClientIDFunc("rg", "id"); err == nil {
		t.Fatal("UserIdentityClientIDFunc() error = nil, want an error when the identity has no properties")
	}
}

// TestUserIdentityClientIDFunc_PropagatesFailure ports krmgen's implicit
// error propagation from GetClientId/getIdentity.
func TestUserIdentityClientIDFunc_PropagatesFailure(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(http.StatusNotFound, `{"error":{"code":"ResourceNotFound","message":"identity not found"}}`), nil
	}}
	p := newTestProvider(t, sender, WithSubscriptionID("sub"))

	if _, err := p.UserIdentityClientIDFunc("rg", "missing"); err == nil {
		t.Fatal("UserIdentityClientIDFunc() error = nil, want the Azure failure to propagate")
	}
}

package azure

import (
	"net/http"
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

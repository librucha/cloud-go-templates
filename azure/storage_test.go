package azure

import (
	"net/http"
	"strings"
	"testing"
)

// TestStorageKeyFunc_ReturnsFirstKey ports krmgen's TestGetStoreKey_ReturnsFirstKeyAndCaches:
// the account's first key is returned.
func TestStorageKeyFunc_ReturnsFirstKey(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"keys":[{"keyName":"key1","value":"first"},{"keyName":"key2","value":"second"}]}`), nil
	}}
	p := newTestProvider(t, sender)

	got, err := p.StorageKeyFunc("sub", "rg", "account")
	if err != nil {
		t.Fatalf("StorageKeyFunc() error = %v", err)
	}
	if got != "first" {
		t.Errorf("got %q, want the first key", got)
	}
}

// The version of this code that shipped in krmgen indexed Keys[0] without
// checking the length, so an account with no keys panicked and took the whole
// process down. This is the one deliberate behaviour change in this phase.
func TestStorageKeyFunc_NoKeysIsAnErrorNotAPanic(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"keys":[]}`), nil
	}}
	p := newTestProvider(t, sender)

	_, err := p.StorageKeyFunc("sub", "rg", "account")
	if err == nil {
		t.Fatal("StorageKeyFunc() error = nil, want an error for an account with no keys")
	}
	if !strings.Contains(err.Error(), "account") {
		t.Errorf("error = %v, want it to name the storage account", err)
	}
}

// TestStorageKeyFunc_PropagatesFailure ports krmgen's TestGetStoreKey_PropagatesFailure.
func TestStorageKeyFunc_PropagatesFailure(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(http.StatusNotFound, `{"error":{"code":"ResourceNotFound","message":"account not found"}}`), nil
	}}
	p := newTestProvider(t, sender)

	if _, err := p.StorageKeyFunc("sub", "rg", "missing"); err == nil {
		t.Fatal("StorageKeyFunc() error = nil, want the Azure failure to propagate")
	}
}

// TestStorageKeyFunc_CachesRepeatedLookups proves a second lookup for the
// same subscription/resource group/account is served from the cache rather
// than the account.
func TestStorageKeyFunc_CachesRepeatedLookups(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"keys":[{"keyName":"key1","value":"first"}]}`), nil
	}}
	p := newTestProvider(t, sender)

	first, err := p.StorageKeyFunc("sub", "rg", "account")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := p.StorageKeyFunc("sub", "rg", "account")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if first != second {
		t.Errorf("got %v then %v, want the same key", first, second)
	}
	if sender.calls != 1 {
		t.Errorf("the account was called %d times, want once", sender.calls)
	}
}

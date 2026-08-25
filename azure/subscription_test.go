package azure

import (
	"net/http"
	"testing"
)

// TestSubscriptionID_PrefersConfiguredOverEverything proves the configured
// subscription wins even when AZURE_SUBSCRIPTION_ID is also set, and that
// neither path calls Azure.
func TestSubscriptionID_PrefersConfiguredOverEverything(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "env-sub")
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		t.Fatal("subscriptionID() called Azure despite a configured subscription")
		return nil, nil
	}}
	p := newTestProvider(t, sender, WithSubscriptionID("configured-sub"))

	got, err := p.subscriptionID()
	if err != nil {
		t.Fatalf("subscriptionID() error = %v", err)
	}
	if got != "configured-sub" {
		t.Errorf("got %q, want the configured subscription", got)
	}
}

// TestSubscriptionID_FallsBackToEnvVar proves AZURE_SUBSCRIPTION_ID is used
// when no subscription is configured, without calling Azure.
func TestSubscriptionID_FallsBackToEnvVar(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "env-sub")
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		t.Fatal("subscriptionID() called Azure despite AZURE_SUBSCRIPTION_ID being set")
		return nil, nil
	}}
	p := newTestProvider(t, sender)

	got, err := p.subscriptionID()
	if err != nil {
		t.Fatalf("subscriptionID() error = %v", err)
	}
	if got != "env-sub" {
		t.Errorf("got %q, want the environment variable's subscription", got)
	}
}

// TestSubscriptionID_FallsBackToAzureList proves that with nothing configured
// and no environment variable set, the first subscription Azure lists is
// used.
func TestSubscriptionID_FallsBackToAzureList(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "")
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"value":[{"subscriptionId":"first-sub"},{"subscriptionId":"second-sub"}]}`), nil
	}}
	p := newTestProvider(t, sender)

	got, err := p.subscriptionID()
	if err != nil {
		t.Fatalf("subscriptionID() error = %v", err)
	}
	if got != "first-sub" {
		t.Errorf("got %q, want the first subscription Azure listed", got)
	}
}

// TestSubscriptionID_NoSubscriptionsIsAnError proves an empty listing is
// reported as an error rather than returning a zero-value subscription ID.
func TestSubscriptionID_NoSubscriptionsIsAnError(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "")
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"value":[]}`), nil
	}}
	p := newTestProvider(t, sender)

	if _, err := p.subscriptionID(); err == nil {
		t.Fatal("subscriptionID() error = nil, want an error when Azure lists no subscriptions")
	}
}

package azure

import (
	"context"
	"net/http"
	"testing"
)

func TestNew_RequiresContext(t *testing.T) {
	if _, err := New(nil); err == nil { //nolint:staticcheck // passing nil is the thing under test
		t.Error("New(nil) error = nil, want an error - the context is not optional")
	}
}

func TestFuncMap_ExposesEveryFunctionUnderItsTemplateName(t *testing.T) {
	p, err := New(context.Background(), WithCredential(fakeCredential{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	funcs := p.FuncMap()
	// Later tasks extend this list and the test with it.
	for _, name := range []string{"azSec", "toPem", "azPfxKey", "azPfxCrt", "azCert", "azKey"} {
		if _, ok := funcs[name]; !ok {
			t.Errorf("FuncMap() is missing %q", name)
		}
	}
}

func TestProvider_CachesRepeatedLookups(t *testing.T) {
	sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
		return mockResponse(200, `{"id":"https://fake.vault.io/secrets/sec/ver1","value":"cached"}`), nil
	}}
	p := newTestProvider(t, sender)

	for i := 0; i < 2; i++ {
		got, err := p.Secret("vault", "sec", "ver1")
		if err != nil {
			t.Fatalf("call %d failed: %v", i+1, err)
		}
		if got != "cached" {
			t.Errorf("call %d = %q, want %q", i+1, got, "cached")
		}
	}
	if sender.calls != 1 {
		t.Errorf("the vault was called %d times, want once", sender.calls)
	}
}

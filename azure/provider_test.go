package azure

import (
	"context"
	"net/http"
	"reflect"
	"sort"
	"testing"
)

func TestNew_RequiresContext(t *testing.T) {
	if _, err := New(nil); err == nil { //nolint:staticcheck // passing nil is the thing under test
		t.Error("New(nil) error = nil, want an error - the context is not optional")
	}
}

// TestFuncMap_ExposesEveryFunctionUnderItsTemplateName pins the FuncMap's
// exact contents. It is the library's public surface: a function appearing
// or vanishing is a change to that surface and should fail this test, not
// surprise a consumer.
func TestFuncMap_ExposesEveryFunctionUnderItsTemplateName(t *testing.T) {
	p, err := New(context.Background(), WithCredential(fakeCredential{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	want := []string{
		"azSec", "toPem", "azPfxKey", "azPfxCrt",
		"azCert", "azKey", "azStoreKey", "azUserIdentityClientId",
		"entraAppClientId", "entraApplicationClientId",
	}
	funcs := p.FuncMap()
	for _, name := range want {
		if _, ok := funcs[name]; !ok {
			t.Errorf("FuncMap() is missing %q", name)
		}
	}
	if len(funcs) != len(want) {
		got := make([]string, 0, len(funcs))
		for name := range funcs {
			got = append(got, name)
		}
		sort.Strings(got)
		t.Errorf("FuncMap() exposes %d functions %v, want exactly %d %v", len(funcs), got, len(want), want)
	}
}

// TestFuncMap_BindsEachNameToItsOwnFunction closes the gap left by
// TestFuncMap_ExposesEveryFunctionUnderItsTemplateName above: that test only
// proves every name is registered, not what it is bound to. Two names of the
// same Go signature (azCert/azKey both take (string, ...string) (any,
// error)) can be swapped in FuncMap() and the exposure test - and every
// other test in this repository and in krmgen - still passes, because
// nothing calls them through a mismatched name. This test compares the code
// pointer behind each map entry to the code pointer of the method it is
// supposed to be, so a swap fails here even though it type-checks and even
// though every behavioural test stays green.
func TestFuncMap_BindsEachNameToItsOwnFunction(t *testing.T) {
	p, err := New(context.Background(), WithCredential(fakeCredential{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	funcs := p.FuncMap()

	ptr := func(fn any) uintptr { return reflect.ValueOf(fn).Pointer() }

	want := map[string]uintptr{
		"azSec":                  ptr(p.SecretFunc),
		"toPem":                  ptr(ToPem),
		"azPfxKey":               ptr(p.PfxKeyFunc),
		"azPfxCrt":               ptr(p.PfxCertFunc),
		"azCert":                 ptr(p.CertificateFunc),
		"azKey":                  ptr(p.KeyFunc),
		"azStoreKey":             ptr(p.StorageKeyFunc),
		"azUserIdentityClientId": ptr(p.UserIdentityClientIDFunc),

		"entraAppClientId":         ptr(p.EntraAppClientIDFunc),
		"entraApplicationClientId": ptr(p.EntraAppClientIDFunc),
	}
	for name, wantPtr := range want {
		fn, ok := funcs[name]
		if !ok {
			t.Errorf("FuncMap() is missing %q", name)
			continue
		}
		if got := ptr(fn); got != wantPtr {
			t.Errorf("FuncMap()[%q] is bound to a different function than expected (code pointer %#x, want %#x)", name, got, wantPtr)
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

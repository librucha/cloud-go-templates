package azure

import (
	"net/http"
	"strings"
	"testing"
)

// TestKeyFunc_LookupsAndErrors ports krmgen's TestResolveKey: a version-less
// lookup, an explicit-version lookup, and the wrong-arity error.
func TestKeyFunc_LookupsAndErrors(t *testing.T) {
	tests := []struct {
		name    string
		keyArgs []string
		resBody string
		wantErr bool
	}{
		{
			name:    "valid key without version",
			keyArgs: []string{"k"},
			resBody: `{"key":{"kid":"https://fake.vault.io/keys/k/ver1","kty":"RSA","n":"AQAB","e":"AQAB"}}`,
		},
		{
			name:    "valid key with version",
			keyArgs: []string{"k", "ver1"},
			resBody: `{"key":{"kid":"https://fake.vault.io/keys/k/ver1","kty":"RSA","n":"AQAB","e":"AQAB"}}`,
		},
		{
			name:    "wrong argument count",
			keyArgs: []string{"k", "ver1", "extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
				return mockResponse(http.StatusOK, tt.resBody), nil
			}}
			p := newTestProvider(t, sender)

			got, err := p.KeyFunc("vault_name", tt.keyArgs...)
			if (err != nil) != tt.wantErr {
				t.Errorf("KeyFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !strings.Contains(got.(string), "-----BEGIN RSA PRIVATE KEY-----") {
				t.Errorf("KeyFunc() got = %q, want the RSA PRIVATE KEY header today's code emits", got)
			}
		})
	}
}

// TestKeyFunc_ReturnsModulusUnderAPrivateKeyHeader documents a deliberately
// preserved oddity: the payload PEM-wrapped here is the RSA modulus (the
// public part of the key pair), not a private key, despite the header text.
// This has been krmgen's behaviour all along; correcting it would change
// rendered output, which this port must not do.
func TestKeyFunc_ReturnsModulusUnderAPrivateKeyHeader(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"key":{"kid":"https://fake.vault.io/keys/k/v1","kty":"RSA","n":"AQAB","e":"AQAB"}}`), nil
	}}
	p := newTestProvider(t, sender)

	got, err := p.KeyFunc("vault", "k", "v1")
	if err != nil {
		t.Fatalf("KeyFunc() error = %v", err)
	}
	// Documented oddity, preserved deliberately: the payload is the RSA
	// modulus, not a private key, despite the header.
	if !strings.Contains(got.(string), "BEGIN RSA PRIVATE KEY") {
		t.Errorf("got %q, want the RSA PRIVATE KEY header today's code emits", got)
	}
	if sender.calls != 1 {
		t.Errorf("the vault was called %d times, want once", sender.calls)
	}
}

// TestKeyFunc_NoKeyMaterialIsAnError proves a Key Vault response with no key
// material (key.Key == nil) is reported as an error rather than a
// nil-pointer dereference. This is the one remaining unusual-response path
// that dereferenced unchecked before this phase converted it, alongside the
// storage and identity paths, from a process-killing panic to an error.
func TestKeyFunc_NoKeyMaterialIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{}`), nil
	}}
	p := newTestProvider(t, sender)

	_, err := p.KeyFunc("vault", "k", "v1")
	if err == nil {
		t.Fatal("KeyFunc() error = nil, want an error when the response has no key material")
	}
	if !strings.Contains(err.Error(), "k") || !strings.Contains(err.Error(), "vault") {
		t.Errorf("error = %v, want it to name the key and vault", err)
	}
}

// TestKeyFunc_NoKeyTypeIsAnError proves a Key Vault response whose key has no
// Kty is reported as an error rather than a nil-pointer dereference.
func TestKeyFunc_NoKeyTypeIsAnError(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"key":{"kid":"https://fake.vault.io/keys/k/v1","n":"AQAB","e":"AQAB"}}`), nil
	}}
	p := newTestProvider(t, sender)

	if _, err := p.KeyFunc("vault", "k", "v1"); err == nil {
		t.Fatal("KeyFunc() error = nil, want an error when the key has no key type")
	}
}

// TestKeyFunc_CachesRepeatedLookups proves a second lookup for the same
// vault/name/version is served from the cache rather than the vault.
func TestKeyFunc_CachesRepeatedLookups(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"key":{"kid":"https://fake.vault.io/keys/k/v1","kty":"RSA","n":"AQAB","e":"AQAB"}}`), nil
	}}
	p := newTestProvider(t, sender)

	first, err := p.KeyFunc("vault", "k", "v1")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := p.KeyFunc("vault", "k", "v1")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if first != second {
		t.Errorf("got %v then %v, want the same key", first, second)
	}
	if sender.calls != 1 {
		t.Errorf("the vault was called %d times, want once", sender.calls)
	}
}

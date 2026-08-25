package azure

import (
	"net/http"
	"strings"
	"testing"
)

// TestCertificateFunc_LookupsAndErrors ports krmgen's TestResolveCert: a
// version-less lookup, an explicit-version lookup, and the wrong-arity error.
func TestCertificateFunc_LookupsAndErrors(t *testing.T) {
	tests := []struct {
		name    string
		keyArgs []string
		resBody string
		wantErr bool
	}{
		{
			name:    "valid certificate without version",
			keyArgs: []string{"crt"},
			resBody: `{"id":"https://fake.vault.io/certificates/crt/ver1","cer":"YmFzZTY0Y2VydA=="}`,
		},
		{
			name:    "valid certificate with version",
			keyArgs: []string{"crt", "ver1"},
			resBody: `{"id":"https://fake.vault.io/certificates/crt/ver1","cer":"YmFzZTY0Y2VydA=="}`,
		},
		{
			name:    "wrong argument count",
			keyArgs: []string{"crt", "ver1", "extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
				return mockResponse(http.StatusOK, tt.resBody), nil
			}}
			p := newTestProvider(t, sender)

			got, err := p.CertificateFunc("vault_name", tt.keyArgs...)
			if (err != nil) != tt.wantErr {
				t.Errorf("CertificateFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !strings.Contains(got.(string), "-----BEGIN CERTIFICATE-----") {
				t.Errorf("CertificateFunc() got = %q, want a PEM certificate", got)
			}
		})
	}
}

// TestCertificateFunc_CachesRepeatedLookups proves a second lookup for the
// same vault/name/version is served from the cache rather than the vault.
func TestCertificateFunc_CachesRepeatedLookups(t *testing.T) {
	sender := &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{"id":"https://fake.vault.io/certificates/crt/v1","cer":"YmFzZTY0Y2VydA=="}`), nil
	}}
	p := newTestProvider(t, sender)

	first, err := p.CertificateFunc("vault", "crt", "v1")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := p.CertificateFunc("vault", "crt", "v1")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if first != second {
		t.Errorf("got %v then %v, want the same certificate", first, second)
	}
	if !strings.Contains(first.(string), "BEGIN CERTIFICATE") {
		t.Errorf("got %q, want a PEM certificate", first)
	}
	if sender.calls != 1 {
		t.Errorf("the vault was called %d times, want once", sender.calls)
	}
}

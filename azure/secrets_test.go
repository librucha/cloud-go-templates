package azure

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// TestSecretFunc_LookupsAndErrors ports krmgen's TestGetSecret: a version-less
// lookup, an explicit-version lookup, and the two "not found" cases (unknown
// secret, unknown version).
func TestSecretFunc_LookupsAndErrors(t *testing.T) {
	tests := []struct {
		name      string
		keyArgs   []string
		listBody  string // response body for the /versions request (version-less lookups only)
		resBody   string // response body for the GetSecret request
		resStatus int
		want      string
		wantErr   bool
	}{
		{
			name:     "valid secret",
			keyArgs:  []string{"key_id"},
			listBody: `{"value":[{"id":"https://fake.vault.io/secrets/key_id/ver1","attributes":{"enabled":true,"created":1000000000}}]}`,
			resBody:  `{"id":"https://fake.vault.io/secrets/key_id/ver1","value":"secretValue"}`,
			want:     "secretValue",
		},
		{
			name:    "secret with version",
			keyArgs: []string{"key_id", "key_version"},
			resBody: `{"id":"https://vault_name.vault.azure.net/key_id/key_version/","value":"secretValueV1"}`,
			want:    "secretValueV1",
		},
		{
			name:      "unknown secret",
			keyArgs:   []string{"unknown_key_id"},
			resStatus: http.StatusNotFound,
			resBody:   `{"error":{"code":"SecretNotFound","message":"Secret unknown_key_id not found"}}`,
			wantErr:   true,
		},
		{
			name:      "unknown secret version",
			keyArgs:   []string{"key_id", "unknown_key_version"},
			resStatus: http.StatusNotFound,
			resBody:   `{"error":{"code":"SecretNotFound","message":"Secret key_id/unknown_key_version not found"}}`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.resStatus
			if status == 0 {
				status = http.StatusOK
			}
			sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.Path, "/versions") {
					return mockResponse(http.StatusOK, tt.listBody), nil
				}
				return mockResponse(status, tt.resBody), nil
			}}
			p := newTestProvider(t, sender)

			got, err := p.SecretFunc("vault_name", tt.keyArgs...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SecretFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SecretFunc() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSecret_LatestActiveVersion ports krmgen's TestGetLatestActiveSecret: the
// version-selection rules for a version-less lookup - a disabled version is
// skipped, a version whose notBefore is in the future is skipped, and the
// most recently created of the survivors wins.
func TestSecret_LatestActiveVersion(t *testing.T) {
	const vaultName = "vault_name"
	const secretName = "sec"

	tests := []struct {
		name     string
		listBody string
		getBody  string
		want     string
		wantErr  bool
	}{
		{
			name: "returns most recently created active version",
			listBody: `{"value":[
				{"id":"https://fake.vault.io/secrets/sec/ver1","attributes":{"enabled":true,"created":1748200000}},
				{"id":"https://fake.vault.io/secrets/sec/ver2","attributes":{"enabled":true,"created":1748300000}}
			]}`,
			getBody: `{"id":"https://fake.vault.io/secrets/sec/ver2","value":"secret_ver2"}`,
			want:    "secret_ver2",
		},
		{
			name: "skips version with future NotBefore",
			listBody: `{"value":[
				{"id":"https://fake.vault.io/secrets/sec/ver1","attributes":{"enabled":true,"created":1748200000}},
				{"id":"https://fake.vault.io/secrets/sec/ver2","attributes":{"enabled":true,"created":1748300000,"nbf":9999999999}}
			]}`,
			getBody: `{"id":"https://fake.vault.io/secrets/sec/ver1","value":"secret_ver1"}`,
			want:    "secret_ver1",
		},
		{
			name: "returns error when all versions have future NotBefore",
			listBody: `{"value":[
				{"id":"https://fake.vault.io/secrets/sec/ver1","attributes":{"enabled":true,"nbf":9999999999}},
				{"id":"https://fake.vault.io/secrets/sec/ver2","attributes":{"enabled":true,"nbf":9999999999}}
			]}`,
			getBody: ``,
			wantErr: true,
		},
		{
			name: "skips disabled version",
			listBody: `{"value":[
				{"id":"https://fake.vault.io/secrets/sec/ver1","attributes":{"enabled":false,"created":1748200000}},
				{"id":"https://fake.vault.io/secrets/sec/ver2","attributes":{"enabled":true,"created":1748300000}}
			]}`,
			getBody: `{"id":"https://fake.vault.io/secrets/sec/ver2","value":"secret_ver2"}`,
			want:    "secret_ver2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.Path, "/versions") {
					return mockResponse(http.StatusOK, tt.listBody), nil
				}
				return mockResponse(http.StatusOK, tt.getBody), nil
			}}
			p := newTestProvider(t, sender)

			got, err := p.Secret(vaultName, secretName, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("Secret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Secret() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSecret_VersionLessLookupCachesTheListing ports krmgen's
// TestGetLatestActiveSecret_CachesNoVersionKey: a second version-less call
// for the same secret must not list versions again. Listing and fetching are
// counted separately - the /versions path identifies a listing request - so
// the assertion can't pass by coincidence if the code fetched the secret
// twice but never cached the listing, or vice versa.
func TestSecret_VersionLessLookupCachesTheListing(t *testing.T) {
	const vaultName = "vault_name"

	listCalls := 0
	sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/versions") {
			listCalls++
			body := `{"value":[{"id":"https://fake.vault.io/secrets/sec/ver1","attributes":{"enabled":true,"created":1748300000}}]}`
			return mockResponse(http.StatusOK, body), nil
		}
		return mockResponse(http.StatusOK, `{"id":"https://fake.vault.io/secrets/sec/ver1","value":"cachedValue"}`), nil
	}}
	p := newTestProvider(t, sender)

	first, err := p.Secret(vaultName, "sec", "")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := p.Secret(vaultName, "sec", "")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if first != "cachedValue" || second != "cachedValue" {
		t.Errorf("expected both calls to return cachedValue, got %q and %q", first, second)
	}
	if listCalls != 1 {
		t.Errorf("expected the versions endpoint called once, but it was called %d times", listCalls)
	}
}

// TestSecret_VersionLessCallAlsoCachesResolvedVersion ports krmgen's test of
// the same name: after a version-less call resolves a version, naming that
// version explicitly must be served from the cache rather than hitting the
// vault again.
func TestSecret_VersionLessCallAlsoCachesResolvedVersion(t *testing.T) {
	const vaultName = "vault_name"

	vaultCalls := 0
	sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
		vaultCalls++
		if strings.Contains(r.URL.Path, "/versions") {
			body := `{"value":[{"id":"https://fake.vault.io/secrets/sec/ver1","attributes":{"enabled":true,"created":1748300000}}]}`
			return mockResponse(http.StatusOK, body), nil
		}
		return mockResponse(http.StatusOK, `{"id":"https://fake.vault.io/secrets/sec/ver1","value":"cachedValue"}`), nil
	}}
	p := newTestProvider(t, sender)

	// Resolves the latest version: one list call plus one get call.
	if _, err := p.Secret(vaultName, "sec", ""); err != nil {
		t.Fatalf("version-less call failed: %v", err)
	}
	callsAfterResolve := vaultCalls

	// Naming the version that was just resolved must not hit the network again.
	got, err := p.Secret(vaultName, "sec", "ver1")
	if err != nil {
		t.Fatalf("explicit-version call failed: %v", err)
	}

	if got != "cachedValue" {
		t.Errorf("Secret() got = %q, want %q", got, "cachedValue")
	}
	if vaultCalls != callsAfterResolve {
		t.Errorf("expected the resolved version to be served from cache, but the vault was called %d more time(s)", vaultCalls-callsAfterResolve)
	}
}

// TestToPem ports krmgen's TestToPemBlock.
func TestToPem(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		blockType string
		want      string
		wantErr   bool
	}{
		{
			name:      "valid",
			text:      string(bytes.Repeat([]byte{13}, 60)),
			blockType: "RSA PRIVATE KEY",
			want:      "-----BEGIN RSA PRIVATE KEY-----\nDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0NDQ0N\nDQ0NDQ0NDQ0NDQ0N\n-----END RSA PRIVATE KEY-----\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToPem(tt.blockType, tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToPem() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToPem() got = %v, want %v", got, tt.want)
			}
		})
	}
}

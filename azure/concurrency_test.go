package azure

import (
	"net/http"
	"strings"
	"sync"
	"testing"
)

// TestProvider_IsSafeForConcurrentUse proves a Provider can be called from
// many goroutines at once. It doesn't assert anything beyond "no error and
// no data race": the guarantee this protects is stated in the README, and
// go test -race is what actually enforces it - a broken lock here doesn't
// fail this test's assertions, it fails the race detector.
//
// Two client kinds are exercised (secrets and certificates) so that a
// regression to a single lock shared across all client kinds - which would
// serialize construction rather than race - is at least covered, even
// though the race detector itself can only catch genuine data races.
func TestProvider_IsSafeForConcurrentUse(t *testing.T) {
	sender := &mockSender{doFunc: func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.Path, "/secrets/"):
			return mockResponse(200, `{"id":"https://fake.vault.io/secrets/sec/v1","value":"v"}`), nil
		case strings.Contains(r.URL.Path, "/certificates/"):
			return mockResponse(200, `{"id":"https://fake.vault.io/certificates/crt/v1","cer":"YmFzZTY0Y2VydA=="}`), nil
		default:
			return mockResponse(200, `{}`), nil
		}
	}}
	p := newTestProvider(t, sender)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := p.Secret("vault", "sec", "v1"); err != nil {
				t.Errorf("concurrent Secret() failed: %v", err)
			}
		}(i)
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := p.CertificateFunc("vault", "crt", "v1"); err != nil {
				t.Errorf("concurrent CertificateFunc() failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

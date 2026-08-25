package azure

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// TestProvider_IsSafeForConcurrentUse proves a Provider can be called from
// many goroutines at once. It doesn't assert anything beyond "no error and
// no data race": the guarantee this protects is stated in the README, and
// go test -race is what actually enforces it - a broken lock here doesn't
// fail this test's assertions, it fails the race detector.
//
// Two client kinds are exercised (secrets and certificates) so a regression
// to a single lock shared across all client kinds is at least reachable by
// this test. Note what this test does NOT prove: the old single-mutex
// design (one sync.Mutex guarding one map, shared by every client kind) was
// already race-free - it serialized unrelated builds instead of racing on
// them - so it would pass this test identically to the fixed code. That
// property is checked separately, by
// TestProvider_UnrelatedClientKindsDoNotSerialize below.
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

// TestProvider_UnrelatedClientKindsDoNotSerialize is the test that actually
// backs the README's claim that building a client of one kind never blocks
// building a client of another kind. TestProvider_IsSafeForConcurrentUse
// above cannot: it only asserts the absence of errors and the absence of
// data races, and the old single-mutex client() was already race-free - it
// just serialized unrelated builds, which is a liveness defect, not a race.
//
// The old design (one sync.Mutex held for the whole build, shared by every
// key) would fail this test: the goroutine building "slow" holds that one
// mutex from the moment it enters client() until its build func returns,
// so the goroutine building "fast" blocks trying to acquire the same
// mutex and never reaches done - the select falls into the timeout branch
// and t.Fatal fires. Against the per-key sync.Once fix, "fast" has its own
// Once with no relationship to "slow"'s, so it completes immediately.
func TestProvider_UnrelatedClientKindsDoNotSerialize(t *testing.T) {
	p := newTestProvider(t, &mockSender{doFunc: func(*http.Request) (*http.Response, error) {
		return mockResponse(200, `{}`), nil
	}})

	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, _ = p.client("slow", func(azcore.TokenCredential) (any, error) {
			close(started)
			<-release
			return "slow-client", nil
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the slow build never started")
	}
	defer close(release)

	done := make(chan struct{})
	go func() {
		_, _ = p.client("fast", func(azcore.TokenCredential) (any, error) {
			return "fast-client", nil
		})
		close(done)
	}()

	select {
	case <-done:
		// Building "fast" completed while "slow" was still blocked on
		// release: the two client kinds did not serialize.
	case <-time.After(2 * time.Second):
		t.Fatal("building a client of a different kind waited on an unrelated client's construction")
	}
}

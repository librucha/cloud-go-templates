package azure

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// fakeCredential stands in for a real Azure credential so no test needs
// credentials or a network.
type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake_token", ExpiresOn: time.Now().Add(time.Hour).UTC()}, nil
}

// mockSender records how many requests were made and answers each one with
// whatever doFunc returns. calls is guarded by a mutex because the
// concurrency test drives one sender from many goroutines at once; every
// other test still only reads calls after all its requests have completed.
type mockSender struct {
	mu     sync.Mutex
	calls  int
	doFunc func(r *http.Request) (*http.Response, error)
}

func (m *mockSender) Do(r *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return m.doFunc(r)
}

func mockResponse(status int, body string) *http.Response {
	h := http.Header{}
	// Key Vault clients perform an authentication challenge before the real
	// request; without this header the SDK rejects the response.
	h.Set("WWW-Authenticate", `Bearer authorization="https://login.windows.net/d5069782-a6df-436e-bac4-67b0c78175c8", resource="not_empty"`)
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// newTestProvider builds a Provider whose every Azure client talks to sender
// instead of the network.
func newTestProvider(t *testing.T, sender *mockSender, extra ...Option) *Provider {
	t.Helper()
	opts := append([]Option{
		WithCredential(fakeCredential{}),
		WithTransport(sender),
	}, extra...)
	p, err := New(context.Background(), opts...)
	if err != nil {
		t.Fatalf("building the test provider: %v", err)
	}
	return p
}

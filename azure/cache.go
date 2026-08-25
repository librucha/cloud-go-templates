package azure

import "sync"

// cache holds every resolved lookup for the lifetime of a Provider. Entries
// are never invalidated: a template render is short-lived, and re-reading a
// secret mid-render would be surprising rather than helpful.
//
// Keys are always built from the function's own arguments, never from the
// resource ID Azure returns. Those two disagree - Azure's IDs carry a
// collection segment such as /secrets/ - and keying on the response made
// every entry unreachable in an earlier version of this code.
type cache struct {
	mu      sync.Mutex
	entries map[string]any
}

func newCache() *cache {
	return &cache{entries: make(map[string]any, 32)}
}

func (c *cache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *cache) put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
}

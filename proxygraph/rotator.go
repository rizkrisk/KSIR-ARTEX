package proxygraph

import (
	"context"
	"sync"
)

// DefaultRotator wraps a Pool and picks the next proxy per call.
type DefaultRotator struct {
	pool   *Pool
	policy RotationPolicy
	mu     sync.Mutex
	last   int // index of the last selected proxy (for future round-robin extensions)
}

// NewRotator returns a DefaultRotator backed by pool with the given default policy.
func NewRotator(pool *Pool, policy RotationPolicy) *DefaultRotator {
	return &DefaultRotator{pool: pool, policy: policy}
}

// Next implements Rotator. The policy argument overrides the rotator's default
// when non-empty; otherwise the default policy set at construction is used.
func (r *DefaultRotator) Next(ctx context.Context, policy RotationPolicy) (Proxy, error) {
	r.mu.Lock()
	p := r.policy
	r.mu.Unlock()
	if policy != "" {
		p = policy
	}
	proxy, err := r.pool.Next(ctx, p)
	if err != nil {
		return Proxy{}, err
	}
	return proxy, nil
}

// ProxyURL returns the URL of the next selected proxy, or "" when the pool is empty.
func (r *DefaultRotator) ProxyURL() string {
	proxy, err := r.pool.Next(context.Background(), r.policy)
	if err != nil {
		return ""
	}
	return proxy.URL
}

// SetPolicy changes the default rotation policy at runtime.
func (r *DefaultRotator) SetPolicy(p RotationPolicy) {
	r.mu.Lock()
	r.policy = p
	r.mu.Unlock()
}

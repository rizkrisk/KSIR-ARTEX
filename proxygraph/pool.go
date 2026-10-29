package proxygraph

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Proxy is the operational view of a single proxy in the rotation pool.
type Proxy struct {
	ID     string
	URL    string
	Type   string // "residential", "tor", "datacenter"
	ASN    string
	Score  float64
	Active bool
}

// RotationPolicy controls how the next proxy is selected from the pool.
type RotationPolicy string

const (
	PolicyRoundRobin RotationPolicy = "round_robin"
	PolicyASNDiverse RotationPolicy = "asn_diverse"
	PolicyLeastUsed  RotationPolicy = "least_used"
)

// Rotator selects the next egress proxy.
type Rotator interface {
	Next(ctx context.Context, policy RotationPolicy) (Proxy, error)
}

// Pool is a thread-safe active proxy pool with health-check support.
type Pool struct {
	mu      sync.Mutex
	proxies []Proxy
	useCount map[string]int
}

// NewPool returns an empty Pool.
func NewPool() *Pool {
	return &Pool{useCount: make(map[string]int)}
}

// Add appends a proxy to the pool (idempotent by ID: replaces if already present).
func (p *Pool) Add(proxy Proxy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, x := range p.proxies {
		if x.ID == proxy.ID {
			p.proxies[i] = proxy
			return
		}
	}
	p.proxies = append(p.proxies, proxy)
}

// Remove deletes a proxy by ID.
func (p *Pool) Remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	keep := p.proxies[:0]
	for _, x := range p.proxies {
		if x.ID != id {
			keep = append(keep, x)
		}
	}
	p.proxies = keep
	delete(p.useCount, id)
}

// Next implements Rotator. Returns an error when the pool is empty or all
// proxies are inactive.
func (p *Pool) Next(ctx context.Context, policy RotationPolicy) (Proxy, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := p.activeSnapshot()
	if len(active) == 0 {
		return Proxy{}, fmt.Errorf("proxygraph: pool empty or no active proxies")
	}

	var chosen Proxy
	switch policy {
	case PolicyASNDiverse:
		chosen = p.pickASNDiverse(active)
	case PolicyLeastUsed:
		chosen = p.pickLeastUsed(active)
	default: // PolicyRoundRobin
		chosen = p.pickRoundRobin(active)
	}

	p.useCount[chosen.ID]++
	return chosen, nil
}

// Snapshot returns a copy of all proxies currently in the pool.
func (p *Pool) Snapshot() []Proxy {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Proxy, len(p.proxies))
	copy(out, p.proxies)
	return out
}

// HealthCheck probes each proxy via a HEAD request to http://example.com.
// Proxies that fail or score ≤ 0 are marked inactive; responsive ones have
// their score updated based on round-trip latency.
func (p *Pool) HealthCheck(ctx context.Context) {
	p.mu.Lock()
	ids := make([]string, len(p.proxies))
	urls := make([]string, len(p.proxies))
	scores := make([]float64, len(p.proxies))
	for i, x := range p.proxies {
		ids[i] = x.ID
		urls[i] = x.URL
		scores[i] = x.Score
	}
	p.mu.Unlock()

	type result struct {
		id     string
		active bool
		score  float64
	}
	results := make([]result, len(ids))

	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := ids[idx]
			proxyURL := urls[idx]
			active, score := probe(ctx, proxyURL)
			results[idx] = result{id: id, active: active, score: score}
		}(i)
	}
	wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, r := range results {
		for i, x := range p.proxies {
			if x.ID == r.id {
				p.proxies[i].Active = r.active
				p.proxies[i].Score = r.score
				break
			}
		}
	}
	// Purge proxies with score ≤ 0 from the active set (already marked inactive above).
}

// probe sends a HEAD request through proxyURL and returns (active, latency score).
// score = 1/(latency_seconds); score 0 = unreachable.
func probe(ctx context.Context, proxyURL string) (bool, float64) {
	transport, err := proxyTransport(proxyURL)
	if err != nil {
		return false, 0
	}
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "http://example.com", nil)
	if err != nil {
		return false, 0
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return false, 0
	}
	resp.Body.Close()
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	return true, 1.0 / elapsed
}

// activeSnapshot returns the subset of proxies marked Active. Caller holds p.mu.
func (p *Pool) activeSnapshot() []Proxy {
	out := make([]Proxy, 0, len(p.proxies))
	for _, x := range p.proxies {
		if x.Active {
			out = append(out, x)
		}
	}
	return out
}

// pickRoundRobin chooses the proxy with the lowest use count (stable round-robin).
// Caller holds p.mu.
func (p *Pool) pickRoundRobin(active []Proxy) Proxy {
	best := active[0]
	min := p.useCount[best.ID]
	for _, x := range active[1:] {
		if c := p.useCount[x.ID]; c < min {
			best = x
			min = c
		}
	}
	return best
}

// pickASNDiverse prefers a proxy whose ASN differs from the most-recently-used one.
// Caller holds p.mu.
func (p *Pool) pickASNDiverse(active []Proxy) Proxy {
	// Determine the ASN that was most recently used (highest use count).
	lastASN := ""
	maxUse := -1
	for _, x := range active {
		if c := p.useCount[x.ID]; c > maxUse {
			maxUse = c
			lastASN = x.ASN
		}
	}
	// Prefer a different ASN; fall back to round-robin if none available.
	for _, x := range active {
		if x.ASN != lastASN {
			return x
		}
	}
	return p.pickRoundRobin(active)
}

// pickLeastUsed selects the proxy with the fewest total uses.
// Caller holds p.mu.
func (p *Pool) pickLeastUsed(active []Proxy) Proxy {
	best := active[0]
	min := p.useCount[best.ID]
	for _, x := range active[1:] {
		if c := p.useCount[x.ID]; c < min {
			best = x
			min = c
		}
	}
	return best
}

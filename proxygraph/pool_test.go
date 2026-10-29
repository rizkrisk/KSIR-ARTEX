package proxygraph

import (
	"context"
	"testing"
)

// TestPoolHealthCheck verifies that a proxy with Score=0 is removed from the
// active pool after HealthCheck runs. We exercise this without real network
// by pre-populating a proxy whose URL is deliberately unreachable.
func TestPoolHealthCheck(t *testing.T) {
	p := NewPool()
	p.Add(Proxy{
		ID:     "bad-proxy",
		URL:    "http://127.0.0.2:19999", // unreachable → probe returns score=0
		Type:   "datacenter",
		Active: true,
		Score:  1.0,
	})

	p.HealthCheck(context.Background())

	snap := p.Snapshot()
	for _, x := range snap {
		if x.ID == "bad-proxy" && x.Active {
			t.Fatalf("expected bad-proxy to be inactive after HealthCheck, got Active=true Score=%f", x.Score)
		}
	}
}

// TestRotatorASNDiverse verifies that the ASN-diverse policy selects proxies
// from different ASNs on consecutive calls.
func TestRotatorASNDiverse(t *testing.T) {
	pool := NewPool()
	pool.Add(Proxy{ID: "p1", URL: "http://10.0.0.1:8080", Type: "datacenter", ASN: "AS1234", Active: true, Score: 1.0})
	pool.Add(Proxy{ID: "p2", URL: "http://10.0.0.2:8080", Type: "datacenter", ASN: "AS5678", Active: true, Score: 1.0})
	pool.Add(Proxy{ID: "p3", URL: "http://10.0.0.3:8080", Type: "datacenter", ASN: "AS1234", Active: true, Score: 1.0})

	r := NewRotator(pool, PolicyASNDiverse)
	ctx := context.Background()

	first, err := r.Next(ctx, PolicyASNDiverse)
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	second, err := r.Next(ctx, PolicyASNDiverse)
	if err != nil {
		t.Fatalf("second Next: %v", err)
	}

	if first.ASN == second.ASN && len(pool.Snapshot()) > 1 {
		// Only fail when there really are multiple ASNs available.
		hasOtherASN := false
		for _, x := range pool.Snapshot() {
			if x.ASN != first.ASN {
				hasOtherASN = true
				break
			}
		}
		if hasOtherASN {
			t.Fatalf("asn_diverse: consecutive picks have same ASN=%q; first=%s second=%s",
				first.ASN, first.ID, second.ID)
		}
	}
}

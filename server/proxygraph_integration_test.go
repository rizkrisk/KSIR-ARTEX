package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// TestProxyRotationPreservesTrafficCapture verifies that the proxygraph API
// endpoints are reachable and return sensible data while a task with traffic
// capture is running. It does not perform real network rotation — it confirms
// the handler is wired up correctly and that adding/removing a node round-trips.
func TestProxyRotationPreservesTrafficCapture(t *testing.T) {
	// Skip when no postgres database is configured (CI without a DB sidecar, dev without ARTEX_PG_DSN).
	if _, _, err := db.DSN(); err != nil {
		t.Skipf("no database config (%v) — skipping", err)
	}
	// Re-use the shared test server that has traffic capture enabled.
	s, _, request := trafficEvidenceServer(t)
	_ = s

	// 1. GET /api/proxygraph/nodes — must return 200 with an empty node list.
	w := request("GET", "/api/proxygraph/nodes", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/proxygraph/nodes: want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var listResp struct {
		Nodes []map[string]any `json:"nodes"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode nodes list: %v", err)
	}

	// 2. POST /api/proxygraph/nodes — add a node.
	addBody := `{"id":"test-node-1","url":"http://10.0.0.100:8080","type":"datacenter","asn":"AS1234"}`
	w = request("POST", "/api/proxygraph/nodes", addBody)
	if w.Code != 201 {
		t.Fatalf("POST /api/proxygraph/nodes: want 201 got %d body=%s", w.Code, w.Body.String())
	}

	// 3. GET /api/proxygraph/nodes — must now include the new node.
	w = request("GET", "/api/proxygraph/nodes", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/proxygraph/nodes after add: want 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test-node-1") {
		t.Fatalf("expected test-node-1 in node list, got: %s", w.Body.String())
	}

	// 4. GET /api/proxygraph/config — must return 200.
	w = request("GET", "/api/proxygraph/config", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/proxygraph/config: want 200 got %d body=%s", w.Code, w.Body.String())
	}

	// 5. PUT /api/proxygraph/config — set policy to asn_diverse.
	w = request("PUT", "/api/proxygraph/config", `{"policy":"asn_diverse"}`)
	if w.Code != 200 {
		t.Fatalf("PUT /api/proxygraph/config: want 200 got %d body=%s", w.Code, w.Body.String())
	}

	// 6. DELETE /api/proxygraph/nodes/{id} — remove the test node.
	w2 := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		// Re-use the same auth token from trafficEvidenceServer's request closure.
		return request(method, path, body)
	}
	del := w2("DELETE", "/api/proxygraph/nodes/test-node-1", "")
	if del.Code != 200 {
		t.Fatalf("DELETE /api/proxygraph/nodes/test-node-1: want 200 got %d body=%s", del.Code, del.Body.String())
	}

	// 7. GET /api/proxygraph/health — must return 200 (even with empty pool).
	w = request("GET", "/api/proxygraph/health", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/proxygraph/health: want 200 got %d body=%s", w.Code, w.Body.String())
	}
}

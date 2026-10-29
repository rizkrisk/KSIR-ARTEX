package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Autumn-27/artex/proxygraph"
)

// proxyGraphState is the process-wide proxy graph singleton, managed by the API.
var proxyGraphState = struct {
	graph  *proxygraph.Graph
	pool   *proxygraph.Pool
	rotator *proxygraph.DefaultRotator
	policy proxygraph.RotationPolicy
}{
	graph:  proxygraph.NewGraph(),
	pool:   proxygraph.NewPool(),
	policy: proxygraph.PolicyRoundRobin,
}

func init() {
	proxyGraphState.rotator = proxygraph.NewRotator(proxyGraphState.pool, proxygraph.PolicyRoundRobin)
}

// --- DTOs ---

type proxyNodeDTO struct {
	ID     string  `json:"id"`
	URL    string  `json:"url"`
	Type   string  `json:"type"`
	ASN    string  `json:"asn"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
}

func nodeToDTO(n proxygraph.Node) proxyNodeDTO {
	return proxyNodeDTO{ID: n.ID, URL: n.URL, Type: n.Type, ASN: n.ASN, Active: n.Active, Score: n.Score}
}

// --- Handlers ---

// GET /api/proxygraph/nodes — list all proxy nodes.
func (s *Server) proxyGraphListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := proxyGraphState.graph.AllNodes()
	dtos := make([]proxyNodeDTO, len(nodes))
	for i, n := range nodes {
		dtos[i] = nodeToDTO(n)
	}
	writeJSON(w, 200, map[string]any{"nodes": dtos})
}

// POST /api/proxygraph/nodes — add a proxy node.
func (s *Server) proxyGraphAddNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		Type string `json:"type"`
		ASN  string `json:"asn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeErr(w, 400, "url required")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeErr(w, 400, "id required")
		return
	}
	nodeType := req.Type
	if nodeType == "" {
		nodeType = "datacenter"
	}
	n := proxygraph.Node{ID: req.ID, URL: req.URL, Type: nodeType, ASN: req.ASN, Active: true, Score: 1.0}
	proxyGraphState.graph.AddNode(n)
	proxyGraphState.pool.Add(proxygraph.Proxy{ID: n.ID, URL: n.URL, Type: n.Type, ASN: n.ASN, Score: n.Score, Active: true})
	writeJSON(w, 201, nodeToDTO(n))
}

// DELETE /api/proxygraph/nodes/{id} — remove a proxy node.
func (s *Server) proxyGraphRemoveNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, 400, "id required")
		return
	}
	proxyGraphState.graph.RemoveNode(id)
	proxyGraphState.pool.Remove(id)
	writeJSON(w, 200, map[string]any{"removed": id})
}

// GET /api/proxygraph/health — run health check and return updated node list.
func (s *Server) proxyGraphHealth(w http.ResponseWriter, r *http.Request) {
	proxyGraphState.pool.HealthCheck(context.Background())
	// Sync graph active/score from pool snapshot.
	for _, p := range proxyGraphState.pool.Snapshot() {
		proxyGraphState.graph.AddNode(proxygraph.Node{
			ID: p.ID, URL: p.URL, Type: p.Type, ASN: p.ASN,
			Active: p.Active, Score: p.Score,
		})
	}
	nodes := proxyGraphState.graph.AllNodes()
	dtos := make([]proxyNodeDTO, len(nodes))
	for i, n := range nodes {
		dtos[i] = nodeToDTO(n)
	}
	writeJSON(w, 200, map[string]any{"nodes": dtos})
}

// GET /api/proxygraph/config — get current rotation policy.
func (s *Server) proxyGraphGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"policy": string(proxyGraphState.policy)})
}

// PUT /api/proxygraph/config — set rotation policy.
func (s *Server) proxyGraphSetConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Policy string `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	p := proxygraph.RotationPolicy(req.Policy)
	switch p {
	case proxygraph.PolicyRoundRobin, proxygraph.PolicyASNDiverse, proxygraph.PolicyLeastUsed:
	default:
		writeErr(w, 400, "policy must be round_robin|asn_diverse|least_used")
		return
	}
	proxyGraphState.policy = p
	proxyGraphState.rotator.SetPolicy(p)
	writeJSON(w, 200, map[string]any{"policy": string(p)})
}

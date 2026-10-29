package proxygraph

import "sync"

// Node is a proxy entry in the graph.
type Node struct {
	ID     string
	URL    string
	Type   string  // "residential", "tor", "datacenter"
	ASN    string
	Active bool
	Score  float64
}

// Edge connects two proxy nodes with a measured latency.
type Edge struct {
	From    string
	To      string
	Latency float64
}

// Graph holds the full proxy topology: nodes keyed by ID and directed edges.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*Node
	edges []Edge
}

// NewGraph returns an empty Graph.
func NewGraph() *Graph {
	return &Graph{nodes: make(map[string]*Node)}
}

// AddNode inserts or replaces a node.
func (g *Graph) AddNode(n Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[n.ID] = &n
}

// RemoveNode deletes a node and all edges that reference it.
func (g *Graph) RemoveNode(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.nodes, id)
	keep := g.edges[:0]
	for _, e := range g.edges {
		if e.From != id && e.To != id {
			keep = append(keep, e)
		}
	}
	g.edges = keep
}

// AddEdge inserts a directed latency edge between two nodes.
func (g *Graph) AddEdge(e Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.edges = append(g.edges, e)
}

// ActiveNodes returns a snapshot of all nodes with Active=true.
func (g *Graph) ActiveNodes() []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		if n.Active {
			out = append(out, *n)
		}
	}
	return out
}

// AllNodes returns a snapshot of every node regardless of active state.
func (g *Graph) AllNodes() []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, *n)
	}
	return out
}

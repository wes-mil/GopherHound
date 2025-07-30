package main

import "golang.org/x/mod/semver"

type Graph struct {
	Nodes map[string]Node
	Edges []Edge
}

type Edge struct {
	From string
	To   string
}

type Node struct {
	Module           string
	Version          string
	UnpickedVersions map[string]struct{}
}

func NewGraph() Graph {
	return Graph{
		Nodes: make(map[string]Node),
		Edges: make([]Edge, 0),
	}
}

func (g *Graph) AddNode(module string, version string) {
	if node, exists := g.Nodes[module]; exists {
		if v := semver.Compare(node.Version, version); v < 0 {
			node.UnpickedVersions[node.Version] = struct{}{}
			node.Version = version
			g.Nodes[module] = node
		} else if v > 0 {
			node.UnpickedVersions[version] = struct{}{}
			g.Nodes[module] = node
		}
	} else {
		g.Nodes[module] = Node{
			Module:           module,
			Version:          version,
			UnpickedVersions: map[string]struct{}{},
		}
	}
}

func (g *Graph) AddEdge(from string, to string) {
	g.Edges = append(g.Edges, Edge{From: from, To: to})
}

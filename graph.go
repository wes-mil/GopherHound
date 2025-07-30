package main

type Graph struct {
	Edges []Edge
}

func (g *Graph) AddEdge(edge Edge) {
	g.Edges = append(g.Edges, edge)
}

type Edge struct {
	From string
	To   string
}

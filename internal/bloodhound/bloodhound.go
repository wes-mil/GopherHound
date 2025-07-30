package bloodhound

type OpenGraph struct {
	Metadata Metadata `json:"metadata"`
	Graph    Graph    `json:"graph"`
}

type Metadata struct {
	SourceKind string `json:"source_kind"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID         string         `json:"id"`
	Kinds      []string       `json:"kinds"`
	Properties map[string]any `json:"properties"`
}

type Edge struct {
	Start NodeIdentifier `json:"start"`
	End   NodeIdentifier `json:"end"`
}

type NodeIdentifier struct {
	MatchBy string `json:"match_by"`
	Value   string `json:"value"`
	Kind    string `json:"kind"`
}

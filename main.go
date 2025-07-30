package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/wes-mil/GopherHound/internal/bloodhound"
	"golang.org/x/mod/modfile"
)

func main() {
	cmd := exec.Command("go", "mod", "graph")

	outpipe, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("Failed to create stdout pipe", "command", "go mod graph", "error", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		slog.Error("Failed to start command", "command", "go mod graph", "error", err)
		os.Exit(1)
	}

	rootNode, err := getRootNode("go.mod")
	if err != nil {
		slog.Error("Failed to get root node", "error", err)
		os.Exit(1)
	}

	graph, err := processOutput(outpipe, rootNode)
	if err != nil {
		slog.Error("Failed to process output", "error", err)
		os.Exit(1)
	}

	if err := outpipe.Close(); err != nil {
		slog.Error("Failed to close outpipe", "error", err)
		os.Exit(1)
	}

	bhGraph := bloodhound.OpenGraph{
		Metadata: bloodhound.Metadata{
			SourceKind: "GopherHound",
		},
		Graph: bloodhound.Graph{
			Nodes: []bloodhound.Node{},
			Edges: []bloodhound.Edge{},
		},
	}

	for _, e := range graph.Edges {
		bhGraph.Graph.Edges = append(bhGraph.Graph.Edges, bloodhound.Edge{
			Start: bloodhound.NodeIdentifier{
				MatchBy: "id",
				Value:   e.From,
			},
			End: bloodhound.NodeIdentifier{
				MatchBy: "id",
				Value:   e.To,
			},
			Kind: "DependsOn",
		})
	}

	file, err := os.Create("opengraph.json")
	if err != nil {
		slog.Error("Failed to open output file", "filename", "opengraph.json", "error", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(bhGraph); err != nil {
		slog.Error("Failed to write to output file", "filename", "opengraph.json", "error", err)
		os.Exit(1)
	}
}

func getRootNode(modPath string) (result string, err error) {
	goModFile, err := os.ReadFile(modPath)
	if err != nil {
		return result, fmt.Errorf("could not read go.mod file: %v", err)
	}

	modFile, err := modfile.Parse("go.mod", goModFile, nil)
	if err != nil {
		return result, fmt.Errorf("could not parse go.mod file: %v", err)
	}

	if modFile.Module == nil {
		return result, fmt.Errorf("go mod is not expected format. module not found")
	}
	return modFile.Module.Mod.Path, nil
}

func processOutput(r io.ReadCloser, root string) (Graph, error) {
	var (
		scanner = bufio.NewScanner(r)
		graph   = Graph{}
	)

	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return graph, fmt.Errorf("expected 2 words in line, but got %d: %s", len(parts), line)
		}

		from, to := parts[0], parts[1]
		if (to != root && !strings.Contains(to, "@")) || (from != root && !strings.Contains(from, "@")) {
			continue
		}

		graph.AddEdge(Edge{From: from, To: to})
	}

	if err := scanner.Err(); err != nil {
		return graph, err
	}

	return graph, nil
}

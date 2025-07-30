package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"slices"
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
			SourceKind: "GopherHoundBase",
		},
		Graph: bloodhound.Graph{
			Nodes: []bloodhound.Node{},
			Edges: []bloodhound.Edge{},
		},
	}

	for _, n := range graph.Nodes {
		newNode := bloodhound.Node{
			ID:    n.Module,
			Kinds: []string{"GoModule"},
			Properties: map[string]any{
				"Version": n.Version,
			},
		}

		unpickedVersions := slices.SortedFunc(maps.Keys(n.UnpickedVersions), func(s1 string, s2 string) int {
			return strings.Compare(s2, s1)
		})

		if len(unpickedVersions) > 0 {
			newNode.Properties["UnpickedVersions"] = unpickedVersions
		}

		bhGraph.Graph.Nodes = append(bhGraph.Graph.Nodes, newNode)
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
		graph   = NewGraph()
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

		var fromModule, fromVersion string
		if parts := strings.Split(from, "@"); len(parts) > 1 {
			fromModule, fromVersion = parts[0], parts[1]
		} else if len(parts) == 1 {
			fromModule = parts[0]
		}

		if fromModule != root && fromVersion == "" {
			continue
		}

		var toModule, toVersion string
		if parts := strings.Split(to, "@"); len(parts) > 1 {
			toModule, toVersion = parts[0], parts[1]
		} else if len(parts) == 1 {
			fromModule = parts[0]
		}

		if toModule != root && toVersion == "" {
			continue
		}

		graph.AddNode(fromModule, fromVersion)
		graph.AddNode(toModule, toVersion)
		graph.AddEdge(fromModule, toModule)
	}

	if err := scanner.Err(); err != nil {
		return graph, err
	}

	return graph, nil
}

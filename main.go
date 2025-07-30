package main

import (
	"bufio"
	"encoding/json"
	"flag"
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
	var outputFile string
	flag.StringVar(&outputFile, "output", "opengraph.json", "output file")

	flag.Parse()

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
			ID:         builderGopherHoundID(rootNode, n.Module),
			Kinds:      []string{"GoModule"},
			Properties: map[string]any{},
		}

		newNode.Properties["name"] = n.Module
		newNode.Properties["module"] = n.Module

		if n.Version != "" {
			newNode.Properties["version"] = n.Version
		}

		unpickedVersions := slices.SortedFunc(maps.Keys(n.UnpickedVersions), func(s1 string, s2 string) int {
			return strings.Compare(s2, s1)
		})

		if len(unpickedVersions) > 0 {
			newNode.Properties["unpicked_versions"] = unpickedVersions
		}

		bhGraph.Graph.Nodes = append(bhGraph.Graph.Nodes, newNode)
	}

	for _, e := range graph.Edges {
		bhGraph.Graph.Edges = append(bhGraph.Graph.Edges, bloodhound.Edge{
			Start: bloodhound.NodeIdentifier{
				MatchBy: "id",
				Value:   builderGopherHoundID(rootNode, e.From),
			},
			End: bloodhound.NodeIdentifier{
				MatchBy: "id",
				Value:   builderGopherHoundID(rootNode, e.To),
			},
			Kind: "RequiredBy",
		})
	}

	file, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		slog.Error("Failed to open output file", "filename", "opengraph.json", "error", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(bhGraph); err != nil {
		slog.Error("Failed to write to output file", "filename", "opengraph.json", "error", err)
		os.Exit(1)
	}

	if err := file.Close(); err != nil {
		slog.Error("Failed to write to close output file", "filename", "opengraph.json", "error", err)
	}
}

func builderGopherHoundID(rootNode string, module string) string {
	return fmt.Sprintf("%s_%s_%s", "gopherhound", rootNode, module)
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

		parent, child := parts[0], parts[1]
		if (child != root && !strings.Contains(child, "@")) || (parent != root && !strings.Contains(parent, "@")) {
			continue
		}

		var parentModule, parentVersion string
		if parts := strings.Split(parent, "@"); len(parts) > 1 {
			parentModule, parentVersion = parts[0], parts[1]
		} else if len(parts) == 1 {
			parentModule = parts[0]
		}

		if parentModule != root && parentVersion == "" {
			continue
		}

		var childModule, childVersion string
		if parts := strings.Split(child, "@"); len(parts) > 1 {
			childModule, childVersion = parts[0], parts[1]
		} else if len(parts) == 1 {
			childModule = parts[0]
		}

		if childModule != root && childVersion == "" {
			continue
		}

		graph.AddNode(parentModule, parentVersion)
		graph.AddNode(childModule, childVersion)
		graph.AddEdge(childModule, parentModule)
	}

	if err := scanner.Err(); err != nil {
		return graph, err
	}

	return graph, nil
}

package export

import (
	"database/sql"
	"fmt"
)

// Graph is the export structure for atlas export graph.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode represents a symbol node in the graph.
type GraphNode struct {
	StableID string `json:"stable_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Language string `json:"language"`
}

// GraphEdge represents a reference edge in the graph.
type GraphEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Confidence string `json:"confidence"`
}

// ExportGraph produces a Graph from the database.
func ExportGraph(database *sql.DB) (*Graph, error) {
	g := &Graph{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Nodes: all symbols
	rows, err := database.Query(`SELECT s.stable_id, s.name, s.symbol_kind, f.path, f.language
		FROM symbols s JOIN files f ON s.file_id = f.id ORDER BY s.stable_id`)
	if err != nil {
		return nil, fmt.Errorf("querying nodes: %w", err)
	}
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.StableID, &n.Name, &n.Kind, &n.File, &n.Language); err != nil {
			_ = rows.Close()
			return nil, err
		}
		g.Nodes = append(g.Nodes, n)
	}
	_ = rows.Close()

	// Edges: all resolved references with symbol stable_ids
	rows, err = database.Query(`SELECT COALESCE(sf.stable_id,''), COALESCE(st.stable_id,''), r.reference_kind, r.confidence
		FROM "references" r
		LEFT JOIN symbols sf ON r.from_symbol_id = sf.id
		LEFT JOIN symbols st ON r.to_symbol_id = st.id
		WHERE r.from_symbol_id IS NOT NULL OR r.to_symbol_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("querying edges: %w", err)
	}
	for rows.Next() {
		var e GraphEdge
		if err := rows.Scan(&e.From, &e.To, &e.Kind, &e.Confidence); err != nil {
			_ = rows.Close()
			return nil, err
		}
		g.Edges = append(g.Edges, e)
	}
	_ = rows.Close()

	return g, nil
}

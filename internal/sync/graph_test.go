package sync

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func setupTestGraph(t *testing.T) (*Graph, *storage.Storage, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	ctx := context.Background()
	st, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	scopeCfg := &config.SyncScope{
		Mode:  "following",
		Depth: 2,
	}

	graph := NewGraph(st, scopeCfg)

	cleanup := func() {
		st.Close()
	}

	return graph, st, cleanup
}

func TestNewGraph(t *testing.T) {
	graph, _, cleanup := setupTestGraph(t)
	defer cleanup()

	if graph == nil {
		t.Fatal("Expected graph, got nil")
	}
}

func TestProcessContactList(t *testing.T) {
	graph, _, cleanup := setupTestGraph(t)
	defer cleanup()

	ctx := context.Background()
	rootPubkey := "root-pubkey"

	event := &nostr.Event{
		Kind:      3,
		PubKey:    rootPubkey,
		CreatedAt: 12345,
		Tags: nostr.Tags{
			{"p", "follow1"},
			{"p", "follow2"},
			{"p", "follow3"},
		},
	}

	if err := graph.ProcessContactList(ctx, event, rootPubkey); err != nil {
		t.Fatalf("ProcessContactList() error = %v", err)
	}

	// Verify graph nodes were created
	nodes, err := graph.storage.GetGraphNodes(ctx, rootPubkey, 1)
	if err != nil {
		t.Fatalf("GetGraphNodes() error = %v", err)
	}

	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// All should be depth 1
	for _, node := range nodes {
		if node.Depth != 1 {
			t.Errorf("Expected depth 1, got %d", node.Depth)
		}
	}
}

func TestGetAuthorsInScope_Self(t *testing.T) {
	graph, _, cleanup := setupTestGraph(t)
	defer cleanup()

	graph.config.Mode = "self"
	ctx := context.Background()
	rootPubkey := "root-pubkey"

	authors, err := graph.GetAuthorsInScope(ctx, rootPubkey)
	if err != nil {
		t.Fatalf("GetAuthorsInScope() error = %v", err)
	}

	if len(authors) != 1 {
		t.Errorf("Expected 1 author, got %d", len(authors))
	}

	if authors[0] != rootPubkey {
		t.Errorf("Expected root pubkey, got %s", authors[0])
	}
}

func TestGetAuthorsInScope_Following(t *testing.T) {
	graph, st, cleanup := setupTestGraph(t)
	defer cleanup()

	graph.config.Mode = "following"
	ctx := context.Background()
	rootPubkey := "root-pubkey"

	// Add some follows
	node1 := &storage.GraphNode{
		RootPubkey: rootPubkey,
		Pubkey:     "follow1",
		Depth:      1,
		Mutual:     false,
		LastSeen:   12345,
	}

	node2 := &storage.GraphNode{
		RootPubkey: rootPubkey,
		Pubkey:     "follow2",
		Depth:      1,
		Mutual:     false,
		LastSeen:   12345,
	}

	st.SaveGraphNode(ctx, node1)
	st.SaveGraphNode(ctx, node2)

	authors, err := graph.GetAuthorsInScope(ctx, rootPubkey)
	if err != nil {
		t.Fatalf("GetAuthorsInScope() error = %v", err)
	}

	// Should include root + 2 follows
	if len(authors) != 3 {
		t.Errorf("Expected 3 authors, got %d", len(authors))
	}
}

func TestGetAuthorsInScope_Mutual(t *testing.T) {
	graph, st, cleanup := setupTestGraph(t)
	defer cleanup()

	graph.config.Mode = "mutual"
	ctx := context.Background()
	rootPubkey := "root-pubkey"

	// Add mutual and non-mutual follows
	mutual := &storage.GraphNode{
		RootPubkey: rootPubkey,
		Pubkey:     "mutual1",
		Depth:      1,
		Mutual:     true,
		LastSeen:   12345,
	}

	nonMutual := &storage.GraphNode{
		RootPubkey: rootPubkey,
		Pubkey:     "nonmutual1",
		Depth:      1,
		Mutual:     false,
		LastSeen:   12345,
	}

	st.SaveGraphNode(ctx, mutual)
	st.SaveGraphNode(ctx, nonMutual)

	authors, err := graph.GetAuthorsInScope(ctx, rootPubkey)
	if err != nil {
		t.Fatalf("GetAuthorsInScope() error = %v", err)
	}

	// Should include root + 1 mutual
	if len(authors) != 2 {
		t.Errorf("Expected 2 authors (root + 1 mutual), got %d", len(authors))
	}
}

func TestApplyLimits(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.SyncScope
		authors  []string
		expected int
	}{
		{
			name: "with denylist",
			config: &config.SyncScope{
				DenylistPubkeys: []string{"denied"},
			},
			authors:  []string{"allowed", "denied", "another"},
			expected: 2,
		},
		{
			name: "with allowlist",
			config: &config.SyncScope{
				AllowlistPubkeys: []string{"allowed"},
			},
			authors:  []string{"allowed", "other", "another"},
			expected: 1,
		},
		{
			name: "with max authors",
			config: &config.SyncScope{
				MaxAuthors: 2,
			},
			authors:  []string{"author1", "author2", "author3", "author4"},
			expected: 2,
		},
		{
			name:     "no limits",
			config:   &config.SyncScope{},
			authors:  []string{"author1", "author2", "author3"},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, "test.db")

			cfg := &config.Storage{
				Driver:     "sqlite",
				SQLitePath: dbPath,
			}

			ctx := context.Background()
			st, _ := storage.New(ctx, cfg)
			defer st.Close()

			graph := NewGraph(st, tt.config)
			filtered := graph.applyLimits(tt.authors)

			if len(filtered) != tt.expected {
				t.Errorf("Expected %d authors, got %d", tt.expected, len(filtered))
			}
		})
	}
}

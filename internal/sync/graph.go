package sync

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Graph handles social graph computation from kind 3 events
type Graph struct {
	storage *storage.Storage
	config  *config.SyncScope
}

// NewGraph creates a new graph processor
func NewGraph(st *storage.Storage, cfg *config.SyncScope) *Graph {
	return &Graph{
		storage: st,
		config:  cfg,
	}
}

// ProcessContactList processes a kind 3 event (contact list) and updates the social graph
func (g *Graph) ProcessContactList(ctx context.Context, event *nostr.Event, rootPubkey string) error {
	if event.Kind != 3 {
		return fmt.Errorf("expected kind 3, got %d", event.Kind)
	}

	// Extract followed pubkeys from p tags
	following := make([]string, 0)
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			following = append(following, tag[1])
		}
	}

	// Determine depth for these contacts
	depth := 1
	if event.PubKey == rootPubkey {
		depth = 1 // Direct follows
	} else {
		// This is a follow list from someone in the graph
		// We need to check their current depth
		nodes, err := g.storage.GetGraphNodes(ctx, rootPubkey, 999)
		if err != nil {
			return fmt.Errorf("failed to get graph nodes: %w", err)
		}

		// Find the depth of the event author
		for _, node := range nodes {
			if node.Pubkey == event.PubKey {
				depth = node.Depth + 1
				break
			}
		}
	}

	// Save each followed pubkey as a graph node
	for _, followedPubkey := range following {
		node := &storage.GraphNode{
			RootPubkey: rootPubkey,
			Pubkey:     followedPubkey,
			Depth:      depth,
			Mutual:     false, // We'll calculate this separately
			LastSeen:   int64(event.CreatedAt),
		}

		if err := g.storage.SaveGraphNode(ctx, node); err != nil {
			return fmt.Errorf("failed to save graph node: %w", err)
		}
	}

	return nil
}

// ComputeMutuals identifies mutual follows and updates the graph
func (g *Graph) ComputeMutuals(ctx context.Context, rootPubkey string) error {
	// Get all direct follows (depth 1)
	allNodes, err := g.storage.GetGraphNodes(ctx, rootPubkey, 1)
	if err != nil {
		return fmt.Errorf("failed to get graph nodes: %w", err)
	}

	directFollows := make(map[string]bool)
	for _, node := range allNodes {
		if node.Depth == 1 {
			directFollows[node.Pubkey] = true
		}
	}

	// For each direct follow, check if they follow us back
	for followedPubkey := range directFollows {
		// Get their contact list from storage
		filter := nostr.Filter{
			Kinds:   []int{3},
			Authors: []string{followedPubkey},
			Limit:   1,
		}

		events, err := g.storage.QueryEvents(ctx, filter)
		if err != nil {
			continue // Skip if we can't query
		}

		if len(events) == 0 {
			continue // No contact list yet
		}

		// Check if they follow root back
		isMutual := false
		for _, tag := range events[0].Tags {
			if len(tag) >= 2 && tag[0] == "p" && tag[1] == rootPubkey {
				isMutual = true
				break
			}
		}

		// Update the node if it's mutual
		if isMutual {
			// Fetch and update the node
			nodes, err := g.storage.GetGraphNodes(ctx, rootPubkey, 1)
			if err != nil {
				continue
			}

			for _, node := range nodes {
				if node.Pubkey == followedPubkey {
					node.Mutual = true
					if err := g.storage.SaveGraphNode(ctx, node); err != nil {
						return fmt.Errorf("failed to update mutual status: %w", err)
					}
					break
				}
			}
		}
	}

	return nil
}

// GetAuthorsInScope returns the list of authors to sync based on scope configuration
func (g *Graph) GetAuthorsInScope(ctx context.Context, rootPubkey string) ([]string, error) {
	switch g.config.Mode {
	case "self":
		return []string{rootPubkey}, nil

	case "following":
		following, err := g.storage.GetFollowingPubkeys(ctx, rootPubkey)
		if err != nil {
			return nil, err
		}
		authors := []string{rootPubkey}
		authors = append(authors, following...)
		return g.applyLimits(authors), nil

	case "mutual":
		mutuals, err := g.storage.GetMutualPubkeys(ctx, rootPubkey)
		if err != nil {
			return nil, err
		}
		authors := []string{rootPubkey}
		authors = append(authors, mutuals...)
		return g.applyLimits(authors), nil

	case "foaf":
		// Get all nodes up to configured depth
		maxDepth := g.config.Depth
		if maxDepth == 0 {
			maxDepth = 2 // Default FOAF depth
		}

		nodes, err := g.storage.GetGraphNodes(ctx, rootPubkey, maxDepth)
		if err != nil {
			return nil, err
		}

		// Extract unique pubkeys
		authorSet := make(map[string]bool)
		authorSet[rootPubkey] = true
		for _, node := range nodes {
			authorSet[node.Pubkey] = true
		}

		authors := make([]string, 0, len(authorSet))
		for pubkey := range authorSet {
			authors = append(authors, pubkey)
		}

		return g.applyLimits(authors), nil

	default:
		return []string{rootPubkey}, nil
	}
}

// applyLimits applies allowlist, denylist, and max_authors limits
func (g *Graph) applyLimits(authors []string) []string {
	filtered := make([]string, 0, len(authors))

	for _, author := range authors {
		// Check denylist
		denied := false
		for _, denied_pk := range g.config.DenylistPubkeys {
			if author == denied_pk {
				denied = true
				break
			}
		}
		if denied {
			continue
		}

		// Check allowlist if configured
		if len(g.config.AllowlistPubkeys) > 0 {
			allowed := false
			for _, allowed_pk := range g.config.AllowlistPubkeys {
				if author == allowed_pk {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		filtered = append(filtered, author)
	}

	// Apply max authors cap
	if g.config.MaxAuthors > 0 && len(filtered) > g.config.MaxAuthors {
		filtered = filtered[:g.config.MaxAuthors]
	}

	return filtered
}

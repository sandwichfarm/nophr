package benchmark

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// BenchmarkStorageInsert benchmarks event insertion
func BenchmarkStorageInsert(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	ctx := context.Background()
	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := &nostr.Event{
			ID:        fmt.Sprintf("event%060d", i),
			PubKey:    "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab",
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Kind:      1,
			Content:   "Benchmark event content",
			Tags:      nostr.Tags{},
			Sig:       "sig0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}

		if err := st.StoreEvent(ctx, event); err != nil {
			b.Fatalf("Failed to store event: %v", err)
		}
	}
}

// BenchmarkStorageQuery benchmarks event querying
func BenchmarkStorageQuery(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	ctx := context.Background()
	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Pre-populate with events
	pubkey := "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab"
	for i := 0; i < 1000; i++ {
		event := &nostr.Event{
			ID:        fmt.Sprintf("event%060d", i),
			PubKey:    pubkey,
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Kind:      1,
			Content:   "Benchmark event",
			Tags:      nostr.Tags{},
			Sig:       "sig0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}
		st.StoreEvent(ctx, event)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter := nostr.Filter{
			Authors: []string{pubkey},
			Kinds:   []int{1},
			Limit:   20,
		}

		_, err := st.QueryEvents(ctx, filter)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkStorageQueryByID benchmarks querying events by ID
func BenchmarkStorageQueryByID(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	ctx := context.Background()
	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Pre-populate
	eventIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		eventIDs[i] = fmt.Sprintf("event%060d", i)
		event := &nostr.Event{
			ID:        eventIDs[i],
			PubKey:    "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab",
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Kind:      1,
			Content:   "Benchmark event",
			Tags:      nostr.Tags{},
			Sig:       "sig0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}
		st.StoreEvent(ctx, event)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter := nostr.Filter{
			IDs: []string{eventIDs[i%len(eventIDs)]},
		}

		_, err := st.QueryEvents(ctx, filter)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkStorageReplaceableEvent benchmarks replaceable event handling
func BenchmarkStorageReplaceableEvent(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	ctx := context.Background()
	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	pubkey := "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := &nostr.Event{
			ID:        fmt.Sprintf("meta%061d", i),
			PubKey:    pubkey,
			CreatedAt: nostr.Timestamp(time.Now().Unix() + int64(i)),
			Kind:      0, // Replaceable
			Content:   `{"name":"Alice"}`,
			Tags:      nostr.Tags{},
			Sig:       "sig0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}

		if err := st.StoreEvent(ctx, event); err != nil {
			b.Fatalf("Failed to store event: %v", err)
		}
	}
}

// Run all benchmarks with:
// go test -bench=. -benchmem ./test/benchmark/...

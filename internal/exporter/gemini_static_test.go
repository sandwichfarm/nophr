package exporter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func TestGeminiExporterGeneratesOnOwnerRoot(t *testing.T) {
	priv := nostr.GeneratePrivateKey()
	pub, err := nostr.GetPublicKey(priv)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	npub, _ := nip19.EncodePublicKey(pub)

	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Identity.Npub = npub
	cfg.Export.Gemini.Enabled = true
	cfg.Export.Gemini.OutputDir = tmp
	cfg.Export.Gemini.Host = "example.com"
	cfg.Export.Gemini.Port = 1965
	cfg.Export.Gemini.MaxItems = 50
	cfg.Storage = config.Storage{
		Driver:     "sqlite",
		SQLitePath: ":memory:",
	}

	ctx := context.Background()
	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer st.Close()

	exporter, err := NewGeminiExporter(cfg, st)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	note := nostr.Event{
		Kind:      1,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		PubKey:    pub,
		Content:   "Hello static gemini",
	}
	if err := note.Sign(priv); err != nil {
		t.Fatalf("failed to sign note: %v", err)
	}
	if err := st.StoreEvent(ctx, &note); err != nil {
		t.Fatalf("failed to store note: %v", err)
	}

	article := nostr.Event{
		Kind:      30023,
		CreatedAt: nostr.Timestamp(time.Now().Add(time.Second).Unix()),
		PubKey:    pub,
		Content:   "Long form gemini content",
	}
	if err := article.Sign(priv); err != nil {
		t.Fatalf("failed to sign article: %v", err)
	}
	if err := st.StoreEvent(ctx, &article); err != nil {
		t.Fatalf("failed to store article: %v", err)
	}

	exporter.HandleEvent(ctx, &article)

	rootIndex := filepath.Join(tmp, "index.gmi")
	if _, err := os.Stat(rootIndex); err != nil {
		t.Fatalf("expected root index to be written: %v", err)
	}

	notesIndex := filepath.Join(tmp, "notes", "index.gmi")
	if _, err := os.Stat(notesIndex); err != nil {
		t.Fatalf("expected notes index to be written: %v", err)
	}

	articlesIndex := filepath.Join(tmp, "articles", "index.gmi")
	if _, err := os.Stat(articlesIndex); err != nil {
		t.Fatalf("expected articles index to be written: %v", err)
	}

	noteFile := filepath.Join(tmp, "notes", note.ID+".gmi")
	content, err := os.ReadFile(noteFile)
	if err != nil {
		t.Fatalf("expected note file to be written: %v", err)
	}
	if !strings.Contains(string(content), "Hello static gemini") {
		t.Fatalf("note file should contain content, got: %s", string(content))
	}
}

func TestGeminiExporterIgnoresReply(t *testing.T) {
	priv := nostr.GeneratePrivateKey()
	pub, err := nostr.GetPublicKey(priv)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	npub, _ := nip19.EncodePublicKey(pub)

	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Identity.Npub = npub
	cfg.Export.Gemini.Enabled = true
	cfg.Export.Gemini.OutputDir = tmp
	cfg.Storage = config.Storage{
		Driver:     "sqlite",
		SQLitePath: ":memory:",
	}

	ctx := context.Background()
	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer st.Close()

	exporter, err := NewGeminiExporter(cfg, st)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	reply := nostr.Event{
		Kind:      1,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		PubKey:    pub,
		Content:   "Reply content",
		Tags: nostr.Tags{
			{"e", "parent"},
		},
	}
	if err := reply.Sign(priv); err != nil {
		t.Fatalf("failed to sign reply: %v", err)
	}
	if err := st.StoreEvent(ctx, &reply); err != nil {
		t.Fatalf("failed to store reply: %v", err)
	}

	exporter.HandleEvent(ctx, &reply)

	if _, err := os.Stat(filepath.Join(tmp, "index.gmi")); !os.IsNotExist(err) {
		t.Fatalf("expected no export for reply, but index.gmi exists")
	}
}

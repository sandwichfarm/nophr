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
	"github.com/sandwich/nophr/internal/config"
	"github.com/sandwich/nophr/internal/storage"
)

func TestGopherExporterGeneratesOnOwnerRoot(t *testing.T) {
	priv := nostr.GeneratePrivateKey()
	pub, err := nostr.GetPublicKey(priv)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	npub, _ := nip19.EncodePublicKey(pub)

	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Identity.Npub = npub
	cfg.Export.Gopher.Enabled = true
	cfg.Export.Gopher.OutputDir = tmp
	cfg.Export.Gopher.Host = "example.com"
	cfg.Export.Gopher.Port = 70
	cfg.Export.Gopher.MaxItems = 50
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

	exporter, err := NewGopherExporter(cfg, st)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	note := nostr.Event{
		Kind:      1,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		PubKey:    pub,
		Content:   "Hello static gopher",
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
		Content:   "Long form content",
	}
	if err := article.Sign(priv); err != nil {
		t.Fatalf("failed to sign article: %v", err)
	}
	if err := st.StoreEvent(ctx, &article); err != nil {
		t.Fatalf("failed to store article: %v", err)
	}

	exporter.HandleEvent(ctx, &article)

	rootMap := filepath.Join(tmp, "gophermap")
	if _, err := os.Stat(rootMap); err != nil {
		t.Fatalf("expected root gophermap to be written: %v", err)
	}

	notesMap := filepath.Join(tmp, "notes", "gophermap")
	if _, err := os.Stat(notesMap); err != nil {
		t.Fatalf("expected notes gophermap to be written: %v", err)
	}

	articlesMap := filepath.Join(tmp, "articles", "gophermap")
	if _, err := os.Stat(articlesMap); err != nil {
		t.Fatalf("expected articles gophermap to be written: %v", err)
	}

	noteFile := filepath.Join(tmp, "notes", note.ID+".txt")
	content, err := os.ReadFile(noteFile)
	if err != nil {
		t.Fatalf("expected note file to be written: %v", err)
	}
	if !strings.Contains(string(content), "Hello static gopher") {
		t.Fatalf("note file should contain content, got: %s", string(content))
	}
}

func TestGopherExporterIgnoresReply(t *testing.T) {
	priv := nostr.GeneratePrivateKey()
	pub, err := nostr.GetPublicKey(priv)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	npub, _ := nip19.EncodePublicKey(pub)

	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Identity.Npub = npub
	cfg.Export.Gopher.Enabled = true
	cfg.Export.Gopher.OutputDir = tmp
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

	exporter, err := NewGopherExporter(cfg, st)
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

	if _, err := os.Stat(filepath.Join(tmp, "gophermap")); !os.IsNotExist(err) {
		t.Fatalf("expected no export for reply, but gophermap exists")
	}
}

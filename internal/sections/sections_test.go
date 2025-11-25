package sections

import (
	"strings"
	"testing"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func TestDefaultSections(t *testing.T) {
	sections := DefaultSections()

	if len(sections) == 0 {
		t.Fatal("expected default sections, got none")
	}

	expectedSections := map[string]bool{
		"notes":     false,
		"articles":  false,
		"reactions": false,
		"zaps":      false,
	}

	for _, section := range sections {
		if _, exists := expectedSections[section.Name]; exists {
			expectedSections[section.Name] = true
		}
	}

	for name, found := range expectedSections {
		if !found {
			t.Errorf("expected default section %s not found", name)
		}
	}
}

// Note: InboxSection and OutboxSection tests removed - these are now config-based
// Inbox/outbox sections are defined in YAML configuration instead of code

func TestSectionManager(t *testing.T) {
	manager := NewManager(nil, "ownerhex")

	t.Run("Register section", func(t *testing.T) {
		section := &Section{
			Name:  "test-section",
			Title: "Test Section",
			Limit: 10,
		}

		err := manager.RegisterSection(section)
		if err != nil {
			t.Fatalf("failed to register section: %v", err)
		}

		retrieved, err := manager.GetSection("test-section")
		if err != nil {
			t.Fatalf("failed to get section: %v", err)
		}

		if retrieved.Name != "test-section" {
			t.Errorf("expected name 'test-section', got '%s'", retrieved.Name)
		}
	})

	t.Run("Get nonexistent section", func(t *testing.T) {
		_, err := manager.GetSection("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent section")
		}
	})

	t.Run("List sections", func(t *testing.T) {
		sections := manager.ListSections()
		if len(sections) == 0 {
			t.Error("expected at least one section")
		}
	})

	t.Run("Register section without name", func(t *testing.T) {
		section := &Section{
			Title: "No Name",
		}

		err := manager.RegisterSection(section)
		if err == nil {
			t.Error("expected error when registering section without name")
		}
	})

	t.Run("Default limit", func(t *testing.T) {
		section := &Section{
			Name:  "no-limit",
			Title: "No Limit Section",
		}

		manager.RegisterSection(section)

		retrieved, _ := manager.GetSection("no-limit")
		if retrieved.Limit != 20 {
			t.Errorf("expected default limit 20, got %d", retrieved.Limit)
		}
	})
}

func TestArchiveFormatting(t *testing.T) {
	t.Run("Day archive", func(t *testing.T) {
		archive := &Archive{
			Period: ArchiveByDay,
			Year:   2025,
			Month:  10,
			Day:    24,
		}

		title := archive.FormatTitle()
		if title != "October 24, 2025" {
			t.Errorf("unexpected day archive title: %s", title)
		}

		selector := archive.FormatArchiveSelector("notes")
		expected := "/archive/notes/2025/10/24"
		if selector != expected {
			t.Errorf("expected selector %s, got %s", expected, selector)
		}
	})

	t.Run("Month archive", func(t *testing.T) {
		archive := &Archive{
			Period: ArchiveByMonth,
			Year:   2025,
			Month:  10,
		}

		title := archive.FormatTitle()
		if title != "October 2025" {
			t.Errorf("unexpected month archive title: %s", title)
		}

		selector := archive.FormatArchiveSelector("notes")
		expected := "/archive/notes/2025/10"
		if selector != expected {
			t.Errorf("expected selector %s, got %s", expected, selector)
		}
	})

	t.Run("Year archive", func(t *testing.T) {
		archive := &Archive{
			Period: ArchiveByYear,
			Year:   2025,
		}

		title := archive.FormatTitle()
		if title != "2025" {
			t.Errorf("unexpected year archive title: %s", title)
		}

		selector := archive.FormatArchiveSelector("notes")
		expected := "/archive/notes/2025"
		if selector != expected {
			t.Errorf("expected selector %s, got %s", expected, selector)
		}
	})
}

func TestResolveAuthorsDefaultsAndNpub(t *testing.T) {
	ownerHex := strings.Repeat("1", 64)
	ownerNpub, err := nip19.EncodePublicKey(ownerHex)
	if err != nil {
		t.Fatalf("failed to encode owner npub: %v", err)
	}

	manager := NewManager(nil, ownerNpub)

	t.Run("DefaultsToOwner", func(t *testing.T) {
		authors := manager.resolveAuthors(nil, "")
		if len(authors) != 1 || authors[0] != ownerHex {
			t.Fatalf("expected default author to be owner hex, got %+v", authors)
		}
	})

	t.Run("NpubAuthorConversion", func(t *testing.T) {
		otherHex := strings.Repeat("2", 64)
		otherNpub, err := nip19.EncodePublicKey(otherHex)
		if err != nil {
			t.Fatalf("failed to encode other npub: %v", err)
		}

		authors := manager.resolveAuthors([]string{otherNpub}, "")
		if len(authors) != 1 || authors[0] != otherHex {
			t.Fatalf("expected npub to convert to hex, got %+v", authors)
		}
	})

	t.Run("ScopeAllDisablesDefaultOwner", func(t *testing.T) {
		authors := manager.resolveAuthors(nil, ScopeAll)
		if len(authors) != 0 {
			t.Fatalf("expected no authors when scope=all, got %+v", authors)
		}
	})
}

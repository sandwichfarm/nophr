package finger

import (
	"fmt"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/markdown"
	nostrclient "github.com/sandwichfarm/nophr/internal/nostr"
)

// Renderer renders Finger protocol responses
type Renderer struct {
	parser *markdown.Parser
}

// NewRenderer creates a new renderer
func NewRenderer() *Renderer {
	return &Renderer{
		parser: markdown.NewParser(),
	}
}

// RenderUser renders user information in Finger format
func (r *Renderer) RenderUser(pubkey string, profile *nostr.Event, notes interface{}, verbose bool) string {
	var sb strings.Builder

	// Parse profile metadata using proper parser
	var meta *nostrclient.ProfileMetadata
	if profile != nil {
		meta = nostrclient.ParseProfile(profile)
	}
	if meta == nil {
		meta = &nostrclient.ProfileMetadata{} // Empty profile
	}

	// Header line with display name
	displayName := meta.GetDisplayName()
	if displayName == "" {
		displayName = truncatePubkey(pubkey)
	}

	sb.WriteString(fmt.Sprintf("User: %s\n", displayName))

	// Basic info (always shown)
	if meta.NIP05 != "" {
		sb.WriteString(fmt.Sprintf("NIP-05: %s\n", meta.NIP05))
	}
	if meta.Name != "" && meta.DisplayName != "" && meta.Name != meta.DisplayName {
		sb.WriteString(fmt.Sprintf("Name: %s\n", meta.Name))
	}
	sb.WriteString(fmt.Sprintf("Pubkey: %s\n", truncatePubkey(pubkey)))

	// Lightning address (basic info)
	lightningAddr := meta.GetLightningAddress()
	if lightningAddr != "" {
		sb.WriteString(fmt.Sprintf("Lightning: %s\n", lightningAddr))
	}

	// Verbose mode shows more details
	if verbose {
		if meta.About != "" {
			// Render about text compactly
			about, _ := r.parser.RenderFinger([]byte(meta.About), &markdown.RenderOptions{
				Width:       80,
				CompactMode: true,
			})
			sb.WriteString(fmt.Sprintf("\nAbout:\n%s\n", about))
		}

		// Additional contact info in verbose mode
		if meta.Website != "" {
			sb.WriteString(fmt.Sprintf("Website: %s\n", meta.Website))
		}

		// Show recent activity
		sb.WriteString("\nRecent Activity:\n")
		sb.WriteString(strings.Repeat("-", 70))
		sb.WriteString("\n")

		switch n := notes.(type) {
		case []*enrichedNote:
			if len(n) == 0 {
				sb.WriteString("No recent notes\n")
			} else {
				for i, note := range n {
					if i >= 5 {
						break
					}
					sb.WriteString(r.renderNoteCompact(note.Event))
					sb.WriteString("\n")
				}
			}
		default:
			sb.WriteString("No recent notes\n")
		}
	} else {
		// Non-verbose: just show summary
		switch n := notes.(type) {
		case []*enrichedNote:
			if len(n) > 0 {
				sb.WriteString(fmt.Sprintf("\nLast post: %s\n", formatTimestamp(n[0].Event.CreatedAt)))
			}
		}
	}

	return sb.String()
}

// renderNoteCompact renders a note in compact format
func (r *Renderer) renderNoteCompact(event *nostr.Event) string {
	var sb strings.Builder

	// Timestamp
	sb.WriteString(fmt.Sprintf("[%s] ", formatTimestamp(event.CreatedAt)))

	// Content (first line, max 60 chars)
	content := event.Content
	if len(content) > 60 {
		content = content[:57] + "..."
	}
	firstLine := strings.Split(content, "\n")[0]

	// Render markdown compactly
	rendered, _ := r.parser.RenderFinger([]byte(firstLine), &markdown.RenderOptions{
		Width:           60,
		CompactMode:     true,
		StripFormatting: true,
	})

	sb.WriteString(strings.TrimSpace(rendered))

	return sb.String()
}

// truncatePubkey truncates a pubkey for display
func truncatePubkey(pubkey string) string {
	if len(pubkey) <= 16 {
		return pubkey
	}
	return pubkey[:8] + "..." + pubkey[len(pubkey)-8:]
}

// formatTimestamp formats a Nostr timestamp for finger output
func formatTimestamp(ts nostr.Timestamp) string {
	t := time.Unix(int64(ts), 0)
	now := time.Now()

	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}

	return t.Format("Jan 2")
}

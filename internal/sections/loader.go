package sections

import (
	"fmt"
	"time"

	"github.com/sandwich/nophr/internal/config"
)

// LoadFromConfig converts config.SectionConfig entries to Section instances
// and registers them with the provided Manager
func LoadFromConfig(manager *Manager, sectionConfigs []config.SectionConfig) error {
	for _, cfg := range sectionConfigs {
		section, err := convertConfigToSection(cfg)
		if err != nil {
			return fmt.Errorf("failed to convert section %s: %w", cfg.Name, err)
		}

		if err := manager.RegisterSection(section); err != nil {
			return fmt.Errorf("failed to register section %s: %w", cfg.Name, err)
		}
	}

	return nil
}

// convertConfigToSection converts a config.SectionConfig to a Section
func convertConfigToSection(cfg config.SectionConfig) (*Section, error) {
	section := &Section{
		Name:        cfg.Name,
		Path:        cfg.Path,
		Title:       cfg.Title,
		Description: cfg.Description,
		ShowDates:   cfg.ShowDates,
		ShowAuthors: cfg.ShowAuthors,
		Order:       cfg.Order,
	}

	// Set limit (default to 20 if not specified)
	if cfg.Limit > 0 {
		section.Limit = cfg.Limit
	} else {
		section.Limit = 20
	}

	// Convert sort field
	if cfg.SortBy != "" {
		section.SortBy = SortField(cfg.SortBy)
	} else {
		section.SortBy = SortByCreatedAt
	}

	// Convert sort order
	if cfg.SortOrder != "" {
		section.SortOrder = SortOrder(cfg.SortOrder)
	} else {
		section.SortOrder = SortDesc
	}

	// Convert group by
	if cfg.GroupBy != "" {
		section.GroupBy = GroupField(cfg.GroupBy)
	}

	// Convert filters
	filterSet, err := convertFilterConfig(cfg.Filters)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filters: %w", err)
	}
	section.Filters = filterSet

	// Convert more link
	if cfg.MoreLink != nil {
		section.MoreLink = &MoreLink{
			Text:       cfg.MoreLink.Text,
			SectionRef: cfg.MoreLink.SectionRef,
		}
	}

	return section, nil
}

// convertFilterConfig converts a config.SectionFilterConfig to FilterSet
func convertFilterConfig(cfg config.SectionFilterConfig) (FilterSet, error) {
	filterSet := FilterSet{
		Kinds:   cfg.Kinds,
		Authors: cfg.Authors,
		Tags:    cfg.Tags,
		Search:  cfg.Search,
	}

	// Convert scope
	if cfg.Scope != "" {
		filterSet.Scope = Scope(cfg.Scope)
	}

	// Convert reply flag
	if cfg.IsReply != nil {
		filterSet.IsReply = cfg.IsReply
	}

	// Parse time ranges
	if cfg.Since != "" {
		sinceTime, err := parseTimeOrDuration(cfg.Since)
		if err != nil {
			return filterSet, fmt.Errorf("invalid since time: %w", err)
		}
		filterSet.Since = &sinceTime
	}

	if cfg.Until != "" {
		untilTime, err := parseTimeOrDuration(cfg.Until)
		if err != nil {
			return filterSet, fmt.Errorf("invalid until time: %w", err)
		}
		filterSet.Until = &untilTime
	}

	return filterSet, nil
}

// parseTimeOrDuration parses a time string that can be either:
// - RFC3339 timestamp (e.g., "2024-01-01T00:00:00Z")
// - Relative duration (e.g., "-24h", "-7d")
func parseTimeOrDuration(s string) (time.Time, error) {
	// Try parsing as RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try parsing as duration (must start with - or +)
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		// Parse duration
		var duration time.Duration
		var err error

		// Handle days suffix (not in time.ParseDuration)
		if s[len(s)-1] == 'd' {
			// Convert days to hours
			days := s[:len(s)-1]
			var daysInt int
			if _, err := fmt.Sscanf(days, "%d", &daysInt); err != nil {
				return time.Time{}, fmt.Errorf("invalid day duration: %w", err)
			}
			duration = time.Duration(daysInt) * 24 * time.Hour
		} else {
			duration, err = time.ParseDuration(s)
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid duration: %w", err)
			}
		}

		return time.Now().Add(duration), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format (expected RFC3339 or duration like '-24h')")
}

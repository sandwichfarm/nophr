package sections

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Archive represents a time-based archive of events
type Archive struct {
	Period     ArchivePeriod
	Year       int
	Month      time.Month
	Day        int
	EventCount int64
	FirstEvent time.Time
	LastEvent  time.Time
}

// ArchivePeriod defines the archive grouping period
type ArchivePeriod string

const (
	ArchiveByDay   ArchivePeriod = "day"
	ArchiveByMonth ArchivePeriod = "month"
	ArchiveByYear  ArchivePeriod = "year"
)

// ArchiveManager manages time-based archives
type ArchiveManager struct {
	storage *storage.Storage
}

// NewArchiveManager creates a new archive manager
func NewArchiveManager(st *storage.Storage) *ArchiveManager {
	return &ArchiveManager{
		storage: st,
	}
}

// ListArchives returns available archives for a section
func (am *ArchiveManager) ListArchives(ctx context.Context, section *Section, period ArchivePeriod) ([]*Archive, error) {
	// Get all events for the section
	filter := nostr.Filter{}
	if len(section.Filters.Kinds) > 0 {
		filter.Kinds = section.Filters.Kinds
	}
	if len(section.Filters.Authors) > 0 {
		filter.Authors = section.Filters.Authors
	}

	events, err := am.storage.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// Group events by time period
	archiveMap := make(map[string]*Archive)

	for _, event := range events {
		eventTime := time.Unix(int64(event.CreatedAt), 0)
		key := am.getPeriodKey(eventTime, period)

		archive, exists := archiveMap[key]
		if !exists {
			archive = &Archive{
				Period:     period,
				Year:       eventTime.Year(),
				Month:      eventTime.Month(),
				Day:        eventTime.Day(),
				EventCount: 0,
				FirstEvent: eventTime,
				LastEvent:  eventTime,
			}
			archiveMap[key] = archive
		}

		archive.EventCount++
		if eventTime.Before(archive.FirstEvent) {
			archive.FirstEvent = eventTime
		}
		if eventTime.After(archive.LastEvent) {
			archive.LastEvent = eventTime
		}
	}

	// Convert map to sorted slice
	archives := make([]*Archive, 0, len(archiveMap))
	for _, archive := range archiveMap {
		archives = append(archives, archive)
	}

	// Sort by date (newest first)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].LastEvent.After(archives[j].LastEvent)
	})

	return archives, nil
}

// GetArchivePage returns events for a specific archive period
func (am *ArchiveManager) GetArchivePage(ctx context.Context, section *Section, year int, month time.Month, day int, pageNum int) (*Page, error) {
	// Build time range based on archive period
	var start, end time.Time

	if day > 0 {
		// Day archive
		start = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 0, 1)
	} else if month > 0 {
		// Month archive
		start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	} else {
		// Year archive
		start = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(1, 0, 0)
	}

	// Build filter with time range
	filter := nostr.Filter{}
	if len(section.Filters.Kinds) > 0 {
		filter.Kinds = section.Filters.Kinds
	}
	if len(section.Filters.Authors) > 0 {
		filter.Authors = section.Filters.Authors
	}

	since := nostr.Timestamp(start.Unix())
	until := nostr.Timestamp(end.Unix())
	filter.Since = &since
	filter.Until = &until

	// Query events
	events, err := am.storage.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query archive events: %w", err)
	}

	// Sort events
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})

	// Paginate
	if pageNum < 1 {
		pageNum = 1
	}

	limit := section.Limit
	if limit == 0 {
		limit = 20
	}

	offset := (pageNum - 1) * limit
	totalItems := int64(len(events))
	totalPages := int((totalItems + int64(limit) - 1) / int64(limit))

	var pageEvents []*nostr.Event
	if offset < len(events) {
		endIdx := offset + limit
		if endIdx > len(events) {
			endIdx = len(events)
		}
		pageEvents = events[offset:endIdx]
	}

	return &Page{
		Section:    section,
		Events:     pageEvents,
		PageNumber: pageNum,
		TotalPages: totalPages,
		TotalItems: totalItems,
		HasNext:    pageNum < totalPages,
		HasPrev:    pageNum > 1,
	}, nil
}

// getPeriodKey generates a unique key for a time period
func (am *ArchiveManager) getPeriodKey(t time.Time, period ArchivePeriod) string {
	switch period {
	case ArchiveByDay:
		return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
	case ArchiveByMonth:
		return fmt.Sprintf("%04d-%02d", t.Year(), t.Month())
	case ArchiveByYear:
		return fmt.Sprintf("%04d", t.Year())
	default:
		return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
	}
}

// FormatArchiveTitle formats an archive title
func (a *Archive) FormatTitle() string {
	switch a.Period {
	case ArchiveByDay:
		return fmt.Sprintf("%s %d, %d", a.Month.String(), a.Day, a.Year)
	case ArchiveByMonth:
		return fmt.Sprintf("%s %d", a.Month.String(), a.Year)
	case ArchiveByYear:
		return fmt.Sprintf("%d", a.Year)
	default:
		return fmt.Sprintf("%s %d, %d", a.Month.String(), a.Day, a.Year)
	}
}

// FormatArchiveSelector formats an archive selector for navigation
func (a *Archive) FormatArchiveSelector(sectionName string) string {
	switch a.Period {
	case ArchiveByDay:
		return fmt.Sprintf("/archive/%s/%04d/%02d/%02d", sectionName, a.Year, a.Month, a.Day)
	case ArchiveByMonth:
		return fmt.Sprintf("/archive/%s/%04d/%02d", sectionName, a.Year, a.Month)
	case ArchiveByYear:
		return fmt.Sprintf("/archive/%s/%04d", sectionName, a.Year)
	default:
		return fmt.Sprintf("/archive/%s/%04d/%02d/%02d", sectionName, a.Year, a.Month, a.Day)
	}
}

// MonthlyArchiveCalendar generates a calendar view of monthly archives
type MonthlyArchiveCalendar struct {
	Year   int
	Month  time.Month
	Days   []*DayArchive
}

// DayArchive represents a single day in the calendar
type DayArchive struct {
	Day        int
	EventCount int64
	HasEvents  bool
}

// GenerateMonthlyCalendar generates a monthly archive calendar
func (am *ArchiveManager) GenerateMonthlyCalendar(ctx context.Context, section *Section, year int, month time.Month) (*MonthlyArchiveCalendar, error) {
	// Get month range
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	// Build filter
	filter := nostr.Filter{}
	if len(section.Filters.Kinds) > 0 {
		filter.Kinds = section.Filters.Kinds
	}
	if len(section.Filters.Authors) > 0 {
		filter.Authors = section.Filters.Authors
	}

	since := nostr.Timestamp(start.Unix())
	until := nostr.Timestamp(end.Unix())
	filter.Since = &since
	filter.Until = &until

	// Query events
	events, err := am.storage.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events for calendar: %w", err)
	}

	// Count events by day
	dayCounts := make(map[int]int64)
	for _, event := range events {
		eventTime := time.Unix(int64(event.CreatedAt), 0)
		dayCounts[eventTime.Day()]++
	}

	// Generate day list
	daysInMonth := end.AddDate(0, 0, -1).Day()
	days := make([]*DayArchive, daysInMonth)

	for i := 1; i <= daysInMonth; i++ {
		count := dayCounts[i]
		days[i-1] = &DayArchive{
			Day:        i,
			EventCount: count,
			HasEvents:  count > 0,
		}
	}

	return &MonthlyArchiveCalendar{
		Year:  year,
		Month: month,
		Days:  days,
	}, nil
}

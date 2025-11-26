package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/sandwich/nophr/internal/config"
	internalnostr "github.com/sandwich/nophr/internal/nostr"
	"github.com/sandwich/nophr/internal/storage"
)

// Engine manages the synchronization of events from Nostr relays
type Engine struct {
	config        *config.Config
	storage       *storage.Storage
	nostrClient   *internalnostr.Client
	discovery     *internalnostr.Discovery
	filterBuilder *FilterBuilder
	graph         *Graph
	cursors       *CursorManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Channels for coordination
	eventChan chan *nostr.Event

	// Performance optimizations (Balanced Plan - Tier 1)
	eventCache *EventCache // LRU cache for fast deduplication

	// Performance optimizations (Balanced Plan - Tier 2)
	aggregateChan chan *AggregateUpdate // Async aggregate processing

	// Phase 20: Optional retention evaluation callback
	evaluateRetention func(context.Context, *nostr.Event) error

	eventHandlers []EventHandler
}

// AggregateUpdate represents a pending aggregate update
type AggregateUpdate struct {
	Type          string // "reply", "reaction", "zap"
	EventID       string
	Reaction      string // For reactions
	Sats          int64  // For zaps
	InteractionAt int64
}

// EventHandler is notified for each stored event.
type EventHandler func(context.Context, *nostr.Event)

// New creates a new sync engine (legacy signature for compatibility)
func New(ctx context.Context, cfg *config.Config, st *storage.Storage, client *internalnostr.Client) *Engine {
	engineCtx, cancel := context.WithCancel(ctx)

	discovery := internalnostr.NewDiscovery(client, st)
	filterBuilder := NewFilterBuilder(&cfg.Sync)
	graph := NewGraph(st, &cfg.Sync.Scope)
	cursors := NewCursorManager(st)

	return &Engine{
		config:        cfg,
		storage:       st,
		nostrClient:   client,
		discovery:     discovery,
		filterBuilder: filterBuilder,
		graph:         graph,
		cursors:       cursors,
		ctx:           engineCtx,
		cancel:        cancel,
		eventChan:     make(chan *nostr.Event, 5000),     // Tier 2: Larger buffer for burst handling
		eventCache:    NewEventCache(5000),               // Tier 1: Cache last 5000 event IDs
		aggregateChan: make(chan *AggregateUpdate, 1000), // Tier 2: Async aggregate queue
	}
}

// NewEngine creates a new sync engine with storage and config only
func NewEngine(st *storage.Storage, cfg *config.Config) *Engine {
	ctx := context.Background()
	engineCtx, cancel := context.WithCancel(ctx)

	// Create nostr client
	nostrClient := internalnostr.New(ctx, &cfg.Relays)

	discovery := internalnostr.NewDiscovery(nostrClient, st)
	filterBuilder := NewFilterBuilder(&cfg.Sync)
	graph := NewGraph(st, &cfg.Sync.Scope)
	cursors := NewCursorManager(st)

	return &Engine{
		config:        cfg,
		storage:       st,
		nostrClient:   nostrClient,
		discovery:     discovery,
		filterBuilder: filterBuilder,
		graph:         graph,
		cursors:       cursors,
		ctx:           engineCtx,
		cancel:        cancel,
		eventChan:     make(chan *nostr.Event, 5000),     // Tier 2: Larger buffer for burst handling
		eventCache:    NewEventCache(5000),               // Tier 1: Cache last 5000 event IDs
		aggregateChan: make(chan *AggregateUpdate, 1000), // Tier 2: Async aggregate queue
	}
}

// Start begins the sync process
func (e *Engine) Start() error {
	// Bootstrap from seed relays
	if err := e.bootstrap(); err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// Tier 2 Optimization: Start event ingestion workers for parallel processing
	workerCount := e.config.Sync.Performance.Workers
	if workerCount <= 0 {
		workerCount = 4 // Safety fallback
	}
	fmt.Printf("[SYNC] Starting %d event processing workers\n", workerCount)
	for i := 0; i < workerCount; i++ {
		e.wg.Add(1)
		go e.eventWorker(i + 1)
	}

	// Tier 2 Optimization: Start async aggregate worker
	e.wg.Add(1)
	go e.processAggregates()

	// Start continuous sync
	e.wg.Add(1)
	go e.continuousSync()

	// Start periodic refresh of replaceables
	e.wg.Add(1)
	go e.periodicRefresh()

	return nil
}

// Stop gracefully stops the sync engine
func (e *Engine) Stop() {
	e.cancel()
	close(e.eventChan)
	close(e.aggregateChan) // Tier 2: Close aggregate channel
	e.wg.Wait()
}

// AddEventHandler registers an optional event handler.
func (e *Engine) AddEventHandler(handler EventHandler) {
	if handler == nil {
		return
	}
	e.eventHandlers = append(e.eventHandlers, handler)
}

// SetRetentionEvaluator sets the retention evaluation callback (Phase 20)
func (e *Engine) SetRetentionEvaluator(fn func(context.Context, *nostr.Event) error) {
	e.evaluateRetention = fn
}

// getOwnerPubkey decodes the npub to hex pubkey
func (e *Engine) getOwnerPubkey() (string, error) {
	if _, hex, err := nip19.Decode(e.config.Identity.Npub); err != nil {
		return "", fmt.Errorf("failed to decode npub: %w", err)
	} else {
		return hex.(string), nil
	}
}

// bootstrap performs initial discovery and graph building
func (e *Engine) bootstrap() error {
	fmt.Printf("[SYNC] Starting bootstrap process...\n")
	ownerPubkey, err := e.getOwnerPubkey()
	if err != nil {
		return err
	}
	fmt.Printf("[SYNC] Owner pubkey (hex): %s\n", ownerPubkey)

	// Step 1: Fetch owner's profile, contacts, and relay hints from seeds
	fmt.Printf("[SYNC] Step 1: Bootstrapping from seed relays...\n")
	if err := e.discovery.BootstrapFromSeeds(e.ctx, ownerPubkey); err != nil {
		return fmt.Errorf("failed to bootstrap from seeds: %w", err)
	}
	fmt.Printf("[SYNC] ✓ Bootstrap from seeds complete\n")

	// Step 2: Fetch owner's contact list (kind 3) to build initial graph
	seedRelays := e.nostrClient.GetSeedRelays()
	fmt.Printf("[SYNC] Step 2: Fetching contact list from %d seed relays\n", len(seedRelays))
	for i, relay := range seedRelays {
		fmt.Printf("[SYNC]   Seed relay %d: %s\n", i+1, relay)
	}

	filter := nostr.Filter{
		Kinds:   []int{3},
		Authors: []string{ownerPubkey},
		Limit:   1,
	}

	events, err := e.nostrClient.FetchEvents(e.ctx, seedRelays, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch contact list: %w", err)
	}
	fmt.Printf("[SYNC] Fetched %d contact list events\n", len(events))

	if len(events) > 0 {
		// Process the contact list to build the graph
		fmt.Printf("[SYNC] Processing contact list (event ID: %s)\n", events[0].ID)
		if err := e.graph.ProcessContactList(e.ctx, events[0], ownerPubkey); err != nil {
			return fmt.Errorf("failed to process contact list: %w", err)
		}
		fmt.Printf("[SYNC] ✓ Contact list processed\n")
	} else {
		fmt.Printf("[SYNC] ⚠ No contact list found - will sync owner events only\n")
	}

	// Step 3: Get authors in scope
	fmt.Printf("[SYNC] Step 3: Getting authors in scope...\n")
	authors, err := e.graph.GetAuthorsInScope(e.ctx, ownerPubkey)
	if err != nil {
		return fmt.Errorf("failed to get authors in scope: %w", err)
	}
	fmt.Printf("[SYNC] Authors in scope: %d\n", len(authors))
	if len(authors) <= 5 {
		for i, author := range authors {
			fmt.Printf("[SYNC]   Author %d: %s\n", i+1, author[:16]+"...")
		}
	} else {
		fmt.Printf("[SYNC]   (First 5 authors shown)\n")
		for i := 0; i < 5; i++ {
			fmt.Printf("[SYNC]   Author %d: %s\n", i+1, authors[i][:16]+"...")
		}
	}

	// Step 4: Discover relay hints for all authors in scope
	fmt.Printf("[SYNC] Step 4: Discovering relay hints...\n")
	// Get owner's outbox relays to search for authors' relay hints
	ownerRelays, err := e.discovery.GetOutboxRelays(e.ctx, ownerPubkey)
	if err != nil || len(ownerRelays) == 0 {
		ownerRelays = seedRelays // Fallback to seeds
		fmt.Printf("[SYNC] Using seed relays as fallback (%d relays)\n", len(ownerRelays))
	} else {
		fmt.Printf("[SYNC] Using owner's outbox relays (%d relays)\n", len(ownerRelays))
	}

	if err := e.discovery.DiscoverRelayHintsForPubkeys(e.ctx, authors, ownerRelays); err != nil {
		return fmt.Errorf("failed to discover relay hints: %w", err)
	}
	fmt.Printf("[SYNC] ✓ Relay hints discovered\n")
	fmt.Printf("[SYNC] ✓ Bootstrap complete!\n\n")

	return nil
}

// continuousSync runs the main sync loop with adaptive intervals
func (e *Engine) continuousSync() {
	defer e.wg.Done()

	// Tier 1 Optimization: Smart adaptive sync intervals
	interval := 10 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	eventsInLastSync := 0

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			// Track events before sync
			sizeBefore := e.eventCache.Size()

			if err := e.syncOnce(); err != nil {
				// Log error but continue
				fmt.Printf("Sync error: %v\n", err)
			}

			// Estimate events received (rough approximation)
			sizeAfter := e.eventCache.Size()
			eventsInLastSync = sizeAfter - sizeBefore
			if eventsInLastSync < 0 {
				eventsInLastSync = 0 // Cache may have evicted old entries
			}

			// Adapt sync interval based on activity
			var newInterval time.Duration
			if eventsInLastSync == 0 {
				newInterval = 30 * time.Second // Slow when idle
			} else if eventsInLastSync < 50 {
				newInterval = 10 * time.Second // Normal activity
			} else {
				newInterval = 5 * time.Second // High activity
			}

			// Only reset ticker if interval changed
			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
				fmt.Printf("[SYNC] Adaptive interval: %v (received %d events)\n", interval, eventsInLastSync)
			}
		}
	}
}

// syncOnce performs a single sync iteration
func (e *Engine) syncOnce() error {
	fmt.Printf("[SYNC] Starting sync iteration...\n")
	ownerPubkey, err := e.getOwnerPubkey()
	if err != nil {
		return err
	}

	// Get authors in scope
	authors, err := e.graph.GetAuthorsInScope(e.ctx, ownerPubkey)
	if err != nil {
		return fmt.Errorf("failed to get authors: %w", err)
	}
	fmt.Printf("[SYNC] Syncing for %d authors\n", len(authors))

	// Get relays to sync from
	relays := e.getActiveRelays(authors)
	if len(relays) == 0 {
		fmt.Printf("[SYNC] ⚠ No active relays found!\n")
		return fmt.Errorf("no active relays")
	}
	fmt.Printf("[SYNC] Active relays: %d\n", len(relays))

	// Build filters with cursors
	kinds := e.filterBuilder.GetConfiguredKinds()
	fmt.Printf("[SYNC] Configured event kinds: %v\n", kinds)

	// STEP 1: Sync authors' posts from their OUTBOX (write relays)
	for i, relay := range relays {
		fmt.Printf("[SYNC] Processing outbox relay %d/%d: %s\n", i+1, len(relays), relay)

		// Get since cursor for this relay
		since, err := e.cursors.GetSinceCursorForRelay(e.ctx, relay, kinds)
		if err != nil {
			fmt.Printf("[SYNC]   ⚠ Failed to get cursor: %v\n", err)
			continue
		}
		if since > 0 {
			fmt.Printf("[SYNC]   Since cursor: %d (%s)\n", since, time.Unix(int64(since), 0).Format(time.RFC3339))
		} else {
			fmt.Printf("[SYNC]   Since cursor: 0 (fetching all history)\n")
		}

		// Build filters for authors' posts (outbox)
		filters := e.filterBuilder.BuildFilters(authors, since)
		fmt.Printf("[SYNC]   Built %d filters for outbox\n", len(filters))

		// Try negentropy sync first, fall back to REQ if unsupported
		go e.syncRelayWithFallback(relay, filters)
	}

	// STEP 2: Sync interactions TO US from OUR INBOX (read relays)
	if e.config.Sync.Scope.IncludeDirectMentions {
		if err := e.syncOwnerInbox(ownerPubkey, kinds); err != nil {
			fmt.Printf("[SYNC] ⚠ Inbox sync failed: %v\n", err)
			// Don't fail the whole sync if inbox fails
		}
	}

	fmt.Printf("[SYNC] ✓ Sync iteration dispatched\n\n")
	return nil
}

// syncRelayWithFallback tries negentropy sync first, falls back to REQ if unsupported
func (e *Engine) syncRelayWithFallback(relay string, filters []nostr.Filter) {
	// Check if negentropy is enabled
	if !e.config.Sync.Performance.UseNegentropy {
		// Negentropy disabled, use traditional REQ
		e.subscribeRelay(relay, filters)
		return
	}

	// Optimization: For negentropy, combine all filters into one complete-set filter
	// Negentropy excels at reconciling complete datasets, not incremental syncs
	// Extract all unique authors and kinds from the filters
	authorSet := make(map[string]bool)
	kindSet := make(map[int]bool)

	for _, filter := range filters {
		for _, author := range filter.Authors {
			authorSet[author] = true
		}
		for _, kind := range filter.Kinds {
			kindSet[kind] = true
		}
	}

	// Convert sets to slices
	authors := make([]string, 0, len(authorSet))
	for author := range authorSet {
		authors = append(authors, author)
	}
	kinds := make([]int, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}

	// Build optimized negentropy filter (no since cursor, complete dataset)
	negentropyFilter := nostr.Filter{
		Authors: authors,
		Kinds:   kinds,
		// No 'since' - negentropy figures out what's missing efficiently
	}

	fmt.Printf("[SYNC] Trying negentropy for %s (%d authors, %d kinds, complete set)\n", relay, len(authors), len(kinds))

	// Try negentropy with the optimized complete-set filter
	success, err := e.NegentropySync(e.ctx, relay, negentropyFilter)
	if err != nil {
		// Hard error - log and fall back to REQ
		fmt.Printf("[SYNC] ⚠ Negentropy error for %s: %v (falling back to REQ)\n", relay, err)
	} else if success {
		// Negentropy succeeded - we're done!
		fmt.Printf("[SYNC] ✓ Negentropy sync complete for %s\n", relay)
		return
	}

	// Fall back to traditional REQ-based sync (always enabled for reliability)
	// REQ uses cursor-based incremental sync (efficient for traditional subscriptions)
	fmt.Printf("[SYNC] Using traditional REQ for %s\n", relay)
	e.subscribeRelay(relay, filters)
}

// syncOwnerInbox syncs interactions directed at the owner from their INBOX (read relays)
// This queries for mentions, replies, reactions, and zaps TO the owner
func (e *Engine) syncOwnerInbox(ownerPubkey string, kinds []int) error {
	fmt.Printf("[SYNC] Starting inbox sync for owner...\n")

	// Get owner's INBOX relays (read relays where they receive interactions)
	inboxRelays, err := e.discovery.GetInboxRelays(e.ctx, ownerPubkey)
	if err != nil {
		return fmt.Errorf("failed to get inbox relays: %w", err)
	}

	if len(inboxRelays) == 0 {
		fmt.Printf("[SYNC] ⚠ No inbox relays found for owner, using seed relays as fallback\n")
		inboxRelays = e.nostrClient.GetSeedRelays()
	}

	fmt.Printf("[SYNC] Owner inbox relays: %d\n", len(inboxRelays))

	// Get since cursor for inbox sync
	// Use a special "inbox" cursor key to track inbox sync separately
	since := nostr.Timestamp(0)
	for _, relay := range inboxRelays {
		relaySince, err := e.cursors.GetSinceCursorForRelay(e.ctx, relay, kinds)
		if err == nil && relaySince > 0 {
			if since == 0 || nostr.Timestamp(relaySince) < since {
				since = nostr.Timestamp(relaySince)
			}
		}
	}

	// Build inbox filter (mentions, replies, reactions, zaps TO owner)
	inboxFilter := e.filterBuilder.BuildInboxFilter(ownerPubkey, int64(since))
	if len(inboxFilter.Kinds) == 0 {
		fmt.Printf("[SYNC] No interaction kinds enabled for inbox, skipping\n")
		return nil
	}

	fmt.Printf("[SYNC] Inbox filter kinds: %v\n", inboxFilter.Kinds)
	if since > 0 {
		fmt.Printf("[SYNC] Inbox since cursor: %d (%s)\n", since, time.Unix(int64(since), 0).Format(time.RFC3339))
	}

	// Sync from each inbox relay
	for i, relay := range inboxRelays {
		fmt.Printf("[SYNC] Processing inbox relay %d/%d: %s\n", i+1, len(inboxRelays), relay)
		go e.syncRelayWithFallback(relay, []nostr.Filter{inboxFilter})
	}

	return nil
}

// subscribeRelay subscribes to a relay with the given filters (traditional REQ-based sync)
func (e *Engine) subscribeRelay(relay string, filters []nostr.Filter) {
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()

	fmt.Printf("[SYNC] Subscribing to %s...\n", relay)
	eventChan := e.nostrClient.SubscribeEvents(ctx, []string{relay}, filters)

	eventCount := 0
	for event := range eventChan {
		eventCount++
		if eventCount == 1 {
			fmt.Printf("[SYNC] ✓ Receiving events from %s\n", relay)
		}
		select {
		case e.eventChan <- event:
		case <-e.ctx.Done():
			fmt.Printf("[SYNC] Subscription to %s cancelled (context done)\n", relay)
			return
		}
	}

	if eventCount > 0 {
		fmt.Printf("[SYNC] ✓ Received %d events from %s\n", eventCount, relay)
	} else {
		fmt.Printf("[SYNC] No events received from %s\n", relay)
	}
}

// eventWorker processes events from the event channel (Tier 2: parallel processing)
func (e *Engine) eventWorker(workerID int) {
	defer e.wg.Done()

	fmt.Printf("[SYNC] Worker %d started\n", workerID)
	eventCount := 0

	for event := range e.eventChan {
		eventCount++
		if eventCount%10 == 1 {
			fmt.Printf("[SYNC] Worker %d: Processing event %d (kind %d, author: %s)\n", workerID, eventCount, event.Kind, event.PubKey[:16]+"...")
		}

		if err := e.processEvent(event); err != nil {
			// Log error but continue
			fmt.Printf("[SYNC] ⚠ Worker %d: Event processing error: %v\n", workerID, err)
		}
	}

	fmt.Printf("[SYNC] Worker %d stopped (processed %d events)\n", workerID, eventCount)
}

// processEvent handles a single event
func (e *Engine) processEvent(event *nostr.Event) error {
	// Tier 1 Optimization: Fast deduplication using LRU cache
	if e.eventCache.Contains(event.ID) {
		// Very likely a duplicate - verify with DB
		exists, err := e.storage.EventExists(e.ctx, event.ID)
		if err == nil && exists {
			return nil // Skip duplicate (saves ~90% of duplicate DB writes)
		}
	}

	// Store event in Khatru
	if err := e.storage.StoreEvent(e.ctx, event); err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	// Add to cache after successful storage
	e.eventCache.Add(event.ID)

	fmt.Printf("[SYNC]   ✓ Stored event %s (kind %d)\n", event.ID[:16]+"...", event.Kind)

	// Handle special event kinds
	switch event.Kind {
	case 3:
		// Contact list - update graph
		if err := e.graph.ProcessContactList(e.ctx, event, e.config.Identity.Npub); err != nil {
			return fmt.Errorf("failed to process contact list: %w", err)
		}

		// Recompute mutuals
		if err := e.graph.ComputeMutuals(e.ctx, e.config.Identity.Npub); err != nil {
			return fmt.Errorf("failed to compute mutuals: %w", err)
		}

	case 10002:
		// Relay hints - update relay hints
		hints, err := internalnostr.ParseRelayHints(event)
		if err != nil {
			return fmt.Errorf("failed to parse relay hints: %w", err)
		}

		for _, hint := range hints {
			if err := e.storage.SaveRelayHint(e.ctx, hint); err != nil {
				return fmt.Errorf("failed to save relay hint: %w", err)
			}
		}

	case 7:
		// Tier 2 Optimization: Queue reaction aggregate update (async, non-blocking)
		e.queueReactionUpdate(event)

	case 1:
		// Tier 2 Optimization: Queue reply aggregate update (async, non-blocking)
		e.queueReplyUpdate(event)

	case 9735:
		// Tier 2 Optimization: Queue zap aggregate update (async, non-blocking)
		e.queueZapUpdate(event)
	}

	// Phase 20: Evaluate retention if enabled
	if e.evaluateRetention != nil {
		if err := e.evaluateRetention(e.ctx, event); err != nil {
			// Log error but don't fail the entire event processing
			fmt.Printf("[SYNC]   ⚠ Retention evaluation error: %v\n", err)
		}
	}

	e.notifyEventHandlers(event)

	return nil
}

func (e *Engine) notifyEventHandlers(event *nostr.Event) {
	if len(e.eventHandlers) == 0 {
		return
	}

	for _, handler := range e.eventHandlers {
		handler(e.ctx, event)
	}
}

// periodicRefresh refreshes replaceable events periodically
func (e *Engine) periodicRefresh() {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if err := e.refreshReplaceables(); err != nil {
				fmt.Printf("Refresh error: %v\n", err)
			}
		}
	}
}

// refreshReplaceables refreshes replaceable events (kinds 0, 3, 10002)
func (e *Engine) refreshReplaceables() error {
	ownerPubkey, err := e.getOwnerPubkey()
	if err != nil {
		return err
	}

	// Get authors in scope
	authors, err := e.graph.GetAuthorsInScope(e.ctx, ownerPubkey)
	if err != nil {
		return err
	}

	// Get active relays
	relays := e.getActiveRelays(authors)
	if len(relays) == 0 {
		return fmt.Errorf("no active relays")
	}

	// Build replaceable filter (no since cursor)
	filter := e.filterBuilder.BuildReplaceableFilter(authors)

	// Fetch events
	events, err := e.nostrClient.FetchEvents(e.ctx, relays, filter)
	if err != nil {
		return err
	}

	// Process events
	for _, event := range events {
		if err := e.processEvent(event); err != nil {
			fmt.Printf("Error processing replaceable event: %v\n", err)
		}
	}

	return nil
}

// getActiveRelays returns the list of active OUTBOX relays to sync authors' posts from
func (e *Engine) getActiveRelays(authors []string) []string {
	relaySet := make(map[string]bool)

	for _, author := range authors {
		// Get author's OUTBOX (write relays) where they publish content
		relays, err := e.discovery.GetOutboxRelays(e.ctx, author)
		if err != nil {
			continue
		}

		for _, relay := range relays {
			relaySet[relay] = true
		}
	}

	// Convert set to slice
	relays := make([]string, 0, len(relaySet))
	for relay := range relaySet {
		relays = append(relays, relay)
	}

	// Fallback to seed relays if no relays discovered
	if len(relays) == 0 {
		fmt.Printf("[SYNC] No relay hints found, falling back to seed relays\n")
		relays = e.nostrClient.GetSeedRelays()
	} else {
		// Also include seed relays as backup
		fmt.Printf("[SYNC] Adding seed relays as backup to discovered relays\n")
		seedRelays := e.nostrClient.GetSeedRelays()
		for _, seed := range seedRelays {
			if !relaySet[seed] {
				relays = append(relays, seed)
			}
		}
	}

	return relays
}

// Tier 2: Async aggregate queueing methods (non-blocking)
func (e *Engine) queueReactionUpdate(event *nostr.Event) {
	// Find the event being reacted to
	var targetEventID string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			targetEventID = tag[1]
			break
		}
	}

	if targetEventID == "" {
		return // No target event
	}

	// Reaction content is the emoji
	reaction := event.Content
	if reaction == "" {
		reaction = "+" // Default like
	}

	// Queue update (non-blocking)
	select {
	case e.aggregateChan <- &AggregateUpdate{
		Type:          "reaction",
		EventID:       targetEventID,
		Reaction:      reaction,
		InteractionAt: int64(event.CreatedAt),
	}:
	default:
		// Channel full, log and drop (graceful degradation)
		fmt.Printf("[SYNC] ⚠ Aggregate queue full, dropped reaction update\n")
	}
}

func (e *Engine) queueReplyUpdate(event *nostr.Event) {
	// Check if this is a reply (has e tags)
	var targetEventID string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			targetEventID = tag[1]
			break
		}
	}

	if targetEventID == "" {
		return // Not a reply
	}

	// Queue update (non-blocking)
	select {
	case e.aggregateChan <- &AggregateUpdate{
		Type:          "reply",
		EventID:       targetEventID,
		InteractionAt: int64(event.CreatedAt),
	}:
	default:
		fmt.Printf("[SYNC] ⚠ Aggregate queue full, dropped reply update\n")
	}
}

func (e *Engine) queueZapUpdate(event *nostr.Event) {
	// Parse zap amount from bolt11 invoice
	// This is simplified - real implementation needs to parse the invoice
	var targetEventID string
	var amount int64 = 1000 // Placeholder

	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			targetEventID = tag[1]
			break
		}
	}

	if targetEventID == "" {
		return
	}

	// Queue update (non-blocking)
	select {
	case e.aggregateChan <- &AggregateUpdate{
		Type:          "zap",
		EventID:       targetEventID,
		Sats:          amount,
		InteractionAt: int64(event.CreatedAt),
	}:
	default:
		fmt.Printf("[SYNC] ⚠ Aggregate queue full, dropped zap update\n")
	}
}

// processAggregates processes aggregate updates in batches (Tier 2 optimization)
func (e *Engine) processAggregates() {
	defer e.wg.Done()

	// Batch aggregates every 200ms for efficiency
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	replies := make(map[string]int64)
	reactions := make(map[string]map[string]int64)
	zaps := make(map[string]struct {
		Sats          int64
		InteractionAt int64
	})

	flush := func() {
		// Process batched replies
		if len(replies) > 0 {
			if err := e.storage.BatchIncrementReplies(e.ctx, replies); err != nil {
				fmt.Printf("[SYNC] ⚠ Failed to batch update replies: %v\n", err)
			}
			replies = make(map[string]int64)
		}

		// Process batched reactions
		if len(reactions) > 0 {
			if err := e.storage.BatchIncrementReactions(e.ctx, reactions); err != nil {
				fmt.Printf("[SYNC] ⚠ Failed to batch update reactions: %v\n", err)
			}
			reactions = make(map[string]map[string]int64)
		}

		// Process batched zaps
		if len(zaps) > 0 {
			if err := e.storage.BatchAddZaps(e.ctx, zaps); err != nil {
				fmt.Printf("[SYNC] ⚠ Failed to batch update zaps: %v\n", err)
			}
			zaps = make(map[string]struct {
				Sats          int64
				InteractionAt int64
			})
		}
	}

	for {
		select {
		case <-e.ctx.Done():
			flush() // Final flush before exit
			return

		case update, ok := <-e.aggregateChan:
			if !ok {
				flush() // Channel closed, final flush
				return
			}

			// Accumulate updates by type
			switch update.Type {
			case "reply":
				replies[update.EventID] = update.InteractionAt

			case "reaction":
				if reactions[update.EventID] == nil {
					reactions[update.EventID] = make(map[string]int64)
				}
				reactions[update.EventID][update.Reaction] = update.InteractionAt

			case "zap":
				zaps[update.EventID] = struct {
					Sats          int64
					InteractionAt int64
				}{Sats: update.Sats, InteractionAt: update.InteractionAt}
			}

		case <-ticker.C:
			// Periodic flush every 200ms
			flush()
		}
	}
}

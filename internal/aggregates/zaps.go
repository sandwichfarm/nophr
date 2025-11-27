package aggregates

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// ZapProcessor handles zap (kind 9735) event processing
type ZapProcessor struct {
	storage *storage.Storage
	config  *config.Inbox
}

// NewZapProcessor creates a new zap processor
func NewZapProcessor(st *storage.Storage, cfg *config.Inbox) *ZapProcessor {
	return &ZapProcessor{
		storage: st,
		config:  cfg,
	}
}

// ZapInfo contains parsed zap information
type ZapInfo struct {
	Amount       int64  // Amount in satoshis
	TargetEventID string // Event being zapped
	TargetPubkey string // Pubkey being zapped (if profile zap)
	Sender       string // Pubkey of sender
	Comment      string // Optional comment
}

// ProcessZap processes a kind 9735 zap receipt and updates aggregates
func (zp *ZapProcessor) ProcessZap(ctx context.Context, event *nostr.Event) error {
	if event.Kind != 9735 {
		return fmt.Errorf("expected kind 9735, got %d", event.Kind)
	}

	// Parse zap info
	info, err := zp.parseZapEvent(event)
	if err != nil {
		return fmt.Errorf("failed to parse zap: %w", err)
	}

	// Apply noise filter
	if info.Amount < int64(zp.config.NoiseFilters.MinZapSats) {
		return nil // Silently ignore small zaps
	}

	// Update aggregate if targeting an event
	if info.TargetEventID != "" {
		return zp.storage.AddZapAmount(ctx, info.TargetEventID, info.Amount, int64(event.CreatedAt))
	}

	// For profile zaps, we could track separately but for now just ignore
	return nil
}

// parseZapEvent extracts zap information from a kind 9735 event
func (zp *ZapProcessor) parseZapEvent(event *nostr.Event) (*ZapInfo, error) {
	info := &ZapInfo{}

	// Extract target event and pubkey from tags
	for _, tag := range event.Tags {
		if len(tag) < 2 {
			continue
		}

		switch tag[0] {
		case "e":
			info.TargetEventID = tag[1]
		case "p":
			info.TargetPubkey = tag[1]
		case "description":
			// The description tag contains the zap request (kind 9734)
			if len(tag) >= 2 {
				if err := zp.parseZapRequest(tag[1], info); err != nil {
					// Log but don't fail
					continue
				}
			}
		case "bolt11":
			// Parse amount from bolt11 invoice
			if len(tag) >= 2 {
				amount, err := zp.parseInvoiceAmount(tag[1])
				if err == nil {
					info.Amount = amount
				}
			}
		}
	}

	// If we couldn't parse from bolt11, try from the event itself
	if info.Amount == 0 {
		amount, err := zp.parseInvoiceAmount(event.Content)
		if err == nil {
			info.Amount = amount
		}
	}

	return info, nil
}

// parseZapRequest parses the zap request from the description tag
func (zp *ZapProcessor) parseZapRequest(descJSON string, info *ZapInfo) error {
	var zapRequest struct {
		Pubkey  string `json:"pubkey"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal([]byte(descJSON), &zapRequest); err != nil {
		return err
	}

	info.Sender = zapRequest.Pubkey
	info.Comment = zapRequest.Content

	return nil
}

// parseInvoiceAmount extracts the amount in satoshis from a bolt11 invoice
// This is a simplified parser - a full implementation would use a proper bolt11 library
func (zp *ZapProcessor) parseInvoiceAmount(invoice string) (int64, error) {
	// Look for lnbc followed by amount and multiplier
	// Format: lnbc{amount}{multiplier}...
	// Multipliers: m (milli), u (micro), n (nano), p (pico)

	re := regexp.MustCompile(`lnbc(\d+)([munp]?)`)
	matches := re.FindStringSubmatch(invoice)

	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse invoice amount")
	}

	amount, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	// Apply multiplier to get satoshis
	multiplier := ""
	if len(matches) >= 3 {
		multiplier = matches[2]
	}

	switch multiplier {
	case "m": // millibitcoin = 100,000 sats
		amount = amount * 100000
	case "u": // microbitcoin = 100 sats
		amount = amount * 100
	case "n": // nanobitcoin = 0.1 sats
		amount = amount / 10
	case "p": // picobitcoin = 0.0001 sats
		amount = amount / 10000
	default: // no multiplier = 1 bitcoin = 100,000,000 sats
		amount = amount * 100000000
	}

	return amount, nil
}

// GetZapStats returns zap statistics for an event
func (zp *ZapProcessor) GetZapStats(ctx context.Context, eventID string) (int64, error) {
	agg, err := zp.storage.GetAggregate(ctx, eventID)
	if err != nil {
		return 0, err
	}

	return agg.ZapSatsTotal, nil
}

// FormatSats formats satoshis for display
func FormatSats(sats int64) string {
	if sats == 0 {
		return "0 sats"
	}

	if sats < 1000 {
		return fmt.Sprintf("%d sats", sats)
	}

	if sats < 1000000 {
		return fmt.Sprintf("%.1fK sats", float64(sats)/1000)
	}

	return fmt.Sprintf("%.2fM sats", float64(sats)/1000000)
}

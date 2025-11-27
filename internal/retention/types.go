package retention

import (
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

// RetentionDecision represents the retention decision for an event
type RetentionDecision struct {
	EventID      string
	RuleName     string
	RulePriority int
	RetainUntil  *time.Time // nil = forever
	Protected    bool       // Cannot be deleted by caps
	Score        int        // For cap enforcement
}

// EvalContext contains all context needed for condition evaluation
type EvalContext struct {
	Event       *nostr.Event
	Storage     StorageReader
	SocialGraph SocialGraphReader
	Config      *config.Config
	OwnerPubkey string
}

// StorageReader provides read access to storage for condition evaluation
type StorageReader interface {
	// GetAggregate returns aggregates for an event
	GetAggregateByID(eventID string) (*AggregateData, error)

	// GetEventsByAuthor returns event count for an author
	CountEventsByAuthor(pubkey string) (int, error)

	// GetEventsByKind returns event count for a kind
	CountEventsByKind(kind int) (int, error)
}

// SocialGraphReader provides read access to social graph
type SocialGraphReader interface {
	// GetDistance returns social distance from owner to pubkey
	// Returns -1 if not in graph
	GetDistance(ownerPubkey, targetPubkey string) int

	// IsFollowing returns true if owner follows target
	IsFollowing(ownerPubkey, targetPubkey string) bool

	// IsMutual returns true if owner and target follow each other
	IsMutual(ownerPubkey, targetPubkey string) bool
}

// AggregateData contains interaction counts for an event
type AggregateData struct {
	ReplyCount    int
	ReactionTotal int
	ZapSatsTotal  int64
}

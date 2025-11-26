package config

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed example.yaml
var exampleConfig embed.FS

// Config represents the complete nophr configuration
type Config struct {
	Site         Site            `yaml:"site"`
	Identity     Identity        `yaml:"identity"`
	Protocols    Protocols       `yaml:"protocols"`
	Relays       Relays          `yaml:"relays"`
	Discovery    Discovery       `yaml:"discovery"`
	Sync         Sync            `yaml:"sync"`
	Inbox        Inbox           `yaml:"inbox"`
	Outbox       Outbox          `yaml:"outbox"`
	Storage      Storage         `yaml:"storage"`
	Export       ExportConfig    `yaml:"export"`
	Rendering    Rendering       `yaml:"rendering"`
	Caching      Caching         `yaml:"caching"`
	Logging      Logging         `yaml:"logging"`
	Layout       Layout          `yaml:"layout"`
	Display      Display         `yaml:"display"`
	Presentation Presentation    `yaml:"presentation"`
	Behavior     Behavior        `yaml:"behavior"`
	Sections     []SectionConfig `yaml:"sections"`
}

// Site contains site metadata
type Site struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Operator    string `yaml:"operator"`
}

// Identity contains Nostr identity information
type Identity struct {
	Npub string `yaml:"npub"` // Public key from file
	// Note: Nsec removed - not needed since Publisher (Phase 13) is not implemented
	// If Publisher is implemented in the future, add: Nsec string `yaml:"-"` and load from NOPHR_NSEC env var
}

// Protocols contains protocol server configurations
type Protocols struct {
	Gopher GopherProtocol `yaml:"gopher"`
	Gemini GeminiProtocol `yaml:"gemini"`
	Finger FingerProtocol `yaml:"finger"`
}

// GopherProtocol contains Gopher server settings
type GopherProtocol struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Bind    string `yaml:"bind"`
}

// GeminiProtocol contains Gemini server settings
type GeminiProtocol struct {
	Enabled bool      `yaml:"enabled"`
	Host    string    `yaml:"host"`
	Port    int       `yaml:"port"`
	Bind    string    `yaml:"bind"`
	TLS     GeminiTLS `yaml:"tls"`
}

// GeminiTLS contains TLS configuration for Gemini
type GeminiTLS struct {
	CertPath     string `yaml:"cert_path"`
	KeyPath      string `yaml:"key_path"`
	AutoGenerate bool   `yaml:"auto_generate"`
}

// FingerProtocol contains Finger server settings
type FingerProtocol struct {
	Enabled  bool   `yaml:"enabled"`
	Port     int    `yaml:"port"`
	Bind     string `yaml:"bind"`
	MaxUsers int    `yaml:"max_users"`
}

// Relays contains relay configuration
type Relays struct {
	Seeds  []string    `yaml:"seeds"`
	Policy RelayPolicy `yaml:"policy"`
}

// RelayPolicy contains relay connection policies
type RelayPolicy struct {
	ConnectTimeoutMs  int   `yaml:"connect_timeout_ms"`
	MaxConcurrentSubs int   `yaml:"max_concurrent_subs"`
	BackoffMs         []int `yaml:"backoff_ms"`
}

// Discovery contains relay discovery settings
type Discovery struct {
	RefreshSeconds     int  `yaml:"refresh_seconds"`
	UseOwnerHints      bool `yaml:"use_owner_hints"`
	UseAuthorHints     bool `yaml:"use_author_hints"`
	FallbackToSeeds    bool `yaml:"fallback_to_seeds"`
	MaxRelaysPerAuthor int  `yaml:"max_relays_per_author"`
}

// Sync contains synchronization settings
type Sync struct {
	Enabled     bool            `yaml:"enabled"`
	Kinds       SyncKinds       `yaml:"kinds"`
	Scope       SyncScope       `yaml:"scope"`
	Retention   Retention       `yaml:"retention"`
	Performance SyncPerformance `yaml:"performance"`
}

// SyncPerformance contains performance tuning options
type SyncPerformance struct {
	Workers       int  `yaml:"workers"`        // Number of parallel event processing workers (default: 4)
	UseNegentropy bool `yaml:"use_negentropy"` // Enable NIP-77 negentropy sync (default: true); always falls back to REQ if unsupported
}

// SyncKinds defines granular control over which event kinds to sync
type SyncKinds struct {
	Profiles    bool  `yaml:"profiles"`     // kind 0
	Notes       bool  `yaml:"notes"`        // kind 1
	ContactList bool  `yaml:"contact_list"` // kind 3
	Reposts     bool  `yaml:"reposts"`      // kind 6
	Reactions   bool  `yaml:"reactions"`    // kind 7
	Zaps        bool  `yaml:"zaps"`         // kind 9735
	Articles    bool  `yaml:"articles"`     // kind 30023
	RelayList   bool  `yaml:"relay_list"`   // kind 10002
	Allowlist   []int `yaml:"allowlist"`    // Additional kinds to sync
}

// ToIntSlice converts SyncKinds to a slice of kind integers
func (sk *SyncKinds) ToIntSlice() []int {
	var kinds []int

	if sk.Profiles {
		kinds = append(kinds, 0)
	}
	if sk.Notes {
		kinds = append(kinds, 1)
	}
	if sk.ContactList {
		kinds = append(kinds, 3)
	}
	if sk.Reposts {
		kinds = append(kinds, 6)
	}
	if sk.Reactions {
		kinds = append(kinds, 7)
	}
	if sk.Zaps {
		kinds = append(kinds, 9735)
	}
	if sk.Articles {
		kinds = append(kinds, 30023)
	}
	if sk.RelayList {
		kinds = append(kinds, 10002)
	}

	// Add allowlist kinds
	kinds = append(kinds, sk.Allowlist...)

	return kinds
}

// SyncScope defines synchronization scope
type SyncScope struct {
	Mode                  string   `yaml:"mode"` // self|following|mutual|foaf
	Depth                 int      `yaml:"depth"`
	IncludeDirectMentions bool     `yaml:"include_direct_mentions"`
	IncludeThreadsOfMine  bool     `yaml:"include_threads_of_mine"`
	MaxAuthors            int      `yaml:"max_authors"`
	AllowlistPubkeys      []string `yaml:"allowlist_pubkeys"`
	DenylistPubkeys       []string `yaml:"denylist_pubkeys"`
}

// Retention defines data retention policies
type Retention struct {
	KeepDays           int                `yaml:"keep_days"`
	PruneOnStart       bool               `yaml:"prune_on_start"`
	PruneIntervalHours int                `yaml:"prune_interval_hours"` // 0 = disabled, >0 = prune every N hours
	Advanced           *AdvancedRetention `yaml:"advanced,omitempty"`   // Phase 20: Advanced retention
}

// Inbox contains inbox aggregation settings
type Inbox struct {
	IncludeReplies   bool         `yaml:"include_replies"`
	IncludeReactions bool         `yaml:"include_reactions"`
	IncludeZaps      bool         `yaml:"include_zaps"`
	GroupByThread    bool         `yaml:"group_by_thread"`
	CollapseReposts  bool         `yaml:"collapse_reposts"`
	NoiseFilters     NoiseFilters `yaml:"noise_filters"`
}

// NoiseFilters defines filtering rules for inbox
type NoiseFilters struct {
	MinZapSats           int      `yaml:"min_zap_sats"`
	AllowedReactionChars []string `yaml:"allowed_reaction_chars"`
}

// Outbox contains outbox/publishing settings
type Outbox struct {
	Publish  PublishSettings `yaml:"publish"`
	DraftDir string          `yaml:"draft_dir"`
	AutoSign bool            `yaml:"auto_sign"`
}

// PublishSettings defines what to publish
type PublishSettings struct {
	Notes     bool `yaml:"notes"`
	Reactions bool `yaml:"reactions"`
	Zaps      bool `yaml:"zaps"`
}

// Storage contains storage backend settings
type Storage struct {
	Driver        string `yaml:"driver"` // sqlite|lmdb
	SQLitePath    string `yaml:"sqlite_path"`
	LMDBPath      string `yaml:"lmdb_path"`
	LMDBMaxSizeMB int    `yaml:"lmdb_max_size_mb"`
}

// Rendering contains protocol-specific rendering options
type Rendering struct {
	Gopher GopherRendering `yaml:"gopher"`
	Gemini GeminiRendering `yaml:"gemini"`
	Finger FingerRendering `yaml:"finger"`
}

// GopherRendering contains Gopher rendering options
type GopherRendering struct {
	MaxLineLength  int    `yaml:"max_line_length"`
	ShowTimestamps bool   `yaml:"show_timestamps"`
	DateFormat     string `yaml:"date_format"`
	ThreadIndent   string `yaml:"thread_indent"`
}

// GeminiRendering contains Gemini rendering options
type GeminiRendering struct {
	MaxLineLength  int  `yaml:"max_line_length"`
	ShowTimestamps bool `yaml:"show_timestamps"`
	Emoji          bool `yaml:"emoji"`
}

// FingerRendering contains Finger rendering options
type FingerRendering struct {
	PlanSource       string `yaml:"plan_source"`
	RecentNotesCount int    `yaml:"recent_notes_count"`
}

// Caching contains caching configuration
type Caching struct {
	Enabled    bool                   `yaml:"enabled"`
	Engine     string                 `yaml:"engine"` // memory|redis
	RedisURL   string                 `yaml:"redis_url"`
	TTL        CacheTTL               `yaml:"ttl"`
	Aggregates AggregatesCaching      `yaml:"aggregates"`
	Overrides  map[string]interface{} `yaml:"overrides,omitempty"`
}

// CacheTTL contains TTL settings for different cache types
type CacheTTL struct {
	Sections map[string]int `yaml:"sections"`
	Render   map[string]int `yaml:"render"`
}

// AggregatesCaching contains aggregate caching settings
type AggregatesCaching struct {
	Enabled                   bool `yaml:"enabled"`
	UpdateOnIngest            bool `yaml:"update_on_ingest"`
	ReconcilerIntervalSeconds int  `yaml:"reconciler_interval_seconds"`
}

// Logging contains logging configuration
type Logging struct {
	Level  string `yaml:"level"`  // debug|info|warn|error
	Format string `yaml:"format"` // text|json
}

// Layout contains layout and section definitions
type Layout struct {
	Sections map[string]interface{} `yaml:"sections,omitempty"`
	Pages    map[string]interface{} `yaml:"pages,omitempty"`
}

// Display contains display and rendering control options
type Display struct {
	Feed   FeedDisplay   `yaml:"feed"`
	Detail DetailDisplay `yaml:"detail"`
	Limits DisplayLimits `yaml:"limits"`
}

// FeedDisplay controls what appears in feed/list views
type FeedDisplay struct {
	ShowInteractions bool `yaml:"show_interactions"`
	ShowReactions    bool `yaml:"show_reactions"`
	ShowZaps         bool `yaml:"show_zaps"`
	ShowReplies      bool `yaml:"show_replies"`
}

// DetailDisplay controls what appears in individual note/detail views
type DetailDisplay struct {
	ShowInteractions bool `yaml:"show_interactions"`
	ShowReactions    bool `yaml:"show_reactions"`
	ShowZaps         bool `yaml:"show_zaps"`
	ShowReplies      bool `yaml:"show_replies"`
	ShowThread       bool `yaml:"show_thread"`
}

// DisplayLimits controls length and truncation
type DisplayLimits struct {
	SummaryLength     int    `yaml:"summary_length"`
	MaxContentLength  int    `yaml:"max_content_length"`
	MaxThreadDepth    int    `yaml:"max_thread_depth"`
	MaxRepliesInFeed  int    `yaml:"max_replies_in_feed"`
	TruncateIndicator string `yaml:"truncate_indicator"`
}

// Presentation contains visual presentation and layout options
type Presentation struct {
	Headers    Headers    `yaml:"headers"`
	Footers    Footers    `yaml:"footers"`
	Separators Separators `yaml:"separators"`
}

// Headers defines header content for pages
type Headers struct {
	Global  HeaderConfig            `yaml:"global"`
	PerPage map[string]HeaderConfig `yaml:"per_page,omitempty"`
}

// HeaderConfig defines a single header configuration
type HeaderConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Content  string `yaml:"content"`
	FilePath string `yaml:"file_path"`
}

// Footers defines footer content for pages
type Footers struct {
	Global  FooterConfig            `yaml:"global"`
	PerPage map[string]FooterConfig `yaml:"per_page,omitempty"`
}

// FooterConfig defines a single footer configuration
type FooterConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Content  string `yaml:"content"`
	FilePath string `yaml:"file_path"`
}

// Separators defines visual separators
type Separators struct {
	Item    SeparatorConfig `yaml:"item"`
	Section SeparatorConfig `yaml:"section"`
}

// SeparatorConfig defines a separator configuration
type SeparatorConfig struct {
	Gopher string `yaml:"gopher"`
	Gemini string `yaml:"gemini"`
	Finger string `yaml:"finger"`
}

// Behavior contains behavioral settings for queries and filtering
type Behavior struct {
	ContentFiltering ContentFiltering `yaml:"content_filtering"`
	SortPreferences  SortPreferences  `yaml:"sort_preferences"`
	Pagination       PaginationConfig `yaml:"pagination"`
}

// ExportConfig configures static exports
type ExportConfig struct {
	Gopher GopherExportConfig `yaml:"gopher"`
	Gemini GeminiExportConfig `yaml:"gemini"`
}

// GopherExportConfig configures static gopher generation
type GopherExportConfig struct {
	Enabled   bool   `yaml:"enabled"`
	OutputDir string `yaml:"output_dir"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	MaxItems  int    `yaml:"max_items"`
}

// GeminiExportConfig configures static gemini generation
type GeminiExportConfig struct {
	Enabled   bool   `yaml:"enabled"`
	OutputDir string `yaml:"output_dir"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	MaxItems  int    `yaml:"max_items"`
}

// ContentFiltering defines content filtering rules
type ContentFiltering struct {
	Enabled             bool     `yaml:"enabled"`
	MinReactions        int      `yaml:"min_reactions"`
	MinZapSats          int      `yaml:"min_zap_sats"`
	MinEngagement       int      `yaml:"min_engagement"` // Combined score
	HideNoInteractions  bool     `yaml:"hide_no_interactions"`
	AllowedContentTypes []string `yaml:"allowed_content_types"`
}

// SortPreferences defines sorting options
type SortPreferences struct {
	Notes    string `yaml:"notes"` // chronological|engagement|zaps|reactions
	Articles string `yaml:"articles"`
	Replies  string `yaml:"replies"`
	Mentions string `yaml:"mentions"`
}

// PaginationConfig defines pagination settings
type PaginationConfig struct {
	Enabled      bool `yaml:"enabled"`
	ItemsPerPage int  `yaml:"items_per_page"`
	MaxPages     int  `yaml:"max_pages"`
}

// applyDefaults fills in missing configuration fields with sensible defaults
func applyDefaults(cfg *Config) {
	defaults := Default()

	// Apply Display defaults if missing
	if cfg.Display.Limits.SummaryLength == 0 {
		cfg.Display.Limits.SummaryLength = defaults.Display.Limits.SummaryLength
	}
	if cfg.Display.Limits.MaxContentLength == 0 {
		cfg.Display.Limits.MaxContentLength = defaults.Display.Limits.MaxContentLength
	}
	if cfg.Display.Limits.MaxThreadDepth == 0 {
		cfg.Display.Limits.MaxThreadDepth = defaults.Display.Limits.MaxThreadDepth
	}
	if cfg.Display.Limits.MaxRepliesInFeed == 0 {
		cfg.Display.Limits.MaxRepliesInFeed = defaults.Display.Limits.MaxRepliesInFeed
	}
	if cfg.Display.Limits.TruncateIndicator == "" {
		cfg.Display.Limits.TruncateIndicator = defaults.Display.Limits.TruncateIndicator
	}

	// Apply Behavior defaults for sort preferences
	if cfg.Behavior.SortPreferences.Notes == "" {
		cfg.Behavior.SortPreferences.Notes = defaults.Behavior.SortPreferences.Notes
	}
	if cfg.Behavior.SortPreferences.Articles == "" {
		cfg.Behavior.SortPreferences.Articles = defaults.Behavior.SortPreferences.Articles
	}
	if cfg.Behavior.SortPreferences.Replies == "" {
		cfg.Behavior.SortPreferences.Replies = defaults.Behavior.SortPreferences.Replies
	}
	if cfg.Behavior.SortPreferences.Mentions == "" {
		cfg.Behavior.SortPreferences.Mentions = defaults.Behavior.SortPreferences.Mentions
	}

	// Apply Presentation defaults for separators if empty maps
	if cfg.Presentation.Headers.PerPage == nil {
		cfg.Presentation.Headers.PerPage = make(map[string]HeaderConfig)
	}
	if cfg.Presentation.Footers.PerPage == nil {
		cfg.Presentation.Footers.PerPage = make(map[string]FooterConfig)
	}

	// Apply Layout defaults if empty
	if cfg.Layout.Sections == nil {
		cfg.Layout.Sections = make(map[string]interface{})
	}
	if cfg.Layout.Pages == nil {
		cfg.Layout.Pages = make(map[string]interface{})
	}

	// Apply Export defaults
	if cfg.Export.Gopher.OutputDir == "" {
		cfg.Export.Gopher.OutputDir = defaults.Export.Gopher.OutputDir
	}
	if cfg.Export.Gopher.Host == "" {
		host := cfg.Protocols.Gopher.Host
		if host == "" {
			host = defaults.Export.Gopher.Host
		}
		cfg.Export.Gopher.Host = host
	}
	if cfg.Export.Gopher.Port == 0 {
		port := cfg.Protocols.Gopher.Port
		if port == 0 {
			port = defaults.Export.Gopher.Port
		}
		cfg.Export.Gopher.Port = port
	}
	if cfg.Export.Gopher.MaxItems == 0 {
		cfg.Export.Gopher.MaxItems = defaults.Export.Gopher.MaxItems
	}

	// Apply Export.Gemini defaults
	if cfg.Export.Gemini.OutputDir == "" {
		cfg.Export.Gemini.OutputDir = defaults.Export.Gemini.OutputDir
	}
	if cfg.Export.Gemini.Host == "" {
		host := cfg.Protocols.Gemini.Host
		if host == "" {
			host = defaults.Export.Gemini.Host
		}
		cfg.Export.Gemini.Host = host
	}
	if cfg.Export.Gemini.Port == 0 {
		port := cfg.Protocols.Gemini.Port
		if port == 0 {
			port = defaults.Export.Gemini.Port
		}
		cfg.Export.Gemini.Port = port
	}
	if cfg.Export.Gemini.MaxItems == 0 {
		cfg.Export.Gemini.MaxItems = defaults.Export.Gemini.MaxItems
	}

	// Apply Sync performance defaults
	if cfg.Sync.Performance.Workers == 0 {
		cfg.Sync.Performance.Workers = defaults.Sync.Performance.Workers
	}
}

// Load reads and parses a configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing fields
	applyDefaults(&cfg)

	// Apply environment variable overrides
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) error {
	// Note: NOPHR_NSEC removed - not needed since Publisher (Phase 13) is not implemented
	// If Publisher is implemented in the future, uncomment:
	// if nsec := os.Getenv("NOPHR_NSEC"); nsec != "" {
	//     cfg.Identity.Nsec = nsec
	// }

	// Redis URL from env if using redis
	if redisURL := os.Getenv("NOPHR_REDIS_URL"); redisURL != "" {
		cfg.Caching.RedisURL = redisURL
	}

	// Allow overriding any config via NOPHR_ prefix
	// This is a simplified implementation - full version would use reflection
	// to handle all nested fields automatically

	return nil
}

// GetExampleConfig returns the embedded example configuration
func GetExampleConfig() ([]byte, error) {
	return exampleConfig.ReadFile("example.yaml")
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		Site: Site{
			Title:       "My Nostr Site",
			Description: "Personal Nostr gateway",
			Operator:    "Anonymous",
		},
		Identity: Identity{
			Npub: "",
		},
		Protocols: Protocols{
			Gopher: GopherProtocol{
				Enabled: true,
				Host:    "localhost",
				Port:    70,
				Bind:    "0.0.0.0",
			},
			Gemini: GeminiProtocol{
				Enabled: true,
				Host:    "localhost",
				Port:    1965,
				Bind:    "0.0.0.0",
				TLS: GeminiTLS{
					CertPath:     "./certs/cert.pem",
					KeyPath:      "./certs/key.pem",
					AutoGenerate: true,
				},
			},
			Finger: FingerProtocol{
				Enabled:  true,
				Port:     79,
				Bind:     "0.0.0.0",
				MaxUsers: 100,
			},
		},
		Relays: Relays{
			Seeds: []string{
				"wss://relay.damus.io",
				"wss://relay.nostr.band",
				"wss://nos.lol",
			},
			Policy: RelayPolicy{
				ConnectTimeoutMs:  5000,
				MaxConcurrentSubs: 8,
				BackoffMs:         []int{500, 1500, 5000},
			},
		},
		Discovery: Discovery{
			RefreshSeconds:     900,
			UseOwnerHints:      true,
			UseAuthorHints:     true,
			FallbackToSeeds:    true,
			MaxRelaysPerAuthor: 8,
		},
		Sync: Sync{
			Kinds: SyncKinds{
				Profiles:    true,
				Notes:       true,
				ContactList: true,
				Reposts:     true,
				Reactions:   true,
				Zaps:        true,
				Articles:    true,
				RelayList:   true,
				Allowlist:   []int{},
			},
			Scope: SyncScope{
				Mode:                  "foaf",
				Depth:                 2,
				IncludeDirectMentions: true,
				IncludeThreadsOfMine:  true,
				MaxAuthors:            5000,
				AllowlistPubkeys:      []string{},
				DenylistPubkeys:       []string{},
			},
			Retention: Retention{
				KeepDays:     365,
				PruneOnStart: true,
			},
			Performance: SyncPerformance{
				Workers:       4,    // Default: 4 parallel event processing workers
				UseNegentropy: true, // Default: enable NIP-77 negentropy (always falls back to REQ if unsupported)
			},
		},
		Inbox: Inbox{
			IncludeReplies:   true,
			IncludeReactions: true,
			IncludeZaps:      true,
			GroupByThread:    true,
			CollapseReposts:  true,
			NoiseFilters: NoiseFilters{
				MinZapSats:           1,
				AllowedReactionChars: []string{"+"},
			},
		},
		Outbox: Outbox{
			Publish: PublishSettings{
				Notes:     true,
				Reactions: false,
				Zaps:      false,
			},
			DraftDir: "./content",
			AutoSign: false,
		},
		Storage: Storage{
			Driver:        "sqlite",
			SQLitePath:    "./data/nophr.db",
			LMDBPath:      "./data/nophr.lmdb",
			LMDBMaxSizeMB: 10240,
		},
		Export: ExportConfig{
			Gopher: GopherExportConfig{
				Enabled:   false,
				OutputDir: "./export/gopher",
				Host:      "localhost",
				Port:      70,
				MaxItems:  200,
			},
			Gemini: GeminiExportConfig{
				Enabled:   false,
				OutputDir: "./export/gemini",
				Host:      "localhost",
				Port:      1965,
				MaxItems:  200,
			},
		},
		Rendering: Rendering{
			Gopher: GopherRendering{
				MaxLineLength:  70,
				ShowTimestamps: true,
				DateFormat:     "2006-01-02 15:04 MST",
				ThreadIndent:   "  ",
			},
			Gemini: GeminiRendering{
				MaxLineLength:  80,
				ShowTimestamps: true,
				Emoji:          true,
			},
			Finger: FingerRendering{
				PlanSource:       "kind_0",
				RecentNotesCount: 5,
			},
		},
		Caching: Caching{
			Enabled:  true,
			Engine:   "memory",
			RedisURL: "",
			TTL: CacheTTL{
				Sections: map[string]int{
					"notes":        60,
					"comments":     30,
					"articles":     300,
					"interactions": 10,
				},
				Render: map[string]int{
					"gopher_menu":     300,
					"gemini_page":     300,
					"finger_response": 60,
					"kind_1":          86400,
					"kind_30023":      604800,
					"kind_0":          3600,
					"kind_3":          600,
				},
			},
			Aggregates: AggregatesCaching{
				Enabled:                   true,
				UpdateOnIngest:            true,
				ReconcilerIntervalSeconds: 900,
			},
		},
		Logging: Logging{
			Level:  "info",
			Format: "text",
		},
		Layout: Layout{
			Sections: make(map[string]interface{}),
			Pages:    make(map[string]interface{}),
		},
		Display: Display{
			Feed: FeedDisplay{
				ShowInteractions: true,
				ShowReactions:    true,
				ShowZaps:         true,
				ShowReplies:      true,
			},
			Detail: DetailDisplay{
				ShowInteractions: true,
				ShowReactions:    true,
				ShowZaps:         true,
				ShowReplies:      true,
				ShowThread:       true,
			},
			Limits: DisplayLimits{
				SummaryLength:     100,
				MaxContentLength:  5000,
				MaxThreadDepth:    10,
				MaxRepliesInFeed:  3,
				TruncateIndicator: "...",
			},
		},
		Presentation: Presentation{
			Headers: Headers{
				Global: HeaderConfig{
					Enabled:  false,
					Content:  "",
					FilePath: "",
				},
				PerPage: make(map[string]HeaderConfig),
			},
			Footers: Footers{
				Global: FooterConfig{
					Enabled:  false,
					Content:  "",
					FilePath: "",
				},
				PerPage: make(map[string]FooterConfig),
			},
			Separators: Separators{
				Item: SeparatorConfig{
					Gopher: "",
					Gemini: "",
					Finger: "",
				},
				Section: SeparatorConfig{
					Gopher: "---",
					Gemini: "---",
					Finger: "---",
				},
			},
		},
		Behavior: Behavior{
			ContentFiltering: ContentFiltering{
				Enabled:             false,
				MinReactions:        0,
				MinZapSats:          0,
				MinEngagement:       0,
				HideNoInteractions:  false,
				AllowedContentTypes: []string{},
			},
			SortPreferences: SortPreferences{
				Notes:    "chronological",
				Articles: "chronological",
				Replies:  "chronological",
				Mentions: "chronological",
			},
			Pagination: PaginationConfig{
				Enabled:      false,
				ItemsPerPage: 50,
				MaxPages:     10,
			},
		},
	}
}

// validLogLevels defines allowed log levels
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// validSyncModes defines allowed sync modes
var validSyncModes = map[string]bool{
	"self":      true,
	"following": true,
	"mutual":    true,
	"foaf":      true,
}

// validStorageDrivers defines allowed storage drivers
var validStorageDrivers = map[string]bool{
	"sqlite": true,
	"lmdb":   true,
}

// validCacheEngines defines allowed cache engines
var validCacheEngines = map[string]bool{
	"memory": true,
	"redis":  true,
}

// Validate checks if a configuration is valid
func Validate(cfg *Config) error {
	// Validate identity
	if cfg.Identity.Npub == "" {
		return fmt.Errorf("identity.npub is required")
	}
	if !strings.HasPrefix(cfg.Identity.Npub, "npub1") {
		return fmt.Errorf("identity.npub must start with 'npub1'")
	}

	// Validate at least one protocol is enabled
	if !cfg.Protocols.Gopher.Enabled && !cfg.Protocols.Gemini.Enabled && !cfg.Protocols.Finger.Enabled {
		return fmt.Errorf("at least one protocol must be enabled")
	}

	// Validate ports
	if cfg.Protocols.Gopher.Enabled && (cfg.Protocols.Gopher.Port < 1 || cfg.Protocols.Gopher.Port > 65535) {
		return fmt.Errorf("gopher port must be between 1 and 65535")
	}
	if cfg.Protocols.Gemini.Enabled && (cfg.Protocols.Gemini.Port < 1 || cfg.Protocols.Gemini.Port > 65535) {
		return fmt.Errorf("gemini port must be between 1 and 65535")
	}
	if cfg.Protocols.Finger.Enabled && (cfg.Protocols.Finger.Port < 1 || cfg.Protocols.Finger.Port > 65535) {
		return fmt.Errorf("finger port must be between 1 and 65535")
	}

	// Validate relay seeds
	if len(cfg.Relays.Seeds) == 0 {
		return fmt.Errorf("at least one relay seed is required")
	}
	for _, seed := range cfg.Relays.Seeds {
		if !strings.HasPrefix(seed, "wss://") && !strings.HasPrefix(seed, "ws://") {
			return fmt.Errorf("relay seed must start with ws:// or wss://: %s", seed)
		}
	}

	// Validate sync mode
	if !validSyncModes[cfg.Sync.Scope.Mode] {
		return fmt.Errorf("invalid sync mode: %s (must be one of: self, following, mutual, foaf)", cfg.Sync.Scope.Mode)
	}

	// Validate storage driver
	if !validStorageDrivers[cfg.Storage.Driver] {
		return fmt.Errorf("invalid storage driver: %s (must be one of: sqlite, lmdb)", cfg.Storage.Driver)
	}

	// Validate cache engine
	if cfg.Caching.Enabled && !validCacheEngines[cfg.Caching.Engine] {
		return fmt.Errorf("invalid cache engine: %s (must be one of: memory, redis)", cfg.Caching.Engine)
	}

	// Validate log level
	if !validLogLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be one of: debug, info, warn, error)", cfg.Logging.Level)
	}

	// Validate display limits
	if cfg.Display.Limits.SummaryLength < 10 || cfg.Display.Limits.SummaryLength > 1000 {
		return fmt.Errorf("display.limits.summary_length must be between 10 and 1000")
	}
	if cfg.Display.Limits.MaxContentLength < 100 || cfg.Display.Limits.MaxContentLength > 100000 {
		return fmt.Errorf("display.limits.max_content_length must be between 100 and 100000")
	}
	if cfg.Display.Limits.MaxThreadDepth < 1 || cfg.Display.Limits.MaxThreadDepth > 100 {
		return fmt.Errorf("display.limits.max_thread_depth must be between 1 and 100")
	}

	// Validate sort preferences
	validSortModes := map[string]bool{
		"chronological": true,
		"engagement":    true,
		"zaps":          true,
		"reactions":     true,
	}
	if !validSortModes[cfg.Behavior.SortPreferences.Notes] {
		return fmt.Errorf("invalid sort mode for notes: %s", cfg.Behavior.SortPreferences.Notes)
	}
	if !validSortModes[cfg.Behavior.SortPreferences.Articles] {
		return fmt.Errorf("invalid sort mode for articles: %s", cfg.Behavior.SortPreferences.Articles)
	}
	if !validSortModes[cfg.Behavior.SortPreferences.Replies] {
		return fmt.Errorf("invalid sort mode for replies: %s", cfg.Behavior.SortPreferences.Replies)
	}
	if !validSortModes[cfg.Behavior.SortPreferences.Mentions] {
		return fmt.Errorf("invalid sort mode for mentions: %s", cfg.Behavior.SortPreferences.Mentions)
	}

	// Validate pagination
	if cfg.Behavior.Pagination.Enabled {
		if cfg.Behavior.Pagination.ItemsPerPage < 1 || cfg.Behavior.Pagination.ItemsPerPage > 500 {
			return fmt.Errorf("behavior.pagination.items_per_page must be between 1 and 500")
		}
		if cfg.Behavior.Pagination.MaxPages < 1 || cfg.Behavior.Pagination.MaxPages > 100 {
			return fmt.Errorf("behavior.pagination.max_pages must be between 1 and 100")
		}
	}

	// Validate gopher export
	if cfg.Export.Gopher.Enabled {
		if cfg.Export.Gopher.OutputDir == "" {
			return fmt.Errorf("export.gopher.output_dir is required when export.gopher.enabled is true")
		}
		if cfg.Export.Gopher.Host == "" {
			return fmt.Errorf("export.gopher.host is required when export.gopher.enabled is true")
		}
		if cfg.Export.Gopher.Port < 1 || cfg.Export.Gopher.Port > 65535 {
			return fmt.Errorf("export.gopher.port must be between 1 and 65535")
		}
		if cfg.Export.Gopher.MaxItems < 1 || cfg.Export.Gopher.MaxItems > 5000 {
			return fmt.Errorf("export.gopher.max_items must be between 1 and 5000")
		}
	}

	// Validate gemini export
	if cfg.Export.Gemini.Enabled {
		if cfg.Export.Gemini.OutputDir == "" {
			return fmt.Errorf("export.gemini.output_dir is required when export.gemini.enabled is true")
		}
		if cfg.Export.Gemini.Host == "" {
			return fmt.Errorf("export.gemini.host is required when export.gemini.enabled is true")
		}
		if cfg.Export.Gemini.Port < 1 || cfg.Export.Gemini.Port > 65535 {
			return fmt.Errorf("export.gemini.port must be between 1 and 65535")
		}
		if cfg.Export.Gemini.MaxItems < 1 || cfg.Export.Gemini.MaxItems > 5000 {
			return fmt.Errorf("export.gemini.max_items must be between 1 and 5000")
		}
	}

	// Validate advanced retention (Phase 20)
	if cfg.Sync.Retention.Advanced != nil {
		if err := cfg.Sync.Retention.Advanced.Validate(); err != nil {
			return fmt.Errorf("advanced retention validation failed: %w", err)
		}
	}

	return nil
}

// SectionConfig represents a section definition in YAML
type SectionConfig struct {
	Name        string                 `yaml:"name"`
	Path        string                 `yaml:"path"`
	Title       string                 `yaml:"title"`
	Description string                 `yaml:"description"`
	Filters     SectionFilterConfig    `yaml:"filters"`
	SortBy      string                 `yaml:"sort_by"`
	SortOrder   string                 `yaml:"sort_order"`
	Limit       int                    `yaml:"limit"`
	ShowDates   bool                   `yaml:"show_dates"`
	ShowAuthors bool                   `yaml:"show_authors"`
	GroupBy     string                 `yaml:"group_by"`
	MoreLink    *SectionMoreLinkConfig `yaml:"more_link"`
	Order       int                    `yaml:"order"`
}

// SectionFilterConfig represents section filters in YAML
type SectionFilterConfig struct {
	Kinds   []int               `yaml:"kinds"`
	Authors []string            `yaml:"authors"`
	Tags    map[string][]string `yaml:"tags"`
	Since   string              `yaml:"since"` // RFC3339 or duration like "-24h"
	Until   string              `yaml:"until"` // RFC3339 or duration
	Search  string              `yaml:"search"`
	Scope   string              `yaml:"scope"` // self, following, mutual, foaf, all
	IsReply *bool               `yaml:"is_reply"` // true = only replies, false = only roots
}

// SectionMoreLinkConfig represents a "more" link configuration
type SectionMoreLinkConfig struct {
	Text       string `yaml:"text"`
	SectionRef string `yaml:"section_ref"`
}

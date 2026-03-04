// Package config reads environment variables into [RawConfig] and builds the
// validated [BuiltConfig] value used by the rest of the updater.
package config

import (
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// RawConfig holds parsed updater settings before cross-field validation and
// runtime-specific derivation.
//
// It intentionally excludes two bootstrap concerns that are handled elsewhere:
// - [SetupPP] reads output-formatting controls such as EMOJI and QUIET.
// - [SetupReporters] reads and constructs heartbeat/notifier services.
type RawConfig struct {
	Auth                            api.Auth
	Provider                        map[ipnet.Type]provider.Provider
	Domains                         []domain.Domain
	IP4Domains                      []domain.Domain
	IP6Domains                      []domain.Domain
	WAFLists                        []api.WAFList
	UpdateCron                      cron.Schedule
	UpdateOnStart                   bool
	DeleteOnStop                    bool
	TTL                             api.TTL
	ProxiedExpression               string
	RecordComment                   string
	ManagedRecordsCommentRegex      string
	WAFListDescription              string
	WAFListItemComment              string
	ManagedWAFListItemsCommentRegex string
	CacheExpiration                 time.Duration
	DetectionTimeout                time.Duration
	UpdateTimeout                   time.Duration
}

// BuiltConfig groups the validated updater runtime config slices.
// Reporter services are kept separate and are not part of this structure.
type BuiltConfig struct {
	Handle    *HandleConfig
	Lifecycle *LifecycleConfig
	Update    *UpdateConfig
}

// HandleConfig holds the validated settings needed to construct an API handle.
//
// The managed-record selector lives here because the current API-handle cache
// contract assumes one stable ownership scope per handle instance.
type HandleConfig struct {
	Auth    api.Auth
	Options api.HandleOptions
}

// LifecycleConfig holds validated process-lifecycle settings such as scheduling
// and shutdown behavior.
// (The timezone is handled directly by the standard library reading the TZ environment variable.)
type LifecycleConfig struct {
	UpdateCron    cron.Schedule
	UpdateOnStart bool
	DeleteOnStop  bool
}

// UpdateConfig holds the validated settings used during IP detection and
// DNS/WAF reconciliation.
type UpdateConfig struct {
	Provider           map[ipnet.Type]provider.Provider
	Domains            map[ipnet.Type][]domain.Domain
	WAFLists           []api.WAFList
	TTL                api.TTL
	Proxied            map[domain.Domain]bool
	RecordComment      string
	WAFListDescription string
	WAFListItemComment string
	DetectionTimeout   time.Duration
	UpdateTimeout      time.Duration
}

// DefaultRaw gives the default raw updater configuration used before reading
// environment variables.
func DefaultRaw() *RawConfig {
	return &RawConfig{
		Auth: nil,
		Provider: map[ipnet.Type]provider.Provider{
			ipnet.IP4: provider.NewCloudflareTrace(),
			ipnet.IP6: provider.NewCloudflareTrace(),
		},
		Domains:                         nil,
		IP4Domains:                      nil,
		IP6Domains:                      nil,
		WAFLists:                        nil,
		UpdateCron:                      cron.MustNew("@every 5m"),
		UpdateOnStart:                   true,
		DeleteOnStop:                    false,
		TTL:                             api.TTLAuto,
		ProxiedExpression:               "false",
		RecordComment:                   "",
		ManagedRecordsCommentRegex:      "",
		WAFListDescription:              "",
		WAFListItemComment:              "",
		ManagedWAFListItemsCommentRegex: "",
		CacheExpiration:                 time.Hour * 6,
		DetectionTimeout:                time.Second * 5,
		UpdateTimeout:                   time.Second * 30,
	}
}

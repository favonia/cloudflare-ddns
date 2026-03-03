// Package config reads environment variables into [RawConfig] and builds the
// validated [HandleConfig], [LifecycleConfig], and [UpdateConfig] values used
// by the rest of the updater.
package config

import (
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// RawConfig holds parsed environment values before cross-field validation and
// runtime-specific derivation.
type RawConfig struct {
	Auth                       api.Auth
	Provider                   map[ipnet.Type]provider.Provider
	Domains                    []domain.Domain
	IP4Domains                 []domain.Domain
	IP6Domains                 []domain.Domain
	WAFLists                   []api.WAFList
	UpdateCron                 cron.Schedule
	UpdateOnStart              bool
	DeleteOnStop               bool
	TTL                        api.TTL
	ProxiedExpression          string
	RecordComment              string
	ManagedRecordsCommentRegex string
	WAFListDescription         string
	CacheExpiration            time.Duration
	DetectionTimeout           time.Duration
	UpdateTimeout              time.Duration
	Monitor                    monitor.Monitor
	Notifier                   notifier.Notifier
}

// HandleConfig holds the validated settings needed to construct an API handle.
//
// The managed-record selector lives here because the current API-handle cache
// contract assumes one stable ownership scope per handle instance.
type HandleConfig struct {
	Auth    api.Auth
	Options api.HandleOptions
}

// LifecycleConfig holds validated process-lifecycle settings such as scheduling,
// shutdown behavior, and external reporting.
// (The timezone is handled directly by the standard library reading the TZ environment variable.)
type LifecycleConfig struct {
	UpdateCron    cron.Schedule
	UpdateOnStart bool
	DeleteOnStop  bool
	Monitor       monitor.Monitor
	Notifier      notifier.Notifier
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
	DetectionTimeout   time.Duration
	UpdateTimeout      time.Duration
}

// DefaultRaw gives the default raw configuration used before reading
// environment variables.
func DefaultRaw() *RawConfig {
	return &RawConfig{
		Auth: nil,
		Provider: map[ipnet.Type]provider.Provider{
			ipnet.IP4: provider.NewCloudflareTrace(),
			ipnet.IP6: provider.NewCloudflareTrace(),
		},
		Domains:                    nil,
		IP4Domains:                 nil,
		IP6Domains:                 nil,
		WAFLists:                   nil,
		UpdateCron:                 cron.MustNew("@every 5m"),
		UpdateOnStart:              true,
		DeleteOnStop:               false,
		TTL:                        api.TTLAuto,
		ProxiedExpression:          "false",
		RecordComment:              "",
		ManagedRecordsCommentRegex: "",
		WAFListDescription:         "",
		CacheExpiration:            time.Hour * 6,
		DetectionTimeout:           time.Second * 5,
		UpdateTimeout:              time.Second * 30,
		Monitor:                    monitor.NewComposed(),
		Notifier:                   notifier.NewComposed(),
	}
}

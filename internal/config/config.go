// Package config reads and parses configurations.
package config

import (
	"regexp"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// Config holds the configuration of the updater except for the timezone.
// (The timezone is handled directly by the standard library reading the TZ environment variable.)
type Config struct {
	Auth                               api.Auth
	Provider                           map[ipnet.Type]provider.Provider
	Domains                            map[ipnet.Type][]domain.Domain
	WAFLists                           []api.WAFList
	UpdateCron                         cron.Schedule
	UpdateOnStart                      bool
	DeleteOnStop                       bool
	CacheExpiration                    time.Duration
	TTL                                api.TTL
	ProxiedTemplate                    string
	Proxied                            map[domain.Domain]bool
	RecordComment                      string
	ManagedRecordsCommentRegexTemplate string
	// ManagedRecordsCommentRegex may be nil before [Config.Normalize].
	// After a successful [Config.Normalize], it is guaranteed to be non-nil
	// (including the default empty template, which compiles to a match-all regex).
	ManagedRecordsCommentRegex *regexp.Regexp
	WAFListDescription         string
	DetectionTimeout           time.Duration
	UpdateTimeout              time.Duration
	Monitor                    monitor.Monitor
	Notifier                   notifier.Notifier
}

// Default gives the default configuration.
func Default() *Config {
	return &Config{
		Auth: nil,
		Provider: map[ipnet.Type]provider.Provider{
			ipnet.IP4: provider.NewCloudflareTrace(),
			ipnet.IP6: provider.NewCloudflareTrace(),
		},
		Domains: map[ipnet.Type][]domain.Domain{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		WAFLists:                           nil,
		UpdateCron:                         cron.MustNew("@every 5m"),
		UpdateOnStart:                      true,
		DeleteOnStop:                       false,
		CacheExpiration:                    time.Hour * 6,
		TTL:                                api.TTLAuto,
		ProxiedTemplate:                    "false",
		Proxied:                            map[domain.Domain]bool{},
		RecordComment:                      "",
		ManagedRecordsCommentRegexTemplate: "",
		ManagedRecordsCommentRegex:         nil,
		WAFListDescription:                 "",
		DetectionTimeout:                   time.Second * 5,
		UpdateTimeout:                      time.Second * 30,
		Monitor:                            monitor.NewComposed(),
		Notifier:                           notifier.NewComposed(),
	}
}

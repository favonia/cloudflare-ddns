// Package config reads and parses configurations.
package config

import (
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// Config holds the configuration of the updater except for the timezone.
// (The timezone is handled directly by the standard library reading the TZ environment variable.)
type Config struct {
	Auth             api.Auth
	Provider         map[ipnet.Type]provider.Provider
	Use1001          bool
	Domains          map[ipnet.Type][]domain.Domain
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              api.TTL
	ProxiedTemplate  string
	Proxied          map[domain.Domain]bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
	Monitor          monitor.Monitor
}

// Default gives the default configuration.
func Default() *Config {
	return &Config{
		Auth: nil,
		Provider: map[ipnet.Type]provider.Provider{
			ipnet.IP4: provider.NewCloudflareTrace(),
			ipnet.IP6: provider.NewCloudflareTrace(),
		},
		Use1001: false,
		Domains: map[ipnet.Type][]domain.Domain{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		UpdateCron:       cron.MustNew("@every 5m"),
		UpdateOnStart:    true,
		DeleteOnStop:     false,
		CacheExpiration:  time.Hour * 6, //nolint:gomnd
		TTL:              api.TTLAuto,
		ProxiedTemplate:  "false",
		Proxied:          map[domain.Domain]bool{},
		UpdateTimeout:    time.Second * 30, //nolint:gomnd
		DetectionTimeout: time.Second * 5,  //nolint:gomnd
		Monitor:          monitor.Monitors{},
	}
}

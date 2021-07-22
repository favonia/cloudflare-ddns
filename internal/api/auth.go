package api

import (
	"log"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/patrickmn/go-cache"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

type Auth interface {
	New(time.Duration) (*Handle, bool)
}

type TokenAuth struct {
	Token     string
	AccountID string
}

func (t *TokenAuth) New(cacheExpiration time.Duration) (*Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token, cloudflare.UsingAccount(t.AccountID))
	if err != nil {
		log.Printf("🤔 The token-based CloudFlare authentication failed: %v", err)
		return nil, false
	}

	cleanupInterval := cacheExpiration * CleanupIntervalFactor

	return &Handle{
		cf: handle,
		cache: Cache{
			listRecords: map[ipnet.Type]*cache.Cache{
				ipnet.IP4: cache.New(cacheExpiration, cleanupInterval),
				ipnet.IP6: cache.New(cacheExpiration, cleanupInterval),
			},
			activeZones:  cache.New(cacheExpiration, cleanupInterval),
			zoneOfDomain: cache.New(cacheExpiration, cleanupInterval),
		},
	}, true
}

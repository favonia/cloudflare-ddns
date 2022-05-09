package config

import (
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Config struct {
	Auth             api.Auth
	Policy           map[ipnet.Type]detector.Policy
	Domains          map[ipnet.Type][]api.Domain
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              api.TTL
	Proxied          bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
	Monitors         []monitor.Monitor
}

// Default gives default values.
func Default() *Config {
	return &Config{
		Auth: nil,
		Policy: map[ipnet.Type]detector.Policy{
			ipnet.IP4: detector.NewCloudflareTrace(),
			ipnet.IP6: detector.NewCloudflareTrace(),
		},
		Domains: map[ipnet.Type][]api.Domain{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		UpdateCron:       cron.MustNew("@every 5m"),
		UpdateOnStart:    true,
		DeleteOnStop:     false,
		CacheExpiration:  time.Hour * 6, //nolint:gomnd
		TTL:              api.TTL(1),
		Proxied:          false,
		UpdateTimeout:    time.Second * 30, //nolint:gomnd
		DetectionTimeout: time.Second * 5,  //nolint:gomnd
		Monitors:         nil,
	}
}

func readAuthToken(ppfmt pp.PP) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)

	// foolproof checks
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		ppfmt.Errorf(pp.EmojiUserError, "You need to provide a real API token as CF_API_TOKEN")
		return "", false
	}

	switch {
	case token != "" && tokenFile != "":
		ppfmt.Errorf(pp.EmojiUserError, "Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set")
		return "", false
	case token != "":
		return token, true
	case tokenFile != "":
		token, ok := file.ReadString(ppfmt, tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			ppfmt.Errorf(pp.EmojiUserError, "The token in the file specified by CF_API_TOKEN_FILE is empty")
			return "", false
		}

		return token, true
	default:
		ppfmt.Errorf(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE")
		return "", false
	}
}

func ReadAuth(ppfmt pp.PP, field *api.Auth) bool {
	token, ok := readAuthToken(ppfmt)
	if !ok {
		return false
	}

	accountID := Getenv("CF_ACCOUNT_ID")

	*field = &api.CloudflareAuth{Token: token, AccountID: accountID, BaseURL: ""}
	return true
}

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list *[]api.Domain) {
	api.SortDomains(*list)

	if len(*list) == 0 {
		return
	}

	j := 0
	for i := range *list {
		if i == 0 || (*list)[j] == (*list)[i] {
			continue
		}
		j++
		(*list)[j] = (*list)[i]
	}

	if len(*list) == j+1 {
		return
	}

	*list = (*list)[:j+1]
}

func ReadDomainMap(ppfmt pp.PP, field *map[ipnet.Type][]api.Domain) bool {
	var domains, ip4Domains, ip6Domains []api.Domain

	if !ReadDomains(ppfmt, "DOMAINS", &domains) ||
		!ReadDomains(ppfmt, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomains(ppfmt, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	ip4Domains = append(ip4Domains, domains...)
	ip6Domains = append(ip6Domains, domains...)

	deduplicate(&ip4Domains)
	deduplicate(&ip6Domains)

	*field = map[ipnet.Type][]api.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}

	return true
}

func ReadPolicyMap(ppfmt pp.PP, field *map[ipnet.Type]detector.Policy) bool {
	ip4Policy := (*field)[ipnet.IP4]
	ip6Policy := (*field)[ipnet.IP6]

	if !ReadPolicy(ppfmt, "IP4_POLICY", &ip4Policy) ||
		!ReadPolicy(ppfmt, "IP6_POLICY", &ip6Policy) {
		return false
	}

	*field = map[ipnet.Type]detector.Policy{
		ipnet.IP4: ip4Policy,
		ipnet.IP6: ip6Policy,
	}
	return true
}

func (c *Config) Print(ppfmt pp.PP) {
	if !ppfmt.IsEnabledFor(pp.Info) {
		return
	}

	ppfmt.Infof(pp.EmojiEnvVars, "Current settings:")
	ppfmt = ppfmt.IncIndent()

	inner := ppfmt.IncIndent()

	ppfmt.Infof(pp.EmojiConfig, "Policies:")
	inner.Infof(pp.EmojiBullet, "IPv4 policy:      %s", detector.Name(c.Policy[ipnet.IP4]))
	if c.Policy[ipnet.IP4] != nil {
		inner.Infof(pp.EmojiBullet, "IPv4 domains:     %v", c.Domains[ipnet.IP4])
	}
	inner.Infof(pp.EmojiBullet, "IPv6 policy:      %s", detector.Name(c.Policy[ipnet.IP6]))
	if c.Policy[ipnet.IP6] != nil {
		inner.Infof(pp.EmojiBullet, "IPv6 domains:     %v", c.Domains[ipnet.IP6])
	}

	ppfmt.Infof(pp.EmojiConfig, "Scheduling:")
	inner.Infof(pp.EmojiBullet, "Timezone:         %s", cron.DescribeLocation(time.Local))
	inner.Infof(pp.EmojiBullet, "Update frequency: %v", c.UpdateCron)
	inner.Infof(pp.EmojiBullet, "Update on start?  %t", c.UpdateOnStart)
	inner.Infof(pp.EmojiBullet, "Delete on stop?   %t", c.DeleteOnStop)
	inner.Infof(pp.EmojiBullet, "Cache expiration: %v", c.CacheExpiration)

	ppfmt.Infof(pp.EmojiConfig, "New DNS records:")
	inner.Infof(pp.EmojiBullet, "TTL:              %s", c.TTL.Describe())
	inner.Infof(pp.EmojiBullet, "Proxied:          %t", c.Proxied)

	ppfmt.Infof(pp.EmojiConfig, "Timeouts:")
	inner.Infof(pp.EmojiBullet, "IP detection:     %v", c.DetectionTimeout)
	inner.Infof(pp.EmojiBullet, "Record updating:  %v", c.UpdateTimeout)

	if len(c.Monitors) > 0 {
		ppfmt.Infof(pp.EmojiConfig, "Monitors:")
		for _, m := range c.Monitors {
			inner.Infof(pp.EmojiBullet, "%-17s %v", m.DescribeService()+":", m.DescribeBaseURL())
		}
	} else {
		ppfmt.Infof(pp.EmojiConfig, "Monitors: (none)")
	}
}

func (c *Config) ReadEnv(ppfmt pp.PP) bool { //nolint:cyclop
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Noticef(pp.EmojiEnvVars, "Reading settings . . .")
		ppfmt = ppfmt.IncIndent()
	}

	if !ReadAuth(ppfmt, &c.Auth) ||
		!ReadPolicyMap(ppfmt, &c.Policy) ||
		!ReadDomainMap(ppfmt, &c.Domains) ||
		!ReadCron(ppfmt, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(ppfmt, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(ppfmt, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(ppfmt, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadNonnegInt(ppfmt, "TTL", (*int)(&c.TTL)) ||
		!ReadBool(ppfmt, "PROXIED", &c.Proxied) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) ||
		!ReadHealthChecksURL(ppfmt, "HEALTHCHECKS", &c.Monitors) {
		return false
	}

	return true
}

func (c *Config) checkUselessDomains(ppfmt pp.PP) {
	count := map[api.Domain]int{}
	for _, domains := range c.Domains {
		for _, domain := range domains {
			count[domain]++
		}
	}

	for ipNet, domains := range c.Domains {
		if c.Policy[ipNet] == nil {
			for i := range domains {
				if count[domains[i]] != len(c.Domains) {
					ppfmt.Warningf(pp.EmojiUserWarning,
						"Domain %q is ignored because it is only for %s but %s is unmanaged",
						domains[i].Describe(), ipNet.Describe(), ipNet.Describe())
				}
			}
		}
	}
}

func (c *Config) NormalizeDomains(ppfmt pp.PP) bool {
	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "No domains were specified")
		return false
	}

	// change useless policies to unmanaged
	for ipNet, domains := range c.Domains {
		if len(domains) == 0 && c.Policy[ipNet] != nil {
			c.Policy[ipNet] = nil
			ppfmt.Warningf(pp.EmojiUserWarning, "IP%d_POLICY was changed to %q because no domains were set for %s",
				ipNet.Int(), detector.Name(c.Policy[ipNet]), ipNet.Describe())
		}
	}

	if c.Policy[ipnet.IP4] == nil && c.Policy[ipnet.IP6] == nil {
		ppfmt.Errorf(pp.EmojiUserError, "Both IPv4 and IPv6 are unmanaged")
		return false
	}

	c.checkUselessDomains(ppfmt)

	return true
}

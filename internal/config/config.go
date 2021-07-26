package config

import (
	"sort"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/cron"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/file"
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type Config struct {
	Quiet            quiet.Quiet
	Auth             api.Auth
	Policy           map[ipnet.Type]detector.Policy
	Domains          map[ipnet.Type][]api.FQDN
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              api.TTL
	Proxied          bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
}

// Default gives default values.
func Default() *Config {
	return &Config{
		Quiet: quiet.Quiet(false),
		Auth:  nil,
		Policy: map[ipnet.Type]detector.Policy{
			ipnet.IP4: &detector.Cloudflare{},
			ipnet.IP6: &detector.Cloudflare{},
		},
		Domains: map[ipnet.Type][]api.FQDN{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		UpdateCron:       cron.MustNew("@every 5m"),
		UpdateOnStart:    true,
		DeleteOnStop:     false,
		CacheExpiration:  time.Hour * 6, //nolint:gomnd
		TTL:              api.TTL(1),
		Proxied:          false,
		UpdateTimeout:    time.Hour,
		DetectionTimeout: time.Second * 5, //nolint:gomnd
	}
}

func readAuthToken(_ quiet.Quiet, indent pp.Indent) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)

	// foolproof checks
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		pp.Printf(indent, pp.EmojiUserError, "You need to provide a real API token as CF_API_TOKEN.")
		return "", false
	}

	switch {
	case token != "" && tokenFile != "":
		pp.Printf(indent, pp.EmojiUserError, "Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set.")
		return "", false
	case token != "":
		return token, true
	case tokenFile != "":
		token, ok := file.ReadString(indent, tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			pp.Printf(indent, pp.EmojiUserError, "The token in the file specified by CF_API_TOKEN_FILE is empty.")
			return "", false
		}

		return token, true
	default:
		pp.Printf(indent, pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE.")
		return "", false
	}
}

func readAuth(quiet quiet.Quiet, indent pp.Indent, field *api.Auth) bool {
	token, ok := readAuthToken(quiet, indent)
	if !ok {
		return false
	}

	accountID := Getenv("CF_ACCOUNT_ID")

	*field = &api.TokenAuth{Token: token, AccountID: accountID}
	return true
}

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list *[]string) {
	sort.Strings(*list)

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

func readDomains(quiet quiet.Quiet, indent pp.Indent, field map[ipnet.Type][]api.FQDN) bool {
	var domains, ip4Domains, ip6Domains []string

	if !ReadDomains(quiet, indent, "DOMAINS", &domains) ||
		!ReadDomains(quiet, indent, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomains(quiet, indent, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	ip4Domains = append(ip4Domains, domains...)
	ip6Domains = append(ip6Domains, domains...)

	deduplicate(&ip4Domains)
	deduplicate(&ip6Domains)

	field[ipnet.IP4] = make([]api.FQDN, 0, len(ip4Domains))
	for _, domain := range ip4Domains {
		field[ipnet.IP4] = append(field[ipnet.IP4], api.FQDN(domain))
	}

	field[ipnet.IP6] = make([]api.FQDN, 0, len(ip6Domains))
	for _, domain := range ip6Domains {
		field[ipnet.IP6] = append(field[ipnet.IP6], api.FQDN(domain))
	}

	return true
}

func readPolicies(quiet quiet.Quiet, indent pp.Indent, field map[ipnet.Type]detector.Policy) bool {
	ip4Policy := field[ipnet.IP4]
	ip6Policy := field[ipnet.IP6]

	if !ReadPolicy(quiet, indent, "IP4_POLICY", &ip4Policy) {
		return false
	}

	if !ReadPolicy(quiet, indent, "IP6_POLICY", &ip6Policy) {
		return false
	}

	field[ipnet.IP4] = ip4Policy
	field[ipnet.IP6] = ip6Policy
	return true
}

func PrintConfig(indent pp.Indent, c *Config) {
	pp.Printf(indent, pp.EmojiConfig, "Policies:")
	pp.Printf(indent+1, pp.EmojiBullet, "IPv4 policy:      %v", c.Policy[ipnet.IP4])
	if c.Policy[ipnet.IP4].IsManaged() {
		pp.Printf(indent+1, pp.EmojiBullet, "IPv4 domains:     %v", c.Domains[ipnet.IP4])
	}
	pp.Printf(indent+1, pp.EmojiBullet, "IPv6 policy:      %v", c.Policy[ipnet.IP6])
	if c.Policy[ipnet.IP6].IsManaged() {
		pp.Printf(indent+1, pp.EmojiBullet, "IPv6 domains:     %v", c.Domains[ipnet.IP6])
	}

	pp.Printf(indent, pp.EmojiConfig, "Scheduling:")
	pp.Printf(indent+1, pp.EmojiBullet, "Timezone:         %s", cron.DescribeTimezone())
	pp.Printf(indent+1, pp.EmojiBullet, "Update frequency: %v", c.UpdateCron)
	pp.Printf(indent+1, pp.EmojiBullet, "Update on start?  %t", c.UpdateOnStart)
	pp.Printf(indent+1, pp.EmojiBullet, "Delete on stop?   %t", c.DeleteOnStop)
	pp.Printf(indent+1, pp.EmojiBullet, "Cache expiration: %v", c.CacheExpiration)

	pp.Printf(indent, pp.EmojiConfig, "New DNS records:")
	pp.Printf(indent+1, pp.EmojiBullet, "TTL:              %v", c.TTL)
	pp.Printf(indent+1, pp.EmojiBullet, "Proxied:          %t", c.Proxied)

	pp.Printf(indent, pp.EmojiConfig, "Timeouts")
	pp.Printf(indent+1, pp.EmojiBullet, "IP detection:     %v", c.DetectionTimeout)
}

func (c *Config) ReadEnv(indent pp.Indent) bool { //nolint:cyclop
	if !ReadQuiet(indent, "QUIET", &c.Quiet) {
		return false
	}

	if c.Quiet {
		pp.Printf(indent, pp.EmojiMute, "Quiet mode enabled.")
	} else {
		pp.Printf(indent, pp.EmojiEnvVars, "Reading settings . . .")
		indent = 1
	}

	if !readAuth(c.Quiet, indent, &c.Auth) ||
		!readPolicies(c.Quiet, indent, c.Policy) ||
		!readDomains(c.Quiet, indent, c.Domains) ||
		!ReadCron(c.Quiet, indent, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(c.Quiet, indent, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(c.Quiet, indent, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(c.Quiet, indent, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadNonnegInt(c.Quiet, indent, "TTL", (*int)(&c.TTL)) ||
		!ReadBool(c.Quiet, indent, "PROXIED", &c.Proxied) ||
		!ReadNonnegDuration(c.Quiet, indent, "DETECTION_TIMEOUT", &c.DetectionTimeout) {
		return false
	}

	return true
}

func (c *Config) checkUselessDomains(indent pp.Indent) {
	var (
		domainSet    = map[ipnet.Type]map[string]bool{ipnet.IP4: {}, ipnet.IP6: {}}
		unionSet     = map[string]bool{}
		intersectSet = map[string]bool{}
	)
	// calculate domainSet[IP4], domainSet[IP6], and unionSet
	for ipNet, domains := range c.Domains {
		for _, domain := range domains {
			domainString := domain.String()
			domainSet[ipNet][domainString] = true
			unionSet[domainString] = true
		}
	}

	// calculate intersectSet
	for domain := range unionSet {
		intersectSet[domain] = domainSet[ipnet.IP4][domain] && domainSet[ipnet.IP6][domain]
	}

	for ipNet := range c.Domains {
		if !c.Policy[ipNet].IsManaged() {
			for domain := range domainSet[ipNet] {
				if !intersectSet[domain] {
					pp.Printf(indent, pp.EmojiUserWarning,
						"Domain %v is ignored because it is only for %v but %v is unmanaged.", domain, ipNet, ipNet)
				}
			}
		}
	}
}

func (c *Config) Normalize(indent pp.Indent) bool {
	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		pp.Printf(indent, pp.EmojiUserError, "No domains were specified.")
		return false
	}

	// change useless policies to
	for ipNet, domains := range c.Domains {
		if len(domains) == 0 && c.Policy[ipNet].IsManaged() {
			c.Policy[ipNet] = &detector.Unmanaged{}
			pp.Printf(indent, pp.EmojiUserWarning, "IP%v_POLICY was changed to %q because no domains were set for %v.",
				ipNet.Int(), c.Policy[ipNet], ipNet)
		}
	}

	if !c.Policy[ipnet.IP4].IsManaged() && !c.Policy[ipnet.IP6].IsManaged() {
		pp.Printf(indent, pp.EmojiUserError, "Both IPv4 and IPv6 are unmanaged.")
		return false
	}

	c.checkUselessDomains(indent)

	return true
}

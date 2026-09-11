module github.com/favonia/cloudflare-ddns

go 1.27.0 // with patch version to satisfy CodeQL

retract (
	v1.14.1 // nil pointer bug
	[v0.0.0, v1.7.99] // incompatible templates for PROXIED before 1.7.1; for safety, 1.7.* are also retracted
)

require (
	github.com/cloudflare/cloudflare-go v0.118.0
	github.com/containrrr/shoutrrr v0.8.0
	github.com/google/go-querystring v1.2.0
	github.com/hashicorp/go-retryablehttp v0.7.8
	github.com/jellydator/ttlcache/v3 v3.4.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.12.1
	go.uber.org/mock v0.6.0
	golang.org/x/net v0.59.0
	golang.org/x/text v0.42.0
	pgregory.net/rapid v1.3.0
)

require (
	github.com/fatih/color v1.16.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
)

tool go.uber.org/mock/mockgen

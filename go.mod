module github.com/favonia/cloudflare-ddns

go 1.27.0 // with patch version to satisfy CodeQL

retract (
	v1.14.1 // nil pointer bug
	[v0.0.0, v1.7.99] // incompatible templates for PROXIED before 1.7.1; for safety, 1.7.* are also retracted
)

require (
	github.com/cloudflare/cloudflare-go v0.118.0
	github.com/google/go-querystring v1.2.0
	github.com/hashicorp/go-retryablehttp v0.7.8
	github.com/jellydator/ttlcache/v3 v3.4.1
	github.com/nicholas-fedor/shoutrrr v0.21.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.12.1
	go.uber.org/mock v0.6.0
	golang.org/x/net v0.59.0
	golang.org/x/text v0.42.0
	pgregory.net/rapid v1.3.0
)

require (
	github.com/eclipse/paho.golang v0.23.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/term v0.46.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	mellium.im/reader v0.1.0 // indirect
	mellium.im/sasl v0.3.2 // indirect
	mellium.im/xmlstream v0.15.4 // indirect
	mellium.im/xmpp v0.23.0 // indirect
)

tool go.uber.org/mock/mockgen

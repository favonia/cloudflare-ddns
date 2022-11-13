# Internal Libraries

- `api`: access and cache DNS service API, currently only supporting Cloudflare
- `config`: read configuration settings from environment variables
- `cron`: parse Cron expressions
- `domain`: handle domain names and split them into possible subdomains and zones
- `domainexp`: parse domain lists and parse boolean expressions on domains (for `PROXIED`)
- `file`: virtualize file systems (to enable testing)
- `ipnet`: define a type for labelling IPv4 and IPv6
- `monitor`: ping the monitoring API, currently only supporting Healthchecks
- `pp`: pretty print messages with emojis
- `provider`: find out the public IP
- `setter`: set the IP of one domain using a DNS service API
- `updater`: detect and update the IP of all domains, using `provider` and `setter`

# 🌟 CloudFlare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/favonia/cloudflare-ddns/Building%20and%20Pushing)](https://github.com/favonia/cloudflare-ddns/actions/workflows/build.yaml)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/favonia/cloudflare-ddns)](https://golang.org/doc/install)
[![Docker Pulls](https://img.shields.io/docker/pulls/favonia/cloudflare-ddns)](https://hub.docker.com/r/favonia/cloudflare-ddns)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)

A small and fast DDNS updater for CloudFlare.

```
2021/07/05 07:15:52 🚷 Erasing supplementary group IDs . . .
2021/07/05 07:15:52 🤷 Could not erase supplementary group IDs: operation not permitted
2021/07/05 07:15:52 🧑 Effective user ID of the process: 1000.
2021/07/05 07:15:52 👪 Effective group ID of the process: 1000.
2021/07/05 07:15:52 👪 Supplementary group IDs of the process: [……].
2021/07/05 07:15:52 📜 Managed domains: [……]
2021/07/05 07:15:52 📜 Policy for IPv4: cloudflare
2021/07/05 07:15:52 📜 Policy for IPv6: cloudflare
2021/07/05 07:15:52 📜 TTL for new DNS entries: 1 (1 = automatic)
2021/07/05 07:15:52 📜 Whether new DNS entries are proxied: false
2021/07/05 07:15:52 📜 Refresh interval: 5m0s
2021/07/05 07:15:52 📜 Whether managed records are deleted on exit: true
2021/07/05 07:15:52 📜 Timeout of each attempt to detect IP addresses: 5s
2021/07/05 07:15:52 📜 Expiration of cached CloudFlare API responses: 6h0m0s
2021/07/05 07:15:53 🧐 Found the IPv4 address: ……
2021/07/05 07:15:53 🧐 Found the IPv6 address: ……
2021/07/05 07:15:53 🧐 Found the zone of the domain ……: …….
2021/07/05 07:15:54 👶 Adding a new A record: ……
2021/07/05 07:15:55 👶 Adding a new AAAA record: ……
2021/07/05 07:15:55 😴 Checking the IP addresses again in 5m0s . . .
```

## 📜 Highlights

* Ultra-small Docker images (~2MB) for all popular architectures.
* Ability to update multiple domains across different zones.
* Ability to remove stale records or choose to remove records on exit.
* Ability to obtain IP addresses from either public servers or local network interfaces (configurable).
* Ability to enable or disable IPv4 and IPv6 individually.
* Full configurability via environment variables.
* Ability to pass API tokens via an environment variable or a file.
* Local caching to reduce CloudFlare API usage.

## 🛡️ Privacy and Security

* By default, public IP addresses are obtained via [CloudFlare’s debugging interface](https://1.1.1.1/cdn-cgi/trace). This minimizes the impact on privacy because we are already using the CloudFlare API to update DNS records. You can also configure the tool to use [ipify](https://www.ipify.org) which, unlike the debugging interface, is fully documented.
* The root privilege is immediately dropped after the program starts.
* The source code dependes on these two external libraries, other than the Go standard library:
  - [cloudflare/cloudflare-go](https://github.com/cloudflare/cloudflare-go): the official Go binding for CloudFlare API v4.
  - [patrickmn/go-cache](https://github.com/patrickmn/go-cache): simple in-memory caching, essentially `map[string]interface{}` with expiration times.

The CloudFlare binding provides robust handling of pagination and other nuisances of the CloudFlare API, and the in-memory caching helps reduce the API usage.

## 🐋 Quick Start with Docker

```bash
docker run \
  --network host \
  -e CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  -e DOMAINS=www.example.org \
  -e PROXIED=true \
  favonia/cloudflare-ddns
```

## 🏃 Quick Start from Source

You need the [Go tool](https://golang.org/doc/install) to run the program from its source.

```bash
export CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
export DOMAINS=www.example.org
export PROXIED=true
go run ./cmd/ddns.go
```

## 🐋 Deployment with Docker Compose

### Step 1: Updating the Compose File

Incorporate the following fragment into the compose file (typically `docker-compose.yml` or `docker-compose.yaml`).

```yaml
version: "3"
services:
  cloudflare-ddns:
    image: favonia/cloudflare-ddns:latest
    security_opt:
      - no-new-privileges:true
    network_mode: host
    environment:
      - CF_API_TOKEN
      - DOMAINS
      - PROXIED=true
```

⚠️ The setting `network_mode: host` is for IPv6. If you wish to keep the network separated from the host network, check out the proper way to [enable IPv6 support](https://docs.docker.com/config/daemon/ipv6/).

💡 The setting `no-new-privileges:true` provides additional protection when you run the container as a non-root user. (The tool itself will also attempt to drop the root privilege.)

💡 The setting `PROXIED=true` instructs CloudFlare to cache webpages and hide your actual IP addresses. If you wish to bypass that, simply remove `PROXIED=true`. (The default value of `PROXIED` is `false`.)

💡 There is no need to use automatic restart (_e.g.,_ `restart: unless-stopped`) because the tool exits only when non-recoverable errors happen or when you manually stop it.

### Step 2: Updating the Environment File

Add these lines to your environment file (typically `.env`):
```bash
CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
DOMAINS=example.org,www.example.org,example.io
```

- The value of `CF_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Create a token with the **Zone - DNS - Edit** permission and copy the token into the environment file.

  ⚠️ The legacy API key authentication is intentionally _not_ supported. Please use the more secure API tokens.

- The value of `DOMAINS` should be a list of fully qualified domain names separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs the tool to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to be in the same zone---the tool will identify their zones automatically.

The tool should be up and running after these commands:
```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```

## Further Customization

Here are all the environment variables the tool recognizes, in the alphabetic order.

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CACHE_EXPIRATION` | Positive time duration with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The expiration of cached CloudFlare API responses | No | `6h0m0s` (6 hours)
| `CF_API_TOKEN_FILE` | Paths to files containing CloudFlare API tokens | A file that contains the token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `CF_API_TOKEN` | CloudFlare API tokens | The token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `DELETE_ON_EXIT` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether managed DNS records should be deleted on exit | No | `false`
| `DETECTION_TIMEOUT` | Positive time duration with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The timeout of each attempt to detect IP addresses | No | `5s` (5 seconds)
| `DOMAINS` | Comma-separated fully qualified domain names | All the domains this tool should manage | Yes, and the list cannot be empty | N/A
| `IP4_POLICY` | `cloudflare`, `ipify`, `local`, and `unmanaged` | (See below) | No | `cloudflare`
| `IP6_POLICY` | `cloudflare`, `ipify`, `local`, and `unmanaged` | (See below) | No | `cloudflare`
| `PGID` | POSIX group ID | The effective group ID the tool should assume | No | Effective group ID; if it is zero, then the real group ID; if it is still zero, then `1000`
| `PROXIED` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether new DNS records should be proxied by CloudFlare | No | `false`
| `PUID` | POSIX user ID | The effective user ID the tool should assume | No | Effective user ID; if it is zero, then the real user ID; if it is still zero, then `1000`
| `QUIET` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether the tool should reduce the logging | No | `false`
| `REFRESH_INTERVAL` | Positive time duration with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The refresh interval for the tool to re-check IP addresses and update DNS records (if necessary) | No | `5m0s` (5 minutes)
| `TTL` | Time-to-live (TTL) values in seconds | The TTL values used to create new DNS records | No | `1` (This means “automatic” to CloudFlare)

💡 The values of `IP4_POLICY` and `IP6_POLICY` should be one of the following policies:

- `cloudflare`: Get the public IP address via [CloudFlare’s debugging interface](https://1.1.1.1/cdn-cgi/trace) and update DNS records accordingly.
- `ipify`: Get the public address via [ipify’s public API](https://www.ipify.org/) and update DNS records accordingly.
- `local`: Get the address via local network interfaces and update DNS records accordingly. When multiple local network interfaces or in general multiple IP addresses are present, the tool will use the address that would have been used for outbound UDP connections to CloudFlare servers.

  ⚠️ You need `network_mode: host` for the `local` policy, for otherwise the tool will detect the addresses inside the [bridge network set up by Docker](https://docs.docker.com/network/bridge/) instead of those in the host network.

- `unmanaged`: Stop the DNS updating completely. Existing DNS records will not be removed.

The option `IP4_POLICY` is governing IPv4 addresses and `A`-type records, while the option `IP6_POLICY` is governing IPv6 addresses and `AAAA`-type records. The two options act independently of each other. Both of them apply to all managed domains.

If you are using Docker Compose, run the following command to recreate the container after customizing the tool:
```bash
docker-compose up --detach
```

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new).

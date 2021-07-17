# 🌟 CloudFlare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/favonia/cloudflare-ddns/Building%20and%20Pushing)](https://github.com/favonia/cloudflare-ddns/actions/workflows/build.yaml)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/favonia/cloudflare-ddns)](https://golang.org/doc/install)
[![Docker Pulls](https://img.shields.io/docker/pulls/favonia/cloudflare-ddns)](https://hub.docker.com/r/favonia/cloudflare-ddns)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)

A small and fast DDNS updater for CloudFlare.

```
2021/07/05 07:15:52 🧑 Effective user ID: 1000.
2021/07/05 07:15:52 👪 Effective group ID: 1000.
2021/07/05 07:15:52 🤫 Quiet mode enabled.
2021/07/05 07:15:53 🧐 Found the IPv4 address: ……
2021/07/05 07:15:53 🧐 Found the IPv6 address: ……
2021/07/05 07:15:53 🧐 Found the zone of the domain ……: …….
2021/07/05 07:15:54 👶 Adding a new A record: ……
2021/07/05 07:15:55 👶 Adding a new AAAA record: ……
2021/07/05 07:15:55 😴 Checking the IP addresses again in 5m0s . . .
```

## 📜 Highlights

* Ultra-small Docker images (about 2 MB) for all architectures.
* Ability to update multiple domains across different zones.
* Ability to enable or disable IPv4 and IPv6 individually.
* Support of internationalized domain names.
* Ability to remove stale records or choose to remove records on exit/stop.
* Ability to obtain IP addresses from CloudFlare, ipify, or local network interfaces.
* Support of timezone and Cron expressions.
* Full configurability via environment variables.
* Ability to pass API tokens via a file instead of an environment variable.
* Local caching to reduce CloudFlare API usage.

## 🕵️ Privacy

By default, public IP addresses are obtained using [CloudFlare via DNS-over-HTTPS](https://developers.cloudflare.com/1.1.1.1/dns-over-https). This minimizes the impact on privacy because we are already using the CloudFlare API to update DNS records. You can also configure the tool to use [ipify](https://www.ipify.org) which, unlike the debugging interface, is fully documented.

## 🛡️ Security

<details><summary>🚷 The superuser privilege is immediately dropped after the program starts.</summary>

The program honors `PGID` and `PUID` and will drop Linux capabilities (divided superuser privileges).
</details>

<details><summary>🔌 The source code depends on four external libraries (outside the Go project).</summary>

- [cap](https://sites.google.com/site/fullycapable):\
  Manipulation of Linux capabilities.
- [cloudflare-go](https://github.com/cloudflare/cloudflare-go):\
  The official Go binding of CloudFlare API v4. It provides robust handling of pagination, rate limiting, and other tricky details.
- [cron](https://github.com/robfig/cron):\
  Parsing of Cron expressions.
- [go-cache](https://github.com/patrickmn/go-cache):\
  Essentially `map[string]interface{}` with expiration times.
</details>

## ⛷️ Quick Start

<details>
<summary>🐋 Directly run the provided Docker images.</summary>

```bash
docker run \
  --network host \
  -e CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  -e DOMAINS=www.example.org \
  -e PROXIED=true \
  favonia/cloudflare-ddns
```
</details>

<details>
<summary>🧬 Directly run the Go program from its source.</summary>

You need the [Go tool](https://golang.org/doc/install) to run the program from its source.

```bash
export CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
export DOMAINS=www.example.org
export PROXIED=true
go run ./cmd/ddns.go
```
</details>

## 🐋 Deployment with Docker Compose

### 📦 Step 1: Updating the Compose File

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

<details>
<summary>📡 You want <code>network_mode: host</code> for IPv6.</summary>

The setting `network_mode: host` is for IPv6. If you wish to keep the network separated from the host network, check out the proper way to [enable IPv6 support](https://docs.docker.com/config/daemon/ipv6/).
</details>

<details>
<summary>🛡️ Use <code>no-new-privileges:true</code> to protect yourself.</summary>

The setting `no-new-privileges:true` provides additional protection, especially when you run the container as a non-superuser. (The program itself will also attempt to drop the superuser privilege and all capabilities.)
</details>

<details>
<summary>🎭 Use <code>PROXIED=true</code> to hide your IP addresses.</summary>

The setting `PROXIED=true` instructs CloudFlare to cache webpages on your machine and hide your actual IP addresses. If you wish to bypass that and expose your actual IP addresses, simply remove `PROXIED=true`. (The default value of `PROXIED` is `false`.)
</details>

<details>
<summary>🙅 No automatic <code>restart</code> is needed.</summary>

There is no need to use automatic restart (_e.g.,_ `restart: unless-stopped`) because the tool exits only when non-recoverable errors happen or when you manually stop it.
</details>

### 🪧 Step 2: Updating the Environment File

Add these lines to your environment file (typically `.env`):
```bash
CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
DOMAINS=example.org,www.example.org,example.io
```

<details>
<summary>🔑 <code>CF_API_TOKEN</code> is your CloudFlare API token.</summary>

The value of `CF_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Use the **Edit zone DNS** template to create and copy a token into the environment file. ⚠️ The less secure API key authentication is deliberately _not_ supported.
</details>

<details>
<summary>📍 <code>DOMAINS</code> contains the domains to update.</summary>

The value of `DOMAINS` should be a list of fully qualified domain names separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs the tool to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to be in the same zone---the tool will identify their zones automatically.
</details>

Run these commands to update the container:
```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```

## 🎛️ Further Customization

<details>
<summary>🔑 Specifying accounts and tokens</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CF_ACCOUNT_ID` | CloudFlare Account IDs | The account ID used to distinguish multiple zone IDs with the same name | No | `""` (unset) |
| `CF_API_TOKEN_FILE` | Paths to files containing CloudFlare API tokens | A file that contains the token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `CF_API_TOKEN` | CloudFlare API tokens | The token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |

In most cases, `CF_ACCOUNT_ID` is not needed.
</details>

<details>
<summary>📍 Detecting IP addresses and updating DNS records</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `DELETE_ON_STOP` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether managed DNS records should be deleted on exit | No | `false`
| `DOMAINS` | Comma-separated fully qualified domain names | The domains this tool should manage | (See below) | N/A
| `IP4_DOMAINS` | Comma-separated fully qualified domain names | The domains this tool should manage for `A` records | (See below) | N/A
| `IP4_POLICY` | `cloudflare`, `ipify`, `local`, and `unmanaged` | (See below) | No | `cloudflare` if `DOMAINS` or `IP4_DOMAINS` is not empty; otherwise, `unmanaged`
| `IP6_DOMAINS` | Comma-separated fully qualified domain names | The domains this tool should manage for `AAAA` records | (See below) | N/A
| `IP6_POLICY` | `cloudflare`, `ipify`, `local`, and `unmanaged` | (See below) | No | `cloudflare` if `DOMAINS` or `IP6_DOMAINS` is not empty; otherwise, `unmanaged`
| `PROXIED` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether new DNS records should be proxied by CloudFlare | No | `false`
| `TTL` | Time-to-live (TTL) values in seconds | The TTL values used to create new DNS records | No | `1` (This means “automatic” to CloudFlare)

> <details>
> <summary>📜 Available policies for <code>IP4_POLICY</code> and <code>IP6_POLICY</code></summary>
>
> - `cloudflare`\
>  Get the public IP address by querying `whoami.cloudflare.` against [CloudFlare via DNS-over-HTTPS](https://developers.cloudflare.com/1.1.1.1/dns-over-https) and update DNS records accordingly.
> - `ipify`\
>   Get the public IP address via [ipify’s public API](https://www.ipify.org/) and update DNS records accordingly.
> - `local`\
>   Get the address via local network interfaces and update DNS records accordingly. When multiple local network interfaces or in general multiple IP addresses are present, the tool will use the address that would have been used for outbound UDP connections to CloudFlare servers. ⚠️ You need `network_mode: host` for this policy, for otherwise the tool will detect the addresses inside the [bridge network](https://docs.docker.com/network/bridge/) instead of those in the host network.
> - `unmanaged`\
>   Stop the DNS updating completely. Existing DNS records will not be removed.
>
> The option `IP4_POLICY` is governing IPv4 addresses and `A`-type records, while the option `IP6_POLICY` is governing IPv6 addresses and `AAAA`-type records. The two options act independently of each other.
> </details>

> <details>
> <summary>📍 Requirements of domain specifications: <code>DOMAINS</code> and <code>IP4/6_DOMAINS</code></summary>
>
> At least one domain should be specified in `DOMAINS`, `IP4_DOMAINS`, or `IP6_DOMAINS`, for otherwise this program is useless. It is fine to list the same domain in both `IP4_DOMAINS` and `IP6_DOMAINS`, which is equivalent to listing it in `DOMAINS`.
> </details>

</details>

<details>
<summary>⏳ Scheduling the updating</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CACHE_EXPIRATION` | Positive time duration with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The expiration of cached CloudFlare API responses | No | `6h0m0s` (6 hours)
| `CF_API_TIMEOUT` | Positive time duration with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The timeout of each attempt to access the CloudFlare API | No | `10s` (10 seconds)
| `DETECTION_TIMEOUT` | Positive time duration with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The timeout of each attempt to detect IP addresses | No | `5s` (5 seconds)
| `REFRESH_CRON` | Cron expressions; [documentation of cron](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format). | The schedule to re-check IP addresses and update DNS records (if necessary) | No | `@every 5m` (every 5 minutes)
| `REFRESH_ON_START` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether to check IP addresses on start regardless of `REFRESH_CRON` | No | `true`
| `TZ` | Recognized timezones, such as `UTC` | The timezone used for logging and parsing `REFRESH_CRON` | No | `UTC`

Note that the update schedule does not take the time to update records into consideration. For example, if the schedule is “for every 5 minutes”, and if the updating itself takes 2 minutes, then the interval between adjacent updates is 3 minutes, not 5 minutes.
</details>

<details>
<summary>🛡️ Dropping superuser privileges</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `PGID` | Non-zero POSIX group ID | The effective group ID the tool should assume | No | Effective group ID; if it is zero, then the real group ID; if it is still zero, then `1000`
| `PUID` | Non-zero POSIX user ID | The effective user ID the tool should assume | No | Effective user ID; if it is zero, then the real user ID; if it is still zero, then `1000`

The tool will also try to drop supplementary groups.
</details>

<details>
<summary>🖥️ Tweaking the user interface</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `QUIET` | `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False` | Whether the tool should reduce the logging | No | `false`
</details>

### 🔁 Restarting the Container

If you are using Docker Compose, run `docker-compose up --detach` after changing the settings.

## 🚵 Migration Guides

<details>
<summary>I am migrating from <a href="https://hub.docker.com/r/oznu/cloudflare-ddns/">oznu/cloudflare-ddns</a>.</summary>

⚠️ [oznu/cloudflare-ddns](https://hub.docker.com/r/oznu/cloudflare-ddns/) relies on unverified DNS responses to obtain public IP addresses; a malicious hacker could potentially manipulate DNS responses and trick it into updating your domain with any IP address.

| Old Parameter |  | New Paramater |
| ------------- | - | ------------- |
| `API_KEY=key` | ✔️ | Use `CF_API_TOKEN=key` |
| `API_KEY_FILE=file` | ✔️ | Use `CF_API_TOKEN_FILE=file` |
| `ZONE=example.org` and `SUBDOMAIN=sub` | ✔️ | Use `DOMAINS=sub.example.org` directly |
| `PROXIED=true` | ✔️ | Same |
| `RRTYPE=A` | ✔️ | Use `IP6_POLICY=unmanaged` |
| `RRTYPE=AAAA` | ✔️ | Use `IP4_POLICY=unmanaged` |
| `DELETE_ON_STOP=true` | ✔️ | Same |
| `INTERFACE=iface` | ✔️ | Not required for `local` policies; the tool can handle multiple network interfaces |
| `CUSTOM_LOOKUP_CMD=cmd` | ❌ | _There is not even a shell in the minimum Docker image._ |
| `DNS_SERVER=server` | ❌ | _Only the CloudFlare server is supported._ |

💡 There is no need to specify `--restart=always` because the tool exits only when unrecoverable errors occur.

</details>

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new).

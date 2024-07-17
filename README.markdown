# 🌟 Cloudflare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![Go Reference](https://pkg.go.dev/badge/github.com/favonia/cloudflare-ddns/.svg)](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/)
[![Codecov](https://img.shields.io/codecov/c/github/favonia/cloudflare-ddns)](https://app.codecov.io/gh/favonia/cloudflare-ddns)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)
[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/6680/badge)](https://bestpractices.coreinfrastructure.org/projects/6680)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/favonia/cloudflare-ddns/badge)](https://securityscorecards.dev/viewer/?uri=github.com/favonia/cloudflare-ddns)

A feature-rich and robust Cloudflare DDNS updater with a small footprint. The program will detect your machine's public IP addresses and update DNS records using the Cloudflare API.

## 📜 Highlights

### ⚡ Efficiency

- 🤏 The Docker image takes less than 5 MB after compression.
- 🔁 The Go runtime re-uses existing HTTP connections.
- 🗃️ Cloudflare API responses are cached to reduce the API usage.

### 💯 Complete Support of Domain Names

- 😌 You can simply list domains (_e.g._, `www.a.org, hello.io`) without knowing their DNS zones.
- 🌍 [Internationalized domain names](https://en.wikipedia.org/wiki/Internationalized_domain_name) (_e.g._, `🐱.example.org` and `日本｡co｡jp`) are fully supported.
- 🃏 [Wildcard domains](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (_e.g._, `*.example.org`) are also supported.
- 🕹️ You can toggle IPv4 (`A` records), IPv6 (`AAAA` records) and Cloudflare proxying for each domain.

### 🕵️ Privacy

By default, public IP addresses are obtained via [Cloudflare debugging page](https://one.one.one.one/cdn-cgi/trace). This minimizes the impact on privacy because we are already using the Cloudflare API to update DNS records. Moreover, if Cloudflare servers are not reachable, chances are you cannot update DNS records anyways.

### 👁️ Notification

- 🩺 The updater can notify you via [Healthchecks](https://healthchecks.io) and [Uptime Kuma](https://uptime.kuma.pet) when it fails.
- 📣 The updater can also send you general updates via [shoutrrr](https://containrrr.dev/shoutrrr/).

### 🛡️ Security

- 🛡️ The updater uses only HTTPS or [DNS over HTTPS](https://en.wikipedia.org/wiki/DNS_over_HTTPS) to detect IP addresses; see the [Security Model](docs/DESIGN.markdown#network-security-threat-model).
- <details><summary>📚 The updater uses only established open-source Go libraries <em>(click to expand)</em></summary>

  - [cloudflare-go](https://github.com/cloudflare/cloudflare-go):\
    The official Go binding of Cloudflare API v4.
  - [cron](https://github.com/robfig/cron):\
    Parsing of Cron expressions.
  - [go-retryablehttp](https://github.com/hashicorp/go-retryablehttp):\
    HTTP clients with automatic retries and exponential backoff.
  - [go-querystring](https://github.com/google/go-querystring):\
    A library to construct URL query parameters.
  - [shoutrrr](https://github.com/containrrr/shoutrrr):\
    A notification library for sending general updates.
  - [ttlcache](https://github.com/jellydator/ttlcache):\
    In-memory cache to hold Cloudflare API responses.
  - [mock](https://go.uber.org/mock) (for testing only):\
    A comprehensive, semi-official framework for mocking.
  - [testify](https://github.com/stretchr/testify) (for testing only):\
    A comprehensive tool set for testing Go programs.

  </details>

## ⛷️ Quick Start

_(Click to expand the following items.)_

<details><summary>🐋 Directly run the Docker image.</summary>

```bash
docker run \
  --network host \
  -e CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  -e DOMAINS=example.org,www.example.org,example.io \
  -e PROXIED=true \
  favonia/cloudflare-ddns:latest
```

</details>

<details><summary>🧬 Directly run the updater from its source.</summary>

You need the [Go tool](https://golang.org/doc/install) to run the updater from its source.

```bash
CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  DOMAINS=example.org,www.example.org,example.io \
  PROXIED=true \
  go run github.com/favonia/cloudflare-ddns/cmd/ddns
```

</details>

## 🐋 Deployment with Docker Compose

### 📦 Step 1: Updating the Compose File

Incorporate the following fragment into the compose file (typically `docker-compose.yml` or `docker-compose.yaml`). The template may look a bit scary, but only because it includes various optional flags for extra security protection.

```yaml
services:
  cloudflare-ddns:
    image: favonia/cloudflare-ddns:latest
    network_mode: host
    # This bypasses network isolation and makes IPv6 easier (optional; see below)
    restart: always
    # Restart the updater after reboot
    user: "1000:1000"
    # Run the updater with a specific user ID and group ID (in that order).
    # You should change the two numbers based on your setup.
    # This is optional but highly recommended; otherwise, you will probably
    # run the updater as a superuser (root), which is usually a bad idea.
    read_only: true
    # Make the container filesystem read-only (optional but recommended)
    cap_drop: [all]
    # Drop all Linux capabilities (optional but recommended)
    security_opt: [no-new-privileges:true]
    # Another protection to restrict superuser privileges (optional but recommended)
    environment:
      - CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
        # Your Cloudflare API token
      - DOMAINS=example.org,www.example.org,example.io
        # Your domains (separated by commas)
      - PROXIED=true
        # Tell Cloudflare to cache webpages and hide your IP (optional)
```

_(Click to expand the following important tips.)_

<details>
<summary>🔑 <code>CF_API_TOKEN</code> is your Cloudflare API token</summary>

The value of `CF_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Use the **Edit zone DNS** template to create and copy a token into the environment file. (The less secure API key authentication is deliberately _not_ supported.)

</details>

<details>
<summary>📍 <code>DOMAINS</code> is the list of domains to update</summary>

The value of `DOMAINS` should be a list of [fully qualified domain names (FQDNs)](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs the updater to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to be in the same zone---the updater will identify their zones automatically.

</details>

<details>
<summary>🚨 Remove <code>PROXIED=true</code> if you are <em>not</em> running a web server</summary>

The setting `PROXIED=true` instructs Cloudflare to cache webpages and hide your IP addresses. If you wish to bypass that and expose your actual IP addresses, remove `PROXIED=true`. If your traffic is not HTTP(S), then Cloudflare cannot proxy it and you should probably turn off the proxying by removing `PROXIED=true`. The default value of `PROXIED` is `false`.

</details>

<details>
<summary>📴 Add <code>IP6_PROVIDER=none</code> if you want to disable IPv6 completely</summary>

The updater, by default, will attempt to update DNS records for both IPv4 and IPv6, and there is no harm in leaving the automatic detection on even if your network does not work for one of them. However, if you want to disable IPv6 entirely (perhaps to avoid all the detection errors), add the setting `IP6_PROVIDER=none`.

</details>

<details>
<summary>📡 Expand this if you want IPv6 without bypassing network isolation (without <code>network_mode: host</code>)</summary>

The easiest way to enable IPv6 is to use `network_mode: host` so that the updater can access the host IPv6 network directly. This has the downside of bypassing the network isolation. If you wish to keep the updater isolated from the host network, remove `network_mode: host` and follow the steps in the [official Docker documentation to enable IPv6](https://docs.docker.com/config/daemon/ipv6/). Use newer versions of Docker that come with (much) better IPv6 support.

</details>

<details>
<summary>🛡️ Change <code>user: "1000:1000"</code> to the user and group IDs you want to use</summary>

Change `1000:1000` to `USER:GROUP` for the `USER` and `GROUP` IDs you wish to use to run the updater. The settings `cap_drop`, `read_only`, and `no-new-privileges` in the template provide additional protection, especially when you run the container as a non-superuser.

</details>

### 🚀 Step 2: Building the Container

```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```

## 🎛️ Further Customization

### ⚙️ All Settings

_(Click to expand the following items.)_

<details>
<summary>🔑 Cloudflare accounts and API tokens</summary>

| Name                | Valid Values                                                                                              | Meaning                                                                                                              | Required?                                                           | Default Value |
| ------------------- | --------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- | ------------- |
| `CF_ACCOUNT_ID`     | [Cloudflare Account IDs](https://developers.cloudflare.com/fundamentals/setup/find-account-and-zone-ids/) | The Cloudflare account ID used to distinguish multiple DNS zones with the same name. It is _not_ your email address! | No (in most cases you can leave it blank)                           | (unset)       |
| `CF_API_TOKEN`      | Cloudflare API tokens                                                                                     | The token to access the Cloudflare API                                                                               | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A           |
| `CF_API_TOKEN_FILE` | Paths to files containing Cloudflare API tokens                                                           | A file that contains the token to access the Cloudflare API                                                          | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A           |

</details>

<details>
<summary>📍 Domains and IP providers</summary>

| Name           | Valid Values                                                          | Meaning                                                                                                             | Required?   | Default Value      |
| -------------- | --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ----------- | ------------------ |
| `DOMAINS`      | Comma-separated fully qualified domain names or wildcard domain names | The domains the updater should manage for both `A` and `AAAA` records                                               | (See below) | (empty list)       |
| `IP4_DOMAINS`  | Comma-separated fully qualified domain names or wildcard domain names | The domains the updater should manage for `A` records                                                               | (See below) | (empty list)       |
| `IP6_DOMAINS`  | Comma-separated fully qualified domain names or wildcard domain names | The domains the updater should manage for `AAAA` records                                                            | (See below) | (empty list)       |
| `IP4_PROVIDER` | `cloudflare.doh`, `cloudflare.trace`, `local`, `url:URL`, or `none`   | How to detect IPv4 addresses, or `none` to disable IPv4. (See below for the detailed description of each provider.) | No          | `cloudflare.trace` |
| `IP6_PROVIDER` | `cloudflare.doh`, `cloudflare.trace`, `local`, `url:URL`, or `none`   | How to detect IPv6 addresses, or `none` to disable IPv6. (See below for the detailed description of each provider.) | No          | `cloudflare.trace` |

> <details>
> <summary>📍 At least one of <code>DOMAINS</code> and <code>IP4/6_DOMAINS</code> must be non-empty.</summary>
>
> At least one domain should be listed in `DOMAINS`, `IP4_DOMAINS`, or `IP6_DOMAINS`. Otherwise, if all of them are empty, then the updater has nothing to do. It is fine to list the same domain in both `IP4_DOMAINS` and `IP6_DOMAINS`, which is equivalent to listing it in `DOMAINS`. Internationalized domain names are supported using the non-transitional processing that is fully compatible with IDNA2008.
>
> </details>

> <details>
> <summary>📜 Available providers for <code>IP4_PROVIDER</code> and <code>IP6_PROVIDER</code>:</summary>
>
> - `cloudflare.doh`\
>   Get the public IP address by querying `whoami.cloudflare.` against [Cloudflare via DNS-over-HTTPS](https://developers.cloudflare.com/1.1.1.1/dns-over-https) and update DNS records accordingly.
> - `cloudflare.trace`\
>   Get the public IP address by parsing the [Cloudflare debugging page](https://one.one.one.one/cdn-cgi/trace) and update DNS records accordingly. This is the default provider.
> - `local`\
>   Get the address via local network interfaces and update DNS records accordingly. When multiple local network interfaces or in general multiple IP addresses are present, the updater will use the address that would have been used for outbound UDP connections to Cloudflare servers.
>   ⚠️ You need access to the host network (such as `network_mode: host` in Docker Compose) for this policy, for otherwise the updater will detect the addresses inside the [bridge network in Docker](https://docs.docker.com/network/bridge/) instead of those in the host network.
> - `url:URL`\
>   Fetch the content at a URL via the HTTP(S) protocol as the IP address. The provider format is `url:` followed by the URL. For example, `IP4_PROVIDER=url:https://api4.ipify.org` will fetch the IPv4 addresses from <https://api4.ipify.org>, a server maintained by [ipify](https://www.ipify.org).
>   ⚠️ Currently, the updater _will not_ force IPv4 or IPv6 when retrieving the IPv4 or IPv6 address at the URL, and thus the service must either restrict its access to the correct IP network or return the correct IP address regardless of what IP network is used. As an example, <https://api4.ipify.org> has restricted its access to IPv4. The reason is that there are no elegant ways to force IPv4 or IPv6 using the Go standard library; please [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new) if you have a use case so that I might add some ugly hack to force it.
> - `none`\
>   Stop the DNS updating completely. Existing DNS records will not be removed.
>
> The option `IP4_PROVIDER` is governing IPv4 addresses and `A`-type records, while the option `IP6_PROVIDER` is governing IPv6 addresses and `AAAA`-type records. The two options act independently of each other; that is, you can specify different address providers for IPv4 and IPv6.
>
> Some technical details: For the providers `cloudflare.doh` and `cloudflare.trace`, the updater will connect to the servers `1.1.1.1` for IPv4 and `2606:4700:4700::1111` for IPv6. Since version 1.9.3, the updater will switch to `1.0.0.1` for IPv4 if `1.1.1.1` appears to be blocked or intercepted by your ISP or your router (which is still not uncommon).
>
> </details>

> <details>
> <summary>🃏 What are wildcard domains?</summary>
>
> Wildcard domains (`*.example.org`) represent all subdomains that _would not exist otherwise._ Therefore, if you have another subdomain entry `sub.example.org`, the wildcard domain is independent of it, because it only represents the _other_ subdomains which do not have their own entries. Also, you can only have one layer of `*`---`*.*.example.org` would not work.
>
> </details>

</details>

<details>
<summary>⏳ Schedules, triggers, and timeouts</summary>

| Name                | Valid Values                                                                                                                                                                  | Meaning                                                                                                                                                                             | Required? | Default Value                 |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ----------------------------- |
| `CACHE_EXPIRATION`  | Positive time durations with a unit, such as `1h` and `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration)                                             | The expiration of cached Cloudflare API responses                                                                                                                                   | No        | `6h0m0s` (6 hours)            |
| `DELETE_ON_STOP`    | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool)                                                           | Whether managed DNS records should be deleted on exit                                                                                                                               | No        | `false`                       |
| `DETECTION_TIMEOUT` | Positive time durations with a unit, such as `1h` and `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration)                                             | The timeout of each attempt to detect IP addresses                                                                                                                                  | No        | `5s` (5 seconds)              |
| `TZ`                | Recognized timezones, such as `UTC`                                                                                                                                           | The timezone used for logging and parsing `UPDATE_CRON`                                                                                                                             | No        | `UTC`                         |
| `UPDATE_CRON`       | Cron expressions or the special value `@once`. See the [documentation of cron](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format) for cron expressions. | The schedule to re-check IP addresses and update DNS records (if necessary). The special value `@once` means the updater will terminate immediately after updating the DNS records. | No        | `@every 5m` (every 5 minutes) |
| `UPDATE_ON_START`   | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool)                                                           | Whether to check IP addresses on start regardless of `UPDATE_CRON`                                                                                                                  | No        | `true`                        |
| `UPDATE_TIMEOUT`    | Positive time durations with a unit, such as `1h` and `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration)                                             | The timeout of each attempt to update DNS records, per domain, per record type                                                                                                      | No        | `30s` (30 seconds)            |

> ⚠️ The update schedule _does not_ take the time to update records into consideration. For example, if the schedule is “for every 5 minutes”, and if the updating itself takes 2 minutes, then the actual interval between adjacent updates is 3 minutes, not 5 minutes.

</details>

<details>
<summary>🐣 Parameters of new DNS records</summary>

> 👉 The updater will preserve existing record parameters (TTL, proxy states, comments, etc.) unless it has to create new DNS records (or recreate deleted ones). Only when it creates DNS records, the following settings will apply. To change existing record parameters now, you can go to your [Cloudflare Dashboard](https://dash.cloudflare.com) and change them directly. If you think you have a use case where the updater should actively overwrite existing record parameters in addition to IP addresses, please [let me know](https://github.com/favonia/cloudflare-ddns/issues/new).

| Name             | Valid Values                                                                                                                                                                             | Meaning                                                                                                                                    | Required? | Default Value                              |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | --------- | ------------------------------------------ |
| `PROXIED`        | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool). 🧪 See below for experimental support of per-domain proxy settings. | Whether new DNS records should be proxied by Cloudflare                                                                                    | No        | `false`                                    |
| `TTL`            | Time-to-live (TTL) values in seconds                                                                                                                                                     | The TTL values used to create new DNS records                                                                                              | No        | `1` (This means “automatic” to Cloudflare) |
| `RECORD_COMMENT` | Strings (that consist of only [Unicode graphic characters](https://en.wikipedia.org/wiki/Graphic_character))                                                                             | The [record comment](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) used to create new DNS records | No        | `""`                                       |

> <details>
> <summary>🧪 Experimental per-domain proxy settings (subject to changes):</summary>
>
> The `PROXIED` can be a boolean expression. Here are some examples:
>
> - `PROXIED=is(example.org)`: proxy only the domain `example.org`
> - `PROXIED=is(example1.org) || sub(example2.org)`: proxy only the domain `example1.org` and subdomains of `example2.org`
> - `PROXIED=!is(example.org)`: proxy every managed domain _except for_ `example.org`
> - `PROXIED=is(example1.org) || is(example2.org) || is(example3.org)`: proxy only the domains `example1.org`, `example2.org`, and `example3.org`
>
> A boolean expression has one of the following forms (all whitespace is ignored):
>
> - A boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `t` as `true` or `FALSE` as `false`.
> - `is(d)` which matches the domain `d`. Note that `is(*.a)` only matches the wildcard domain `*.a`; use `sub(a)` to match all subdomains of `a` (including `*.a`).
> - `sub(d)` which matches subdomains of `d`, such as `a.d` and `b.d`. It does not match the domain `d` itself.
> - `! e` where `e` is a boolean expression, representing logical negation of `e`.
> - `e1 || e2` where `e1` and `e2` are boolean expressions, representing logical disjunction of `e1` and `e2`.
> - `e1 && e2` where `e1` and `e2` are boolean expressions, representing logical conjunction of `e1` and `e2`.
>
> One can use parentheses to group expressions, such as `!(is(a) && (is(b) || is(c)))`.
> For convenience, the engine also accepts these short forms:
>
> - `is(d1, d2, ..., dn)` is `is(d1) || is(d2) || ... || is(dn)`
> - `sub(d1, d2, ..., dn)` is `sub(d1) || sub(d2) || ... || sub(dn)`
>
> For example, these two settings are equivalent:
>
> - `PROXYD=is(example1.org) || is(example2.org) || is(example3.org)`
> - `PROXIED=is(example1.org,example2.org,example3.org)`
> </details>

</details>

<details>
<summary>👁️ Logging, Healthchecks, Uptime Kuma, and shoutrrr</summary>

| Name           | Valid Values                                                                                                                                                      | Meaning                                                                                                                                                                                         | Required? | Default Value |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ------------- |
| `EMOJI`        | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool)                                               | Whether the updater should use emojis in the logging                                                                                                                                            | No        | `true`        |
| `HEALTHCHECKS` | [Healthchecks ping URLs](https://healthchecks.io/docs/), such as `https://hc-ping.com/<uuid>` or `https://hc-ping.com/<project-ping-key>/<name-slug>` (see below) | If set, the updater will ping the URL when it successfully updates IP addresses                                                                                                                 | No        | (unset)       |
| `QUIET`        | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool)                                               | Whether the updater should reduce the logging                                                                                                                                                   | No        | `false`       |
| `UPTIMEKUMA`   | Uptime Kuma’s Push URLs, such as `https://<host>/push/<id>`. For convenience, you can directly copy the ‘Push URL’ from the Uptime Kuma configuration page.       | If set, the updater will ping the URL when it successfully updates IP addresses. ⚠️ Remember to change the “Heartbeat Interval” to match your DNS updating schedule specified in `UPDATE_CRON`. | No        | (unset)       |
| 🧪 `SHOUTRRR`  | 🧪 Newline-separated [shoutrrr URLs](https://containrrr.dev/shoutrrr/) such as `discord://<token>@<id>`                                                           | 🧪 If set, the updater will send messages when it updates IP addresses                                                                                                                          | No        | (unset)       |

> 🩺 For `HEALTHCHECKS`, the updater can work with any server following the [same notification protocol](https://healthchecks.io/docs/http_api/), including but not limited to self-hosted instances of [Healthchecks](https://github.com/healthchecks/healthchecks). Both UUID and Slug URLs are supported, and the updater works regardless whether the POST-only mode is enabled.

> ⚠️ If using Healthchecks or Uptime Kuma, please note that a failure of IPv6 would be reported as _down_ even if IPv4 records are updated successfully (and similarly if IPv6 works but IPv4 fails). If your setup does not support IPv6, please add `IP6_PROVIDER=none` to disable IPv6 completely.

</details>

### 🔂 Restarting the Container

If you are using Docker Compose, run `docker-compose up --detach` to reload settings.

## 🚵 Migration Guides

_(Click to expand the following items.)_

<details>
<summary>I am migrating from oznu/cloudflare-ddns (now archived)</summary>

⚠️ [oznu/cloudflare-ddns](https://github.com/oznu/docker-cloudflare-ddns) relies on the insecure DNS protocol to obtain public IP addresses; a malicious hacker could more easily forge DNS responses and trick it into updating your domain with any IP address. In comparison, we use only verified responses from Cloudflare, which makes the attack much more difficult. See the [design document](docs/DESIGN.markdown) for more information on security.

| Old Parameter                          |     | Note                                                                               |
| -------------------------------------- | --- | ---------------------------------------------------------------------------------- |
| `API_KEY=key`                          | ✔️  | Use `CF_API_TOKEN=key`                                                             |
| `API_KEY_FILE=file`                    | ✔️  | Use `CF_API_TOKEN_FILE=file`                                                       |
| `ZONE=example.org` and `SUBDOMAIN=sub` | ✔️  | Use `DOMAINS=sub.example.org` directly                                             |
| `PROXIED=true`                         | ✔️  | Same (`PROXIED=true`)                                                              |
| `RRTYPE=A`                             | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP6_PROVIDER=none` to disable IPv6 |
| `RRTYPE=AAAA`                          | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP4_PROVIDER=none` to disable IPv4 |
| `DELETE_ON_STOP=true`                  | ✔️  | Same (`DELETE_ON_STOP=true`)                                                       |
| `INTERFACE=iface`                      | ✔️  | Not required for `local` providers; we can handle multiple network interfaces      |
| `CUSTOM_LOOKUP_CMD=cmd`                | ❌  | There are no shells in the minimal Docker image                                    |
| `DNS_SERVER=server`                    | ❌  | Only Cloudflare is supported, except the `url:URL` provider via HTTP(S)            |

</details>

<details>
<summary>I am migrating from timothymiller/cloudflare-ddns</summary>

| Old JSON Key                          |     | Note                                                                                                                                                                                                                                     |
| ------------------------------------- | --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloudflare.authentication.api_token` | ✔️  | Use `CF_API_TOKEN=key`                                                                                                                                                                                                                   |
| `cloudflare.authentication.api_key`   | ❌  | Please use the newer, more secure [API tokens](https://dash.cloudflare.com/profile/api-tokens)                                                                                                                                           |
| `cloudflare.zone_id`                  | ✔️  | Not needed; automatically retrieved from the server                                                                                                                                                                                      |
| `cloudflare.subdomains[].name`        | ✔️  | Use `DOMAINS` with [**fully qualified domain names (FQDNs)**](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) directly; for example, if your zone is `example.org` and your subdomain is `sub`, use `DOMAINS=sub.example.org` |
| `cloudflare.subdomains[].proxied`     | 🧪  | _(experimental)_ Write boolean expressions for `PROXIED` to specify per-domain settings; see above for the detailed documentation for this experimental feature                                                                          |
| `load_balancer`                       | ❌  | Not supported yet; please [make a request](https://github.com/favonia/cloudflare-ddns/issues/new) if you want it                                                                                                                         |
| `a`                                   | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP4_PROVIDER=none` to disable IPv4                                                                                                                                                       |
| `aaaa`                                | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP6_PROVIDER=none` to disable IPv6                                                                                                                                                       |
| `proxied`                             | ✔️  | Use `PROXIED=true` or `PROXIED=false`                                                                                                                                                                                                    |
| `purgeUnknownRecords`                 | ❌  | The updater never deletes unmanaged DNS records                                                                                                                                                                                          |

> This updater was originally written as a Go clone of the Python program [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) because the Python code always purged unmanaged DNS records back then and it was not configurable via environment variables. There were feature requests to address these issues but they seemed to be neglected by its author [timothymiller](https://github.com/timothymiller/); I thus made my clone after unsuccessful communications. Understandably, [timothymiller](https://github.com/timothymiller/) did not seem happy with my cloning and my other critical comments. [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) eventually provided an option `purgeUnknownRecords` to disable the unwanted purging, but this updater already went on its way. I believe my Go clone is now much improved and enhanced, but my opinions are biased and you should check the technical details by yourself.

</details>

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new).

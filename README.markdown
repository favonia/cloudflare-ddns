# 🌟 Cloudflare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![Go Reference](https://pkg.go.dev/badge/github.com/favonia/cloudflare-ddns/.svg)](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/)
[![Codecov](https://img.shields.io/codecov/c/github/favonia/cloudflare-ddns)](https://app.codecov.io/gh/favonia/cloudflare-ddns)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)
[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/6680/badge)](https://bestpractices.coreinfrastructure.org/projects/6680)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/favonia/cloudflare-ddns/badge)](https://securityscorecards.dev/viewer/?uri=github.com/favonia/cloudflare-ddns)

A feature-rich and robust Cloudflare DDNS updater with a small footprint. The program will detect your machine’s public IP addresses and update DNS records using the Cloudflare API.

## 📜 Highlights

### ⚡ Efficiency

- 🤏 The Docker image takes less than 5 MB after compression.
- 🔁 The Go runtime re-uses existing HTTP connections.
- 🗃️ Cloudflare API responses are cached to reduce the API usage.

### 💯 Complete Support of Domain Names

- 😌 You can simply list domains (_e.g._, `www.a.org, hello.io`) without knowing their DNS zones.
- 🌍 [Internationalized domain names](https://en.wikipedia.org/wiki/Internationalized_domain_name) (_e.g._, `🐱.example.org` and `日本｡co｡jp`) are fully supported.
- 🃏 [Wildcard domains](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (_e.g._, `*.example.org`) are also supported.
- 🕹️ You can toggle IPv4 (`A` records) and IPv6 (`AAAA` records) for each domain.

### 🌥️ Cloudflare-specific Features

- 😶‍🌫️ You can toggle [Cloudflare proxying](https://developers.cloudflare.com/dns/manage-dns-records/reference/proxied-dns-records/) for each domain.
- 📝 You can set [comments](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) for DNS records managed by the updater.
- 📜 The updater can maintain [lists](https://developers.cloudflare.com/waf/tools/lists/custom-lists/) of detected IP addresses. These lists can then be referenced in any Cloudflare product that uses [Cloudflare’s Rules language](https://developers.cloudflare.com/ruleset-engine/), such as [Cloudflare Web Application Firewall (WAF)](https://developers.cloudflare.com/waf/) and [Cloudflare Rules](https://developers.cloudflare.com/rules/). (We call the lists “WAF lists”, but their use is not limited to Cloudflare WAF.)

### 👁️ Integration with Notification Services

- 🩺 The updater can report to [Healthchecks](https://healthchecks.io) or [Uptime Kuma](https://uptime.kuma.pet) so that you receive notifications when it fails to update IP addresses.
- 📣 The updater can also actively update you via any service supported by the [shoutrrr library](https://containrrr.dev/shoutrrr/), including emails, major notification services, major messaging platforms, and generic webhooks.

### 🕵️ Minimum Privacy Impact

By default, public IP addresses are obtained via [Cloudflare’s debugging page](https://one.one.one.one/cdn-cgi/trace). This minimizes the impact on privacy because we are already using the Cloudflare API to update DNS records. Moreover, if Cloudflare servers are not reachable, chances are you cannot update DNS records anyways.

### 🛡️ Attention to Security

- 🛡️ The updater uses only HTTPS or [DNS over HTTPS](https://en.wikipedia.org/wiki/DNS_over_HTTPS) to detect IP addresses. This makes it harder for someone else to trick the updater into updating your DNS records with wrong IP addresses. See the [Security Model](docs/designs/network-security-model.markdown) for more information.
- <details><summary><em>Click to expand:</em> ✍️ You can verify the Docker images were built from this repository using the cosign tool.</summary>

  ```bash
  cosign verify favonia/cloudflare-ddns:latest \
    --certificate-identity-regexp https://github.com/favonia/cloudflare-ddns/ \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com
  ```

  Note: this only proves that the Docker image is from this repository, assuming that no one hacks into GitHub or the repository. It does not prove that the code itself is secure.

- <details><summary><em>Click to expand:</em> 📚 The updater uses only established open-source Go libraries.</summary>
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

<details><summary><em>Click to expand:</em> 🐋 Directly run the Docker image</summary>

```bash
docker run \
  --network host \
  -e CLOUDFLARE_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  -e DOMAINS=example.org,www.example.org,example.io \
  -e PROXIED=true \
  favonia/cloudflare-ddns:latest
```

</details>

<details><summary><em>Click to expand:</em> 🧬 Directly run the updater from its source</summary>

You need the [Go tool](https://golang.org/doc/install) to run the updater from its source.

```bash
CLOUDFLARE_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  DOMAINS=example.org,www.example.org,example.io \
  PROXIED=true \
  go run github.com/favonia/cloudflare-ddns/cmd/ddns@latest
```

</details>

## 🏁 Deployment as a System Service

See [community-contributed sample configurations](./contrib/README.markdown) for OpenBSD.

## 🐋 Deployment with Docker Compose

### 📦 Step 1: Updating the Compose File

Incorporate the following fragment into the compose file (typically `docker-compose.yml` or `docker-compose.yaml`). The template may look a bit scary, but only because it includes various optional flags for extra security protection.

```yaml
services:
  cloudflare-ddns:
    image: favonia/cloudflare-ddns:latest
    # Choose the appropriate tag based on your need:
    # - "latest" for the latest stable version (which could become 2.x.y
    #   in the future and break things)
    # - "1" for the latest stable version whose major version is 1
    # - "1.x.y" to pin the specific version 1.x.y
    network_mode: host
    # This bypasses network isolation and makes IPv6 easier (optional; see below)
    restart: always
    # Restart the updater after reboot
    user: "1000:1000"
    # Run the updater with specific user and group IDs (in that order).
    # You can change the two numbers based on your need.
    read_only: true
    # Make the container filesystem read-only (optional but recommended)
    cap_drop: [all]
    # Drop all Linux capabilities (optional but recommended)
    security_opt: [no-new-privileges:true]
    # Another protection to restrict superuser privileges (optional but recommended)
    environment:
      - CLOUDFLARE_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
        # Your Cloudflare API token
      - DOMAINS=example.org,www.example.org,example.io
        # Your domains (separated by commas)
      - PROXIED=true
        # Tell Cloudflare to cache webpages and hide your IP (optional)
#networks:
#  LAN0:
#    external: true
#    name: LAN0
# Introduce custom Docker networks to the 'services' in this file. A common use case
# for this is binding one of the 'services' to a specific network interface available at
# Docker's host. This section is required for the 'networks' section of each 'services'.
```

<details>
<summary id="generate-scoped-api-token-minimal-setup"><em>Click to expand:</em> 🔑 <code>CLOUDFLARE_API_TOKEN</code> is your Cloudflare API token</summary>

The value of `CLOUDFLARE_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Use the **Edit zone DNS** template to create a token. The less secure API key authentication is deliberately _not_ supported.

There is an optional feature (available since version 1.14.0) that lets you maintain a [WAF list](https://developers.cloudflare.com/waf/tools/lists/custom-lists/) of detected IP addresses. To use this feature, edit the token and grant it the **Account - Account Filter Lists - Edit** permission. If you only need to update WAF lists, not DNS records, you can remove the **Zone - DNS - Edit** permission. Refer to the detailed documentation below for information on updating WAF lists.

</details>

<details>
<summary><em>Click to expand:</em> 📍 <code>DOMAINS</code> is the list of domains to update</summary>

The value of `DOMAINS` should be a list of [fully qualified domain names (FQDNs)](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs the updater to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to share the same DNS zone---the updater will take care of the DNS zones behind the scene.

</details>

<details>
<summary><em>Click to expand:</em> 🚨 Remove <code>PROXIED=true</code> if you are <em>not</em> running a web server</summary>

The setting `PROXIED=true` instructs Cloudflare to cache webpages and hide your IP addresses. If you wish to bypass that and expose your actual IP addresses, remove `PROXIED=true`. If your traffic is not HTTP(S), then Cloudflare cannot proxy it and you should probably turn off the proxying by removing `PROXIED=true`. The default value of `PROXIED` is `false`.

</details>

<details>
<summary><em>Click to expand:</em> 📴 Add <code>IP6_PROVIDER=none</code> if you want to disable IPv6 completely</summary>

The updater, by default, will attempt to update DNS records for both IPv4 and IPv6, and there is no harm in leaving the automatic detection on even if your network does not work for one of them. However, if you want to disable IPv6 entirely (perhaps to avoid seeing the detection errors), add `IP6_PROVIDER=none`.

</details>

<details>
<summary><em>Click to expand:</em> 📡 Use IPv6 without bypassing network isolation (without <code>network_mode: host</code>)</summary>

The easiest way to enable IPv6 is to use `network_mode: host` so that the updater can access the host IPv6 network directly. This has the downside of bypassing the network isolation. If you wish to keep the updater isolated from the host network, remove `network_mode: host` and follow the steps in the [official Docker documentation to enable IPv6](https://docs.docker.com/config/daemon/ipv6/). Do use newer versions of Docker that come with much better IPv6 support!

</details>

<details>
<summary><em>Click to expand:</em> 🛜 Bind to a specific network interface for updates</summary>

📜 This method uses a MacVLAN sub-device to bind to a specific network interface and may bypass your custom `iptables` and `nftables` configurations.

To be able to use a specific network interface when detecting the IP in the DDNS updates, the following Docker network must be created before running a Docker container with a custom network:

```bash
docker network create
    -d macvlan
    -o parent=eth0 # host network interface name to bind to
    --subnet=192.168.1.0/24 # IP space for running containers within this network
    --gateway=192.168.1.1 # IP address of the gateway/router
    --ip-range=192.168.1.128/25 # communication IP range for containers in this network
    LAN0 # name that will be used in the docker-compose.yml
```

Once the new Docker network is created, add the following to the Docker Compose that will start the `cloudflare-ddns` service. This enforces all requests from this service to go through the mentioned network, e.g. 'LAN0'.

```yaml
networks:
  LAN0:
  #  ipv4_address: 192.168.1.131 # A static IP within subnet (line can be removed for a random IP)
```

If a static IP is preferred, an `ipv4_address` section like the example can be added. NOTE: this IP must be within the `--subnet` of the Docker network.

</details>

<details>
<summary><em>Click to expand:</em> 🛡️ Change <code>user: "1000:1000"</code> to the user and group IDs you want to use</summary>

Change `1000:1000` to `USER:GROUP` for the `USER` and `GROUP` IDs you wish to use to run the updater. The settings `cap_drop`, `read_only`, and `no-new-privileges` in the template provide additional protection, especially when you run the container as a non-superuser.

</details>

### 🚀 Step 2: Building and Running the Container

```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```

## ❓ Frequently Asked Questions

<details>
<summary><em>Click to expand:</em> ❔ I simulated an IP address change by editing the DNS records, but the updater never picked it up!</summary>

Please rest assured that the updater is working as expected. **It will update the DNS records _immediately_ for a real IP change.** Here is a detailed explanation. There are two causes of an IP mismatch:

1. A change of your actual IP address (a _real_ change), or
2. A change of the IP address in the DNS records (a _simulated_ change).

The updater assumes no one will actively change the DNS records. In other words, it assumes simulated changes will not happen. It thus caches the DNS records and cannot detect your simulated changes. However, when your actual IP address changes, the updater will immediately update the DNS records. Also, the updater will eventually check the DNS records and detect simulated changes after `CACHE_EXPIRATION` (six hours by default) has passed.

If you really wish to test the updater with simulated IP changes in the DNS records, you can set `CACHE_EXPIRATION=1ns` (all cache expiring in one nanosecond), effectively disabling the caching. However, it is recommended to keep the default value (six hours) to reduce your network traffic.

</details>

<details>
<summary><em>Click to expand:</em> ❔ How can I see the timestamps of the IP checks and/or updates?</summary>

The updater does not itself add timestamps because all major systems already timestamp everything:

- If you are using Docker Compose, Kubernetes, or Docker directly, add the option `--timestamps` when viewing the logs.
- If you are using Portainer, [enable “Show timestamp” when viewing the logs](https://docs.portainer.io/user/docker/containers/logs).

</details>

<details>
<summary><em>Click to expand:</em> ❔ Why did the updater detect a public IP address different from the WAN IP address on my router?</summary>

Is your “public” IP address on your router between `100.64.0.0` and `100.127.255.255`? If so, you are within your ISP’s [CGNAT (Carrier-grade NAT)](https://en.wikipedia.org/wiki/Carrier-grade_NAT). In practice, there is no way for DDNS to work with CGNAT, because your ISP does not give you a real public IP address, nor does it allow you to forward IP packages to your router using cool protocols such as [Port Control Protocol](https://en.wikipedia.org/wiki/Port_Control_Protocol). You have to give up DDNS or switch to another ISP. You may consider other services such as [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) that can work around CGNAT.

</details>

<details>
<summary><em>Click to expand:</em> ❔ How should I install this updater in ☸️ Kubernetes?</summary>

Due to high maintenance costs, the Kubernetes instructions have been removed. However, you can still generate Kubernetes configurations from the provided Docker Compose template using a conversion tool like [Kompose](https://kompose.io/). **Important:** Only use Kompose version 1.35.0 or later, as these versions support the `user: "UID:GID"` attribute with `:GID`.

Note that a simple [Kubernetes Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) will suffice here. Since there’s no inbound network traffic, a [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service/) isn’t required.

</details>

<details>
<summary><em>Click to expand:</em> ❔ Help! I got <code>exec /bin/ddns: operation not permitted</code></summary>

Certain Docker installations may have issues with the `no-new-privileges` security option. If you cannot run Docker images with this option (including this updater), removing it might be necessary. This will slightly compromise security, but it’s better than not running the updater at all. If _only_ this updater is affected, please [report this issue on GitHub](https://github.com/favonia/cloudflare-ddns/issues/new).

</details>

<details>
<summary><em>Click to expand:</em> ❔ I am getting <code>error code: 1034</code></summary>

We have received reports of recent issues with the default IP provider, `cloudflare.trace`. Some users are encountering an "error code: 1034," likely due to internal problems with Cloudflare's servers. To work around this, please upgrade the updater to version 1.15.1 or later. Alternatively, you may switch to a different IP provider.

</details>

## 🎛️ Further Customization

### ⚙️ All Settings

The emoji “🧪” indicates experimental features and the emoji “🤖” indicates technical details that most users can safely ignore.

<details>
<summary id="generate-scoped-api-token-all-settings"><em>Click to expand:</em> 🔑 Cloudflare API Access</summary>

> Starting with version 1.15.0, the updater supports environment variables that begin with `CLOUDFLARE_*`. Multiple environment variables can be used at the same time, provided they all specify the same token.

| Name                                                      | Meaning                                                                                                                                |
| --------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `CLOUDFLARE_API_TOKEN`                                    | The [Cloudflare API token](https://dash.cloudflare.com/profile/api-tokens) to access the Cloudflare API                                |
| `CLOUDFLARE_API_TOKEN_FILE`                               | A path to a file that contains the [Cloudflare API token](https://dash.cloudflare.com/profile/api-tokens) to access the Cloudflare API |
| `CF_API_TOKEN` (will be deprecated in version 2.0.0)      | Same as `CLOUDFLARE_API_TOKEN`                                                                                                         |
| `CF_API_TOKEN_FILE` (will be deprecated version in 2.0.0) | Same as `CLOUDFLARE_API_TOKEN_FILE`                                                                                                    |

> 🚂 Cloudflare is updating its tools to use environment variables starting with `CLOUDFLARE_*` instead of `CF_*`. It is recommended to align your setting with this new convention. However, the updater will fully support both `CLOUDFLARE_*` and `CF_*` environment variables until version 2.0.0.
>
> 🔑 To update DNS records, the updater needs the **Zone - DNS - Edit** permission.
>
> 🔑 To manipulate WAF lists, the updater needs the **Account - Account Filter Lists - Edit** permission.
>
> 💡 `CLOUDFLARE_API_TOKEN_FILE` works well with [Docker secrets](https://docs.docker.com/compose/how-tos/use-secrets/) where secrets will be mounted as files at `/run/secrets/<secret-name>`.

</details>

<details>
<summary><em>Click to expand:</em> 📍 DNS and WAF Scope</summary>

> You need to specify at least one thing in `DOMAINS`, `IP4_DOMAINS`, `IP6_DOMAINS`, or 🧪 `WAF_LISTS` (since version 1.14.0) for the updater to update.

Managed DNS records:

| Name                                         | Meaning                                                                                                                                                                                                                                               | Default Value                               |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `DOMAINS`                                    | Comma-separated fully qualified domain names or wildcard domain names that the updater should manage for both `A` and `AAAA` records. Listing a domain in `DOMAINS` is equivalent to listing the same domain in both `IP4_DOMAINS` and `IP6_DOMAINS`. | `""` (empty list)                           |
| `IP4_DOMAINS`                                | Comma-separated fully qualified domain names or wildcard domain names that the updater should manage for `A` records                                                                                                                                  | `""` (empty list)                           |
| `IP6_DOMAINS`                                | Comma-separated fully qualified domain names or wildcard domain names that the updater should manage for `AAAA` records                                                                                                                               | `""` (empty list)                           |
| `MANAGED_RECORDS_COMMENT_REGEX` (unreleased) | Regex that matches comments of existing DNS records this updater manages. Only records whose comments match are updated or deleted. Uses [RE2](https://github.com/google/re2/wiki/Syntax) syntax (not Perl/PCRE).                                     | `""` (empty regex; manages all DNS records) |

Managed WAF lists:

| Name                                                   | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Default Value                                  |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| 🧪 `WAF_LISTS` (since version 1.14.0)                  | <p>🧪 Comma-separated references of [WAF lists](https://developers.cloudflare.com/waf/tools/lists/custom-lists/) the updater should manage. A list reference is written in the format `<account-id>/<list-name>` where `account-id` is your account ID and `list-name` is the list name; it should look like `0123456789abcdef0123456789abcdef/mylist`. If the referenced WAF list does not exist, the updater will try to create it.</p><p>🔑 The API token needs the **Account - Account Filter Lists - Edit** permission.<br/>💡 See [how to find your account ID](https://developers.cloudflare.com/fundamentals/setup/find-account-and-zone-ids/).</p>            | `""` (empty list)                              |
| 🧪 `WAF_LIST_ITEM_COMMENT` (unreleased)                | 🧪 Default comment for new WAF list items.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `""`                                           |
| 🧪 `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` (unreleased) | 🧪 Regex that matches comments of existing WAF list items this updater manages. Only items whose comments match are managed by this updater and may be deleted during reconciliation or shutdown cleanup. Cloudflare does not provide an API to edit a single WAF list item in place. During reconciliation, when the desired IP/range set changes, the updater adds missing items and removes stale items. Uses [RE2](https://github.com/google/re2/wiki/Syntax) syntax (not Perl/PCRE). With `DELETE_ON_STOP=true`, a non-empty regex prevents whole-list deletion and limits shutdown cleanup to matched items.                         | `""` (empty regex; manages all WAF list items) |

> 🤖 **Advanced setup for multi-instance shared domains/WAF lists**
>
> Use this setup when multiple updater instances overlap on DNS domains or share WAF lists, and each instance should manage only its own items.
>
> 1. Give each instance a unique `RECORD_COMMENT`.
> 2. Set `MANAGED_RECORDS_COMMENT_REGEX` to the same DNS comment (typically with `^...$`).
> 3. 🧪 If instances may touch the same WAF list, give each instance a unique `WAF_LIST_ITEM_COMMENT`.
> 4. 🧪 Set `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` to the same WAF item comment (typically with `^...$`).
>
> Example:
>
> - Instance A: `RECORD_COMMENT=managed-by-ddns-a`, `MANAGED_RECORDS_COMMENT_REGEX=^managed-by-ddns-a$`, 🧪 `WAF_LIST_ITEM_COMMENT=managed-by-ddns-a`, 🧪 `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=^managed-by-ddns-a$`
> - Instance B: `RECORD_COMMENT=managed-by-ddns-b`, `MANAGED_RECORDS_COMMENT_REGEX=^managed-by-ddns-b$`, 🧪 `WAF_LIST_ITEM_COMMENT=managed-by-ddns-b`, 🧪 `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=^managed-by-ddns-b$`
>
> `RECORD_COMMENT` must match `MANAGED_RECORDS_COMMENT_REGEX`, and 🧪 `WAF_LIST_ITEM_COMMENT` must match 🧪 `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`; otherwise the updater fails at startup.
>
> `DELETE_ON_STOP=true` always deletes managed DNS records. 🧪 For WAF lists, a non-empty `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` keeps the list and deletes only items managed by this updater.

Other scope notes:

> 🃏🤖 **Wildcard domains** (`*.example.org`) represent all subdomains that _would not exist otherwise._ Therefore, if you have another subdomain entry `sub.example.org`, the wildcard domain is independent of it, because it only represents the _other_ subdomains which do not have their own entries. Also, you can only have one layer of `*`---`*.*.example.org` would not work.

> 🌐🤖 **Internationalized domain names** are handled using the _nontransitional processing_ (fully compatible with IDNA2008). At this point, all major browsers and whatnot have switched to the same nontransitional processing. See [this useful FAQ on internationalized domain names](https://www.unicode.org/faq/idn.html).

> 🧪🤖 WAF list reconciliation notes:
>
> 1. [Cloudflare does not allow single IPv6 addresses in a WAF list](https://developers.cloudflare.com/waf/tools/lists/custom-lists/#lists-with-ip-addresses-ip-lists), and thus the updater will use the smallest IP range allowed by Cloudflare that contains the detected IPv6 address.
> 2. The updater removes addresses from disabled IP families only from items managed by this updater (_e.g.,_ if you set `IP6_PROVIDER=none`, existing IPv6 addresses or ranges in managed items are deleted). The managed set is selected by `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`.
> 3. Cloudflare does not provide an API to edit a single WAF list item in place. When the desired IP/range set changes, reconciliation adds missing items and removes stale items. For newly created items, the updater inherits stale managed comments when they agree; otherwise it uses `WAF_LIST_ITEM_COMMENT`.

</details>

<details>
<summary><em>Click to expand:</em> 🔍 IP Detection</summary>

| Name           | Meaning                                                                                                                                                                                                                                                                                                  | Default Value      |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `IP4_PROVIDER` | This specifies how to detect the current IPv4 address. Available providers include `cloudflare.trace`, `cloudflare.doh`, `local`, `local.iface:<iface>`, `url:<url>`, `literal:<ip1>,<ip2>,...`, and `none`. The special `none` provider disables IPv4 completely. See below for a detailed explanation. | `cloudflare.trace` |
| `IP6_PROVIDER` | This specifies how to detect the current IPv6 address. Available providers include `cloudflare.trace`, `cloudflare.doh`, `local`, `local.iface:<iface>`, `url:<url>`, `literal:<ip1>,<ip2>,...`, and `none`. The special `none` provider disables IPv6 completely. See below for a detailed explanation. | `cloudflare.trace` |

> 👉 The option `IP4_PROVIDER` governs `A`-type DNS records and IPv4 addresses in WAF lists, while the option `IP6_PROVIDER` governs `AAAA`-type DNS records and IPv6 addresses in WAF lists. The two options act independently of each other. You can specify different address providers for IPv4 and IPv6.

| Provider Name                                                                                 | Explanation                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloudflare.trace`                                                                            | Get the IP address by parsing the [Cloudflare debugging page](https://api.cloudflare.com/cdn-cgi/trace). **This is the default provider.**                                                                                                                                                                                                                                                                                                                                                                                                  |
| `cloudflare.doh`                                                                              | Get the IP address by querying `whoami.cloudflare.` against [Cloudflare via DNS-over-HTTPS](https://developers.cloudflare.com/1.1.1.1/dns-over-https).                                                                                                                                                                                                                                                                                                                                                                                      |
| `local`                                                                                       | <p>Get the IP address via local network interfaces and routing tables. The updater will use the local address that _would have_ been used for outbound UDP connections to Cloudflare servers. (No data will be transmitted.)</p><p>⚠️ The updater needs access to the host network (such as `network_mode: host` in Docker Compose) for this provider, for otherwise the updater will detect the addresses inside [the default bridge network in Docker](https://docs.docker.com/network/bridge/) instead of those in the host network.</p> |
| 🧪 `local.iface:<iface>` (available since version 1.15.0) | <p>🧪 Get IP addresses via the specific local network interface `iface`. Since the unreleased version, the updater collects all matching global unicast addresses of the selected IP family (IPv4 or IPv6) instead of just the first one, then reconciles DNS records and WAF lists against that full set.</p><p>⚠️ The updater needs access to the host network (such as `network_mode: host` in Docker Compose) for this provider, for otherwise the updater cannot access host network interfaces.</p>                                                                                                  |
| `url:<url>`                                                                                   | Fetch the IP address from a URL. The provider format is `url:` followed by the URL itself. For example, `IP4_PROVIDER=url:https://api4.ipify.org` will fetch the IPv4 address from <https://api4.ipify.org>. Since version 1.15.0, the updater will enforce the matching protocol (IPv4 or IPv6) when connecting to the provided URL. Currently, only HTTP(S) is supported.                                                                                                                                                                 |
| `literal:<ip1>,<ip2>,...` (unreleased)                                                        | Use one or more explicit IP addresses for detection (handy for tests/debugging). The addresses are parsed, deduplicated, sorted, and validated for the selected IP family via the same normalization pipeline used by other providers.                                                                                                                                                                                                                                                                                                      |
| `none`                                                                                        | <p>Stop the DNS updating for the specified IP version completely. For example `IP4_PROVIDER=none` will disable IPv4 completely. Existing DNS records will not be removed.</p><p>🧪 The IP addresses of the disabled IP version will be removed from WAF lists; so `IP4_PROVIDER=none` will remove all IPv4 addresses from all managed WAF lists. As the support of WAF lists is still experimental, this behavior is subject to changes and please [provide feedback](https://github.com/favonia/cloudflare-ddns/issues/new).</p>           |

</details>

<details>
<summary><em>Click to expand:</em> 📅 Update Schedule and Lifecycle</summary>

| Name               | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Default Value                 |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| `CACHE_EXPIRATION` | The expiration of cached Cloudflare API responses. It can be any positive time duration accepted by [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration), such as `1h` or `10m`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | `6h0m0s` (6 hours)            |
| `DELETE_ON_STOP`   | <p>Whether managed DNS records and managed WAF content are deleted when the updater exits. It accepts any boolean value supported by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1`.</p><p>🧪 For WAF lists, cleanup scope depends on `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX`: with a non-empty selector, shutdown cleanup deletes only matched managed items and keeps the list; with the default empty selector, all list items are managed, so the updater may try deleting each configured list (or clear items when list deletion is blocked by a rule expression).</p>                                                                                   | `false`                       |
| `TZ`               | <p>The timezone used for logging messages and parsing `UPDATE_CRON`. It can be any timezone accepted by [time.LoadLocation](https://pkg.go.dev/time#LoadLocation), including any IANA Time Zone.</p><p>🤖 The pre-built Docker images come with the embedded timezone database via the [time/tzdata](https://pkg.go.dev/time/tzdata) package.</p>                                                                                                                                                                                                                                                                                                                                                              | `UTC`                         |
| `UPDATE_CRON`      | <p>The schedule to re-check IP addresses and update DNS records and WAF lists (if needed). The format is [any cron expression accepted by the `cron` library](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format) or the special value `@once`. The special value `@once` means the updater will terminate immediately after updating the DNS records or WAF lists, effectively disabling the scheduling feature.</p><p>🤖 The update schedule _does not_ take the time to update records into consideration. For example, if the schedule is `@every 5m`, and if the updating itself takes 2 minutes, then the actual interval between adjacent updates is 3 minutes, not 5 minutes.</p> | `@every 5m` (every 5 minutes) |
| `UPDATE_ON_START`  | Whether to check IP addresses (and possibly update DNS records and WAF lists) _immediately_ on start, regardless of the update schedule specified by `UPDATE_CRON`. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0` or `1`.                                                                                                                                                                                                                                                                                                                                                                                                    | `true`                        |

</details>

<details>
<summary><em>Click to expand:</em> ⏳ Operation Timeouts</summary>

| Name                | Meaning                                                                                                                                                                                                                                       | Default Value      |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `DETECTION_TIMEOUT` | The timeout of each attempt to detect IP address, per IP version (IPv4 and IPv6). It can be any positive time duration accepted by [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration), such as `1h` or `10m`.                   | `5s` (5 seconds)   |
| `UPDATE_TIMEOUT`    | The timeout of each attempt to update DNS records, per domain and per record type, or per WAF list. It can be any positive time duration accepted by [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration), such as `1h` or `10m`. | `30s` (30 seconds) |

</details>

<details>
<summary><em>Click to expand:</em> 🐣 DNS and WAF Defaults</summary>

> 👉 These settings are the configured defaults for managed DNS records and WAF content. Some are used only when something new must be created, such as `WAF_LIST_DESCRIPTION`. For DNS record metadata and WAF item comments, the updater reuses existing managed values when they clearly agree; otherwise it uses the default value below.

| Name                                             | Meaning                                                                                                                                                                                                                                                                                                   | Default Value                              |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `PROXIED`                                        | <p>Default proxy setting for DNS records managed by the updater. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0` or `1`.</p><p>🤖 Advanced usage: it can also be a domain-dependent boolean expression as described below.</p> | `false`                                    |
| `TTL`                                            | Default TTL (in seconds) for DNS records managed by the updater.                                                                                                                                                                                                                                                                                | `1` (This means “automatic” to Cloudflare) |
| `RECORD_COMMENT`                                 | Default [record comment](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) for DNS records managed by the updater.                                                                                                                                                                                                      | `""`                                       |
| 🧪 `WAF_LIST_DESCRIPTION` (since version 1.14.0) | 🧪 Default description for new WAF lists.                                                                                                                                                                                                                                                         | `""`                                       |
| 🧪 `WAF_LIST_ITEM_COMMENT` (unreleased)          | 🧪 Default comment for new WAF list items.                                                                                                                                                                                                                                                        | `""`                                       |

> 🤖 For advanced users: the `PROXIED` can be a boolean expression involving domains! This allows you to enable Cloudflare proxying for some domains but not the others. Here are some example expressions:
>
> - `PROXIED=is(example.org)`: proxy only the domain `example.org`
> - `PROXIED=is(example1.org) || sub(example2.org)`: proxy only the domain `example1.org` and subdomains of `example2.org`
> - `PROXIED=!is(example.org)`: proxy every managed domain _except for_ `example.org`
> - `PROXIED=is(example1.org) || is(example2.org) || is(example3.org)`: proxy only the domains `example1.org`, `example2.org`, and `example3.org`
>
> A boolean expression must be one of the following forms (all whitespace is ignored):
>
> | Syntax                                                                                                                 | Meaning                                                                                                                                             |
> | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
> | Any string accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1` | Logical truth or falsehood                                                                                                                          |
> | `is(d)`                                                                                                                | Matching the domain `d`. Note that `is(*.a)` only matches the wildcard domain `*.a`; use `sub(a)` to match all subdomains of `a` (including `*.a`). |
> | `sub(d)`                                                                                                               | Matching subdomains of `d`, such as `a.d`, `b.c.d`, and `*.d`. It does not match the domain `d` itself.                                             |
> | `! e`                                                                                                                  | Logical negation of the boolean expression `e`                                                                                                      |
> | <code>e1 &#124;&#124; e2</code>                                                                                        | Logical disjunction of the boolean expressions `e1` and `e2`                                                                                        |
> | `e1 && e2`                                                                                                             | Logical conjunction of the boolean expressions `e1` and `e2`                                                                                        |
>
> One can use parentheses to group expressions, such as `!(is(a) && (is(b) || is(c)))`. For convenience, the parser also accepts these short forms:
>
> | Short Form             | Equivalent Full Form                                                            |
> | ---------------------- | ------------------------------------------------------------------------------- |
> | `is(d1, d2, ..., dn)`  | <code>is(d1) &#124;&#124; is(d2) &#124;&#124; ... &#124;&#124; is(dn)</code>    |
> | `sub(d1, d2, ..., dn)` | <code>sub(d1) &#124;&#124; sub(d2) &#124;&#124; ... &#124;&#124; sub(dn)</code> |
>
> For example, these two settings are equivalent:
>
> - `PROXIED=is(example1.org) || is(example2.org) || is(example3.org)`
> - `PROXIED=is(example1.org,example2.org,example3.org)`
> </details>

</details>

<details>
<summary><em>Click to expand:</em> 👁️ Logging</summary>

| Name    | Meaning                                                                                                                                                                                       | Default Value |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `EMOJI` | Whether the updater should use emojis in the logging. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0` or `1`. | `true`        |
| `QUIET` | Whether the updater should reduce the logging. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0` or `1`.        | `false`       |

</details>

<details>
<summary><em>Click to expand:</em> 📣 Notifications</summary>

> 💡 If your network doesn’t support IPv6, set `IP6_PROVIDER=none` to disable IPv6. This will prevent the updater from reporting failures in detecting IPv6 addresses to monitoring services. Similarly, set `IP4_PROVIDER=none` if your network doesn’t support IPv4.

| Name                                 | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `HEALTHCHECKS`                       | <p>The [Healthchecks ping URL](https://healthchecks.io/docs/) to ping when the updater successfully updates IP addresses, such as `https://hc-ping.com/<uuid>` or `https://hc-ping.com/<project-ping-key>/<name-slug>`</p><p>⚠️ The ping schedule should match the update schedule specified by `UPDATE_CRON`.<br/>🤖 The updater can work with _any_ server following the [same Healthchecks protocol](https://healthchecks.io/docs/http_api/), including self-hosted instances of [Healthchecks](https://github.com/healthchecks/healthchecks). Both UUID and Slug URLs are supported, and the updater works regardless whether the POST-only mode is enabled.</p> |
| `UPTIMEKUMA`                         | <p>The Uptime Kuma’s Push URL to ping when the updater successfully updates IP addresses, such as `https://<host>/push/<id>`. You can directly copy the “Push URL” from the Uptime Kuma configuration page.</p><p>⚠️ The “Heartbeat Interval” should match the update schedule specified by `UPDATE_CRON`.</p>                                                                                                                                                                                                                                                                                                                                                       |
| 🧪 `SHOUTRRR` (since version 1.12.0) | Newline-separated [shoutrrr URLs](https://containrrr.dev/shoutrrr/latest/services/overview/) to which the updater sends notifications of IP address changes and other events. Each shoutrrr URL represents a notification service; for example, `discord://<token>@<id>` means sending messages to Discord.                                                                                                                                                                                                                                                                                                                                                          |

</details>

### 🔂 Restarting the Container

If you are using Docker Compose, run `docker-compose up --detach` to reload settings.

## 🚵 Migration Guides

<details>
<summary><em>Click to expand:</em> I am migrating from oznu/cloudflare-ddns (now archived)</summary>

⚠️ [oznu/cloudflare-ddns](https://github.com/oznu/docker-cloudflare-ddns) relies on the insecure DNS protocol to obtain public IP addresses; a malicious hacker could more easily forge DNS responses and trick it into updating your domain with any IP address. In comparison, we use only verified responses from Cloudflare, which makes the attack much more difficult. See the [network security design note](docs/designs/network-security-model.markdown) for more information.

| Old Parameter                          |     | Note                                                                                                                                                                                                                                                                                                                                            |
| -------------------------------------- | --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `API_KEY=<key>`                        | ⚠️  | Legacy global API keys are not supported. Please [generate a scoped API token](#generate-scoped-api-token-minimal-setup) and use `CLOUDFLARE_API_TOKEN=<token>`.                                                                                                                                                                                |
| `API_KEY_FILE=/path/to/key-file`       | ⚠️  | Legacy global API keys are not supported. Please [generate a scoped API token](#generate-scoped-api-token-minimal-setup), save it, and use `CLOUDFLARE_API_TOKEN_FILE=/path/to/token-file`.                                                                                                                                                     |
| `ZONE=example.org` and `SUBDOMAIN=sub` | ✔️  | Use `DOMAINS=sub.example.org` directly                                                                                                                                                                                                                                                                                                          |
| `PROXIED=true`                         | ✔️  | Same (`PROXIED=true`)                                                                                                                                                                                                                                                                                                                           |
| `RRTYPE=A`                             | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP6_PROVIDER=none` to disable IPv6                                                                                                                                                                                                                                                              |
| `RRTYPE=AAAA`                          | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP4_PROVIDER=none` to disable IPv4                                                                                                                                                                                                                                                              |
| `DELETE_ON_STOP=true`                  | ✔️  | Same (`DELETE_ON_STOP=true`)                                                                                                                                                                                                                                                                                                                    |
| `INTERFACE=<iface>`                    | ✔️  | To automatically select the local address, use `IP4/6_PROVIDER=local`. 🧪 To select addresses of a specific network interface, use `IP4/6_PROVIDER=local.iface:<iface>` (available since version 1.15.0). Since the unreleased version, the updater collects all matching global unicast addresses instead of just the first one, then reconciles DNS records and WAF lists against that full detected set. |
| `CUSTOM_LOOKUP_CMD=cmd`                | ❌  | Custom commands are not supported because there are no other programs in the minimal Docker image                                                                                                                                                                                                                                               |
| `DNS_SERVER=server`                    | ❌  | For DNS-based IP detection, the updater only supports secure DNS queries using Cloudflare’s DNS over HTTPS (DoH) server. To enable this, set `IP4/6_PROVIDER=cloudflare.doh`. To detect IP addresses via HTTPS by querying other servers, use `IP4/6_PROVIDER=url:<url>`                                                                        |

</details>

<details>
<summary><em>Click to expand:</em> I am migrating from timothymiller/cloudflare-ddns</summary>

| Old JSON Key                          |     | Note                                                                                                                                                                                                                                     |
| ------------------------------------- | --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cloudflare.authentication.api_token` | ✔️  | Use `CLOUDFLARE_API_TOKEN=<token>`                                                                                                                                                                                                       |
| `cloudflare.authentication.api_key`   | ⚠️  | Legacy global API keys are not supported. [Generate a scoped API token](#generate-scoped-api-token-all-settings), then follow the row above.                                                                                             |
| `cloudflare.zone_id`                  | ✔️  | Not needed; automatically retrieved from the server                                                                                                                                                                                      |
| `cloudflare.subdomains[].name`        | ✔️  | Use `DOMAINS` with [**fully qualified domain names (FQDNs)**](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) directly; for example, if your zone is `example.org` and your subdomain is `sub`, use `DOMAINS=sub.example.org` |
| `cloudflare.subdomains[].proxied`     | ✔️  | Write boolean expressions for `PROXIED` to specify per-domain settings; see above for the detailed documentation for this advanced feature                                                                                               |
| `load_balancer`                       | ❌  | Not supported yet; please [make a request](https://github.com/favonia/cloudflare-ddns/issues/new) if you want it                                                                                                                         |
| `a`                                   | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP4_PROVIDER=none` to disable IPv4                                                                                                                                                       |
| `aaaa`                                | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP6_PROVIDER=none` to disable IPv6                                                                                                                                                       |
| `proxied`                             | ✔️  | Use `PROXIED=true` or `PROXIED=false`                                                                                                                                                                                                    |
| `purgeUnknownRecords`                 | ❌  | The updater never deletes unmanaged DNS records                                                                                                                                                                                          |

> 📜 Some historical notes: This updater was originally written as a Go clone of the Python program [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) because the Python program purged unmanaged DNS records back then and it was not configurable via environment variables on its default branch. Eventually, an option `purgeUnknownRecords` was added to the Python program to disable purging, and it became configurable via environment variables, but my Go clone had already gone its own way. Beyond the migration points above, there were other issues and discussions that I prefer not to detail here, and some of that context is no longer publicly available. My opinions are biased, so please check the technical details by yourself. 😉

</details>

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new).

## 📜 License

The code is licensed under [Apache 2.0 with LLVM exceptions](./LICENSE). (The LLVM exceptions provide better compatibility with GPL 2.0 and other license exceptions.)

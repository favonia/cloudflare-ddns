# 🌟 Cloudflare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![Codecov](https://img.shields.io/codecov/c/gh/favonia/cloudflare-ddns)](https://app.codecov.io/gh/favonia/cloudflare-ddns)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6680/badge)](https://www.bestpractices.dev/en/projects/6680/passing)
[![OpenSSF Scorecard](https://img.shields.io/ossf-scorecard/github.com/favonia/cloudflare-ddns?label=openssf+scorecard)](https://securityscorecards.dev/viewer/?uri=github.com/favonia/cloudflare-ddns)

A feature-rich and robust Cloudflare DDNS updater with a small Docker image. It detects your machine’s public IP addresses and updates DNS records through the Cloudflare API.

## ✨️ Highlights

### ⚡️ Efficiency

- <img src="https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest?label=" alt="Docker Image Size" align="top"> The default Docker image stays small.
- 🔁 The Go runtime re-uses existing HTTP connections.
- 🗃️ Cloudflare API responses are cached to reduce the API usage.

### ✅️ Comprehensive Support of Domain Names

- 😌 You can simply list domains (_e.g._, `www.a.org, hello.io`) without knowing their DNS zones.
- 🌍️ [Internationalized domain names](https://en.wikipedia.org/wiki/Internationalized_domain_name) (_e.g._, `🐱.example.org` and `日本｡co｡jp`) are fully supported.
- 🃏 [Wildcard domains](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (_e.g._, `*.example.org`) are also supported.
- 🕹️ You can toggle IPv4 (`A` records) and IPv6 (`AAAA` records) for each domain.

### 🌥️ Cloudflare-specific Features

- 📝 The updater preserves existing [Cloudflare proxy statuses](https://developers.cloudflare.com/dns/proxy-status/), [TTLs](https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl/), and [comments](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) for managed DNS records. You can set fallback values for cases where the updater needs to supply them.
- 📜 The updater can maintain [lists](https://developers.cloudflare.com/waf/tools/lists/custom-lists/) of detected IP addresses. These lists can then be referenced in any Cloudflare product that uses [Cloudflare’s Rules language](https://developers.cloudflare.com/ruleset-engine/), such as [Cloudflare Web Application Firewall (WAF)](https://developers.cloudflare.com/waf/) and [Cloudflare Rules](https://developers.cloudflare.com/rules/). (We call the lists “WAF lists”, but their use is not limited to Cloudflare WAF.)

### 🔔 Integration with Notification Services

- 🩺 The updater can report to [Healthchecks](https://healthchecks.io) or [Uptime Kuma](https://uptime.kuma.pet) so that you receive notifications when it fails to update IP addresses.
- 📣 The updater can also actively update you via any service supported by the [shoutrrr library](https://containrrr.dev/shoutrrr/), including emails, major notification services, major messaging platforms, and generic webhooks.

### 📐 Attention to Correctness and Security

- <img src="https://img.shields.io/codecov/c/gh/favonia/cloudflare-ddns?label=" alt="Codecov" align="top"> The testing coverage is high (though the coverage itself doesn’t say much).

- 📚 The updater is guided by detailed and principled [design documents](./docs/designs/README.markdown).

- 🙈 By default, public IP addresses are obtained via [Cloudflare’s debugging page](https://one.one.one.one/cdn-cgi/trace). This minimizes the impact on privacy because we are already using the Cloudflare API to update DNS records.

- 🛡️ By default, the updater uses only HTTPS or [DNS over HTTPS](https://en.wikipedia.org/wiki/DNS_over_HTTPS) to detect IP addresses. This makes it harder for someone else to trick the updater into updating your DNS records with wrong IP addresses. See the [Security Model](docs/designs/features/network-security-model.markdown) for more information.

- <details><summary>🔏 You can verify with cosign that the Docker images were built from this repository <sup><em>click to expand</em></sup></summary>

  ```bash
  cosign verify favonia/cloudflare-ddns:1 \
    --certificate-identity-regexp https://github.com/favonia/cloudflare-ddns/ \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com
  ```

  This only proves that the Docker image is from this repository, assuming that no one hacks into GitHub or the repository. It does not prove that the code itself is secure.

  </details>

- <details><summary>📚️ The updater uses only a small set of established external Go packages <sup><em>click to expand</em></sup></summary>
  <ul>
    <li><a href="https://github.com/cloudflare/cloudflare-go">cloudflare-go</a>: official Go binding of Cloudflare API v4.</li>
    <li><a href="https://github.com/robfig/cron">cron</a>: parsing of Cron expressions.</li>
    <li><a href="https://github.com/hashicorp/go-retryablehttp">go-retryablehttp</a>: HTTP clients with retries and exponential backoff.</li>
    <li><a href="https://github.com/google/go-querystring">go-querystring</a>: library to construct URL query parameters.</li>
    <li><a href="https://github.com/containrrr/shoutrrr">shoutrrr</a>: notification library for sending general updates.</li>
    <li><a href="https://github.com/jellydator/ttlcache">ttlcache</a>: in-memory cache to hold Cloudflare API responses.</li>
    <li><a href="https://pkg.go.dev/golang.org/x/net">x/net</a>: official Go supplementary packages for domain handling and low-level DNS support.</li>
    <li><a href="https://pkg.go.dev/golang.org/x/text">x/text</a>: official Go supplementary packages for locale-aware text handling.</li>
    <li><a href="https://github.com/uber-go/mock">mock</a> (for testing only): semi-official framework for mocking.</li>
    <li><a href="https://github.com/stretchr/testify">testify</a> (for testing only): tool set for testing Go programs.</li>
  </ul>
  </details>

<a id="quick-start"></a>

## 🚀 Quick Start

<details><summary>🐋 Directly run the Docker image <sup><em>click to expand</em></sup></summary>

Create a Cloudflare API token from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens) with the **Zone - DNS - Edit** and **Account - Account Filter Lists - Edit** permissions. You can remove unneeded permissions based on your setup; see [Cloudflare API Tokens](#cloudflare-api-token) for details.

```bash
docker run \
  --network host \
  -e CLOUDFLARE_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  -e DOMAINS=example.org,www.example.org,example.io \
  -e PROXIED=true \
  favonia/cloudflare-ddns:1
```

⚠️ `PROXIED=true` does not change the proxy statuses of existing records. See [DNS and WAF Fallback Values](#dns-and-waf-fallback-values).

</details>

<details><summary>🧬 Directly run the updater from its source <sup><em>click to expand</em></sup></summary>

Create a Cloudflare API token from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens) with the **Zone - DNS - Edit** and **Account - Account Filter Lists - Edit** permissions. You can remove unneeded permissions based on your setup; see [Cloudflare API Tokens](#cloudflare-api-token) for details.

You need the [Go tool](https://go.dev/doc/install) to run the updater from its source.

```bash
CLOUDFLARE_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  DOMAINS=example.org,www.example.org,example.io \
  PROXIED=true \
  go run github.com/favonia/cloudflare-ddns/cmd/ddns@latest
```

⚠️ `PROXIED=true` does not change the proxy statuses of existing records. See [DNS and WAF Fallback Values](#dns-and-waf-fallback-values).

</details>

## 🐋 Deployment with Docker Compose

<a id="docker-compose-template"></a>

### 📦️ Step 1: Updating the Compose File

Incorporate the following fragment into the compose file (typically `docker-compose.yml` or `docker-compose.yaml`). The template looks a bit scary only because it includes various optional flags for extra security protection.

```yaml
services:
  cloudflare-ddns:
    image: favonia/cloudflare-ddns:1
    # Prefer "1" or "1.x.y" in production.
    #
    # - "1" tracks the latest stable release whose major version is 1
    # - "1.x.y" pins one specific stable version
    # - "latest" moves to each new stable release and may pick up breaking
    #   changes in a future major release, so it is not recommended in production
    # - "edge" tracks the latest unreleased development build
    network_mode: host
    # Optional. This bypasses network isolation and makes IPv6 easier.
    # See "Use IPv6 without sharing the host network".
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
        # Leaning toward using Cloudflare's proxy for these domains (optional)
        # Existing DNS records in Cloudflare keep their current proxy statuses
```

<p id="cloudflare-api-token"><code>CLOUDFLARE_API_TOKEN</code> should be a Cloudflare API token, not the older global API key used by some other tools. Create one from the <a href="https://dash.cloudflare.com/profile/api-tokens">API Tokens page</a>, typically using the <strong>Edit zone DNS</strong> template. If you also use <a href="https://developers.cloudflare.com/waf/tools/lists/custom-lists/">WAF lists</a>, add the <strong>Account - Account Filter Lists - Edit</strong> permission.</p>

The `user: "1000:1000"` line sets the user and group IDs that the container runs as, and you can change those two numbers to match your system. The `cap_drop`, `read_only`, and `no-new-privileges` lines add extra protection, especially when you run the container as a non-superuser.

<details>
<summary>📍 <code>DOMAINS</code> is the list of domains to update <sup><em>click to expand</em></sup></summary>

The value of `DOMAINS` should be a list of [fully qualified domain names (FQDNs)](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs the updater to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to share the same DNS zone---the updater will take care of the DNS zones behind the scenes.

</details>

<details>
<summary>🚨 Remove <code>PROXIED=true</code> if you are <em>not</em> running a web server <sup><em>click to expand</em></sup></summary>

Keep `PROXIED=true` when you want [Cloudflare’s proxy](https://developers.cloudflare.com/dns/proxy-status/) for the domains managed by this updater. Proxying lets Cloudflare cache webpages and hide your IP addresses.

| If you want...                                        | Do this                                                                                                                |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Create new records that expose your real IP addresses | Remove `PROXIED=true` or change it to `PROXIED=false`                                                                  |
| Create new records for non-HTTP(S) traffic            | Remove `PROXIED=true` or change it to `PROXIED=false`, because Cloudflare cannot proxy it                              |
| Create new records whose HTTP(S) traffic is proxied   | Keep `PROXIED=true`                                                                                                    |
| Change the proxy statuses of existing records         | Change them manually on the [Cloudflare DNS Records page](https://dash.cloudflare.com/?to=/:account/:zone/dns/records) |

The default value of `PROXIED` is `false`.

</details>

If you need a non-default Docker Compose deployment, see [Docker Compose Special Setups](#docker-compose-special-setups).

### 🚀 Step 2: Building and Running the Container

```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```

The updater should now be running in the background. Check the logs with `docker-compose logs cloudflare-ddns` and confirm that it started correctly.

<a id="docker-compose-special-setups"></a>

## 🧩 Docker Compose Special Setups

These setups are additive changes on top of the basic Docker Compose template in [Step 1: Updating the Compose File](#docker-compose-template). Each setup shows a minimal delta. For the exact behavior of each environment variable, see [All Settings](#all-settings).

### ✅️ Validation and Testing

#### Test a new setup with explicit IPs

Use this when you want to validate the updater without waiting for a real IP change.

Point the updater at dedicated test domain names and feed it explicit test IPs:

```yaml
environment:
  - DOMAINS=ddns-test.example.org
  - IP4_PROVIDER=static:203.0.113.10
  - IP6_PROVIDER=static:2001:db8::10
```

After the testing is done, switch `DOMAINS`, `IP4_PROVIDER`, and `IP6_PROVIDER` to your production values.

⚠️ `static:<ip1>,<ip2>,...` is an advanced provider that supplies a fixed set of IP addresses. It is useful for tests, debugging, and other setups where you want to feed a known address set into the updater, but it is not the normal long-running DDNS path.

#### 🧪 Test a new setup with changing IPs (unreleased)

Use this when you want to validate the updater with simulated IP changes by reading test addresses from local files.

Create `ip4.txt` and `ip6.txt` with one IP address per line (blank lines and `#` comments are ignored). Then, point the updater at dedicated test domain names and feed it the file paths:

```yaml
environment:
  - DOMAINS=ddns-test.example.org
  - IP4_PROVIDER=file:/ip4.txt
  - IP6_PROVIDER=file:/ip6.txt
volumes:
  - $PWD/ip4.txt:/ip4.txt
  - $PWD/ip6.txt:/ip6.txt
```

After the updater creates or updates the expected records, change the addresses in `ip4.txt` or `ip6.txt` to simulate further IP changes. The updater should pick up new content and reconcile the DNS records.

After testing is done, switch `DOMAINS`, `IP4_PROVIDER`, and `IP6_PROVIDER` to your production values and remove the test files and `volumes:` entries.

#### Test how the updater reconciles manual DNS edits

Use this when you want to test how the updater responds after DNS records are changed directly in Cloudflare.

By default, the updater caches Cloudflare API responses to reduce network traffic. To make it fetch the latest DNS records every time, disable that cache:

```yaml
environment:
  - CACHE_EXPIRATION=1ns
```

With `CACHE_EXPIRATION=1ns`, you can edit DNS records in Cloudflare and watch the updater reconcile them right away.

`CACHE_EXPIRATION` affects cached Cloudflare API responses. It does not affect public IP detection. The updater still detects the current public IP addresses each time it runs.

Restore the default `CACHE_EXPIRATION` afterward to avoid unnecessary network traffic.

### 🌐 Networking

#### Run IPv4-only or IPv6-only

Use this when your network supports only one IP family or when you want to stop seeing detection failures for the other one.

```yaml
environment:
  - IP6_PROVIDER=none
```

Use `IP6_PROVIDER=none` to stop managing IPv6, or `IP4_PROVIDER=none` to stop managing IPv4. Existing managed DNS records of that IP family are preserved. 🧪 If you also use WAF lists, existing managed items of that IP family are preserved there too.

#### Use IPv6 without sharing the host network

Use this when you want IPv6 support but do not want `network_mode: host`.

```yaml
services:
  cloudflare-ddns:
    # Remove this line:
    # network_mode: host
```

After removing `network_mode: host`, follow the [official Docker instructions for enabling IPv6](https://docs.docker.com/engine/daemon/ipv6/) on your Docker bridge network.

<a id="docker-network-routing"></a>

#### Route outbound requests through a specific Docker network

Use this when the updater runs in Docker and must send requests through one specific network path so Cloudflare sees the right public IP address.

If you want all outbound requests from the container to use a specific Docker-attached network, one solution is to create a [MacVLAN network](https://docs.docker.com/engine/network/drivers/macvlan/):

```bash
docker network create \
  -d macvlan \
  -o parent=eth0 \
  --subnet=192.168.1.0/24 \
  --gateway=192.168.1.1 \
  --ip-range=192.168.1.128/25 \
  LAN0
```

Then attach the service to that network instead of using `network_mode: host`:

```yaml
services:
  cloudflare-ddns:
    networks: [LAN0]

networks:
  LAN0:
    external: true
    name: LAN0
```

⚠️ [MacVLAN](https://docs.docker.com/engine/network/drivers/macvlan/) can bypass parts of your host firewall setup, so host `iptables` or `nftables` rules may not see this traffic.

#### 🧪 Read addresses from one host interface

Use this when you are already using `network_mode: host` and want the updater to read addresses from one specific host interface instead of choosing from all host interfaces.

```yaml
environment:
  - IP4_PROVIDER=local.iface:eth0
  - IP6_PROVIDER=local.iface:eth0
```

If you want to change where outbound requests leave the container instead, see [Route outbound requests through a specific Docker network](#docker-network-routing).

🧪 `local.iface:<iface>` is still experimental.

### 🔐 Cloudflare API Tokens

#### Read the Cloudflare token from a Docker secret

Use this when you do not want to put the token directly in the Compose file or `.env` file.

Replace the inline token with a file-backed token:

```yaml
services:
  cloudflare-ddns:
    environment:
      - CLOUDFLARE_API_TOKEN_FILE=/run/secrets/cloudflare_api_token
    secrets:
      - cloudflare_api_token

secrets:
  cloudflare_api_token:
    file: ./secrets/cloudflare_api_token.txt
```

⚠️ The token file must be readable by the user configured by `user: "UID:GID"`.

### 🧭 Resource Scope and Ownership

#### 🧪 Update only WAF lists

The updater can work without DNS records and manage only WAF lists.

```yaml
environment:
  - WAF_LISTS=0123456789abcdef0123456789abcdef/home-ips
  # Do not set DOMAINS, IP4_DOMAINS, or IP6_DOMAINS
```

Use a Cloudflare API token with the **Account - Account Filter Lists - Edit** permission.

### 🤝 Multiple Instances

Use this when multiple updater instances may overlap and each instance should manage only its own resources. The DNS setup and the WAF setup are independent:

- If multiple instances share DNS domains, configure the DNS subsection.
- If multiple instances share WAF lists, configure the WAF subsection.
- If both apply, configure both subsections.

#### Share DNS domains across updater instances

Use this when multiple instances share DNS domains. This setup does not use WAF lists. Give each instance its own DNS record comment value and matching selector:

1. Set a unique `RECORD_COMMENT`.
2. Set `MANAGED_RECORDS_COMMENT_REGEX` to match that same DNS comment, typically with `^...$`.

Example:

- Instance A: `RECORD_COMMENT=managed-by-ddns-a`, `MANAGED_RECORDS_COMMENT_REGEX=^managed-by-ddns-a$`
- Instance B: `RECORD_COMMENT=managed-by-ddns-b`, `MANAGED_RECORDS_COMMENT_REGEX=^managed-by-ddns-b$`

> This setup requires `MANAGED_RECORDS_COMMENT_REGEX` (unreleased).

#### Share WAF lists across updater instances

Use this when multiple instances share WAF lists. This setup does not use the DNS settings in Share DNS domains across updater instances. Give each instance its own WAF list item comment and matching selector:

1. Set a unique `WAF_LIST_ITEM_COMMENT`.
2. Set `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` to match that same WAF list item comment, typically with `^...$`.

Example:

- Instance A: `WAF_LIST_ITEM_COMMENT=managed-by-ddns-a`, `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=^managed-by-ddns-a$`
- Instance B: `WAF_LIST_ITEM_COMMENT=managed-by-ddns-b`, `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=^managed-by-ddns-b$`

> 🧪 This setup requires `WAF_LIST_ITEM_COMMENT` (unreleased) and `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` (unreleased). Both settings are experimental.

## 🚚 Non-Docker Setups

These setups are for runtimes that are not additive changes on top of the Docker Compose template in [Step 1: Updating the Compose File](#docker-compose-template).

### ⚙️ Deploy as a system service

The repository currently includes [community-contributed sample configurations](./contrib/README.markdown) for OpenBSD. Additional service-manager examples, such as `systemd`, belong there too.

### 🦭 Run the container with Podman

Start with the same image and environment variables shown in [Quick Start](#quick-start) or [Step 1: Updating the Compose File](#docker-compose-template), then adapt the run command to your Podman workflow. This README does not currently maintain Podman-specific commands, Quadlet files, or Compose conversions.

### ☸️ Run on Kubernetes

Due to high maintenance costs, the dedicated Kubernetes instructions have been removed. You can still generate Kubernetes configurations from the Docker Compose template using [Kompose](https://kompose.io/) version 1.35.0 or later. A simple [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) is sufficient here; there is no inbound traffic, so a [Service](https://kubernetes.io/docs/concepts/services-networking/service/) is not required. This README does not maintain first-party Kubernetes manifests.

## 🛠️ Troubleshooting

### 🤔 I got <code>exec /bin/ddns: operation not permitted</code>

Some Docker, kernel, and virtualization combinations do not work well with [`security_opt: [no-new-privileges:true]`](https://docs.docker.com/reference/cli/docker/container/run/). If this happens, try removing that one hardening option and start the container again. This slightly reduces security, so keep the other hardening options if possible.

If removing `no-new-privileges` fixes the problem, keep it disabled for this container or adjust your security policy to allow this binary.

If removing `no-new-privileges` does not help, try a minimal image such as `alpine` or another popular Docker image with the same hardening option. If that also fails, the problem is likely in the host environment rather than this updater. Reported cases have included older kernels and some QEMU/Proxmox-style virtualized setups.

If none of these applies, please [open an issue on GitHub](https://github.com/favonia/cloudflare-ddns/issues/new/choose) and include your compose file with secrets redacted, `docker version`, `uname -a`, your host OS and virtualization platform (if any), and whether a minimal image such as `alpine` shows the same error.

### 🤔 I am getting <code>error code: 1034</code>

There have been reports of intermittent issues with the default provider `cloudflare.trace`. If you see `error code: 1034`, upgrade to version 1.15.1 or later, or switch to another provider such as `cloudflare.doh` or `url:<url>`.

### 🤔 I got <code>context deadline exceeded</code> and IP detection failed

The first thing to check is whether a container can reach Cloudflare from the Docker environment at all. A simple way to test that is to run a minimal image such as `alpine` and try both DNS resolution and HTTPS connectivity:

```bash
docker run --rm alpine nslookup api.cloudflare.com
docker run --rm alpine wget -qO- https://api.cloudflare.com/cdn-cgi/trace
```

If `nslookup` fails, your Docker setup likely has a DNS problem. If `wget` fails, outbound HTTPS connectivity to Cloudflare is likely blocked or broken. If both commands work, try increasing `DETECTION_TIMEOUT` (for example, `DETECTION_TIMEOUT=1m`) in case requests are simply slow in your environment.

If that still does not help, please [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new/choose) and include your setup details, relevant configs with secrets redacted, and any logs you have so that we can investigate further.

### 🤔 Why did the updater detect a public IP address different from the WAN IP address on my router?

If your router shows an address between `100.64.0.0` and `100.127.255.255`, you are likely behind [CGNAT (Carrier-grade NAT)](https://en.wikipedia.org/wiki/Carrier-grade_NAT). In that case, your ISP is not giving you a real public IP address, so ordinary DDNS cannot make your home network directly reachable from the Internet.

Your options are usually to switch to an ISP that gives you a real public IP address or to use a different approach such as [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/).

### 🤔 How can I see the timestamps of the IP checks and/or updates?

The updater does not add timestamps itself because most runtimes already do:

- If you are using Docker Compose, Kubernetes, or Docker directly, add `--timestamps` when viewing the logs.
- If you are using Portainer, [enable “Show timestamp” when viewing the logs](https://docs.portainer.io/user/docker/containers/logs).

## 🎛️ Further Customization

<a id="all-settings"></a>

### ⚙️ All Settings

The emoji “🧪” marks experimental features, and the emoji “🤖” marks technical details that most readers can skip on a first pass.

<details>
<summary>🔐 Cloudflare API Access <sup><em>click to expand</em></sup></summary>

> Starting with version 1.15.0, the updater supports environment variables that begin with `CLOUDFLARE_*`. Multiple environment variables can be used at the same time, provided they all specify the same token.

| Name                                                      | Meaning                                                                                                                                          |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `CLOUDFLARE_API_TOKEN`                                    | The [Cloudflare API token](https://dash.cloudflare.com/profile/api-tokens) to access the Cloudflare API                                          |
| `CLOUDFLARE_API_TOKEN_FILE`                               | An absolute path to a file that contains the [Cloudflare API token](https://dash.cloudflare.com/profile/api-tokens) to access the Cloudflare API |
| `CF_API_TOKEN` (will be deprecated in version 2.0.0)      | Same as `CLOUDFLARE_API_TOKEN`                                                                                                                   |
| `CF_API_TOKEN_FILE` (will be deprecated in version 2.0.0) | Same as `CLOUDFLARE_API_TOKEN_FILE`                                                                                                              |

> 🚂 Cloudflare is updating its tools to use environment variables starting with `CLOUDFLARE_*` instead of `CF_*`. It is recommended to align your setting with this new convention. However, the updater will fully support both `CLOUDFLARE_*` and `CF_*` environment variables until version 2.0.0.
>
> 🌐 To update DNS records, the updater needs the **Zone - DNS - Edit** permission.
>
> 📋️ To manipulate WAF lists, the updater needs the **Account - Account Filter Lists - Edit** permission.
>
> 💡 `CLOUDFLARE_API_TOKEN_FILE` works well with [Docker secrets](https://docs.docker.com/compose/how-tos/use-secrets/) where secrets will be mounted as files at `/run/secrets/<secret-name>`.
>
> ⚠️ Any `*_FILE` variable must point to a file readable by the user configured by `user: "UID:GID"`.

</details>

<details>
<summary>🌐 DNS Record Scope <sup><em>click to expand</em></sup></summary>

> You need to specify at least one thing in `DOMAINS`, `IP4_DOMAINS`, or `IP6_DOMAINS` for the updater to manage DNS records.

| Name                                         | Meaning                                                                                                                                                                                                                                               | Default Value                               |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `DOMAINS`                                    | Comma-separated fully qualified domain names or wildcard domain names that the updater should manage for both `A` and `AAAA` records. Listing a domain in `DOMAINS` is equivalent to listing the same domain in both `IP4_DOMAINS` and `IP6_DOMAINS`. | `""` (empty list)                           |
| `IP4_DOMAINS`                                | Comma-separated fully qualified domain names or wildcard domain names that the updater should manage for `A` records                                                                                                                                  | `""` (empty list)                           |
| `IP6_DOMAINS`                                | Comma-separated fully qualified domain names or wildcard domain names that the updater should manage for `AAAA` records                                                                                                                               | `""` (empty list)                           |
| `MANAGED_RECORDS_COMMENT_REGEX` (unreleased) | Regex that matches comments of existing DNS records this updater manages. Only records whose comments match are updated or deleted. Uses [RE2](https://github.com/google/re2/wiki/Syntax) syntax (the Go `regexp` syntax, not Perl/PCRE).             | `""` (empty regex; manages all DNS records) |

> 🤖 **Wildcard domains** (`*.example.org`) represent all subdomains that _would not exist otherwise._ Therefore, if you have another subdomain entry `sub.example.org`, the wildcard domain is independent of it, because it only represents the _other_ subdomains which do not have their own entries. Also, you can only have one layer of `*`---`*.*.example.org` would not work.
>
> 🤖 **Internationalized domain names** are handled using the _nontransitional processing_ (fully compatible with IDNA2008). At this point, all major browsers and whatnot have switched to the same nontransitional processing. See [this useful FAQ on internationalized domain names](https://www.unicode.org/faq/idn).

</details>

<details>
<summary>📋️ WAF List Scope <sup><em>click to expand</em></sup></summary>

> The updater can maintain [WAF lists](https://developers.cloudflare.com/waf/tools/lists/custom-lists/) to match detected IP addresses. By default, IPv4 addresses are stored individually and IPv6 addresses are stored as `/64` ranges.

| Name                                                   | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Default Value                                  |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| 🧪 `WAF_LISTS` (available since version 1.14.0)        | <p>🧪 Comma-separated references of [WAF lists](https://developers.cloudflare.com/waf/tools/lists/custom-lists/) the updater should manage. A list reference is written in the format `<account-id>/<list-name>` where `account-id` is your account ID and `list-name` is the list name; it should look like `0123456789abcdef0123456789abcdef/mylist`. If the referenced WAF list does not exist, the updater will try to create it.</p><p>🔑 The API token needs the **Account - Account Filter Lists - Edit** permission.<br/>💡 See [how to find your account ID](https://developers.cloudflare.com/fundamentals/account/find-account-and-zone-ids/).</p> | `""` (empty list)                              |
| 🧪 `MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX` (unreleased) | 🧪 Regex that matches comments of existing WAF list items this updater manages. This lets multiple updater instances share one WAF list safely: only matched items are updated or deleted. Uses [RE2](https://github.com/google/re2/wiki/Syntax) syntax (the Go `regexp` syntax, not Perl/PCRE).                                                                                                                                                                                                                                                                                                                                                              | `""` (empty regex; manages all WAF list items) |

> 🧪 The defaults (individual IPv4 addresses, i.e. `/32`; `/64` ranges for IPv6) are configurable via `IP4_DEFAULT_PREFIX_LEN` and `IP6_DEFAULT_PREFIX_LEN` in the [IP Detection](#ip-detection) section. If a detected address already carries its own prefix length (from CIDR notation), that prefix length is used instead of the default.
>
> 🤖 Existing ranges in the list that already cover a detected address are kept as-is. See [IPv6 Default Prefix Length Policy](docs/designs/features/ipv6-default-prefix-length-policy.markdown) for the design rationale behind the `/64` default.

</details>

<a id="ip-detection"></a>

<details>
<summary>🔍️ IP Detection <sup><em>click to expand</em></sup></summary>

| Name                                     | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Default Value      |
| ---------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `IP4_PROVIDER`                           | This specifies how to detect the current IPv4 address. Available providers include `cloudflare.trace`, `cloudflare.doh`, `local`, `local.iface:<iface>`, `url:<url>`, `url.via4:<url>`, `url.via6:<url>`, `static:<ip1>,<ip2>,...`, `static.empty`, `file:<absolute-path>`, and `none`. The special `none` provider stops managing IPv4. See the provider table in this section for the detailed explanation.                                                                                                                                                                           | `cloudflare.trace` |
| `IP6_PROVIDER`                           | This specifies how to detect the current IPv6 address. Available providers include `cloudflare.trace`, `cloudflare.doh`, `local`, `local.iface:<iface>`, `url:<url>`, `url.via4:<url>`, `url.via6:<url>`, `static:<ip1>,<ip2>,...`, `static.empty`, `file:<absolute-path>`, and `none`. The special `none` provider stops managing IPv6. See the provider table in this section for the detailed explanation.                                                                                                                                                                           | `cloudflare.trace` |
| 🧪 `IP4_DEFAULT_PREFIX_LEN` (unreleased) | 🧪 The default CIDR prefix length for detected bare IPv4 addresses. When a provider discovers a bare address (without CIDR notation), this prefix length is attached. DNS records currently ignore this setting, but future features may use it. WAF lists use the prefix length to determine the stored range: for example, `24` stores each bare detection as a `/24` range. Valid range: 8–32.                                                                                                                                                                                       | `32`               |
| 🧪 `IP6_DEFAULT_PREFIX_LEN` (unreleased) | 🧪 The default CIDR prefix length for detected bare IPv6 addresses. When a provider discovers a bare address (without CIDR notation), this prefix length is attached. DNS records currently ignore this setting, but future features may use it. WAF lists use the prefix length to determine the stored range: for example, `48` stores each bare detection as a `/48` range. Valid range: 12–128. 🤖 See [IPv6 Default Prefix Length Policy](docs/designs/features/ipv6-default-prefix-length-policy.markdown) for the design rationale behind the `/64` default (instead of `/128`). | `64`               |

> 👉️ The option `IP4_PROVIDER` governs `A`-type DNS records and IPv4 addresses in WAF lists, while the option `IP6_PROVIDER` governs `AAAA`-type DNS records and IPv6 addresses in WAF lists. The two options act independently of each other. You can specify different address providers for IPv4 and IPv6.

| Provider Name                                             | Explanation                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `cloudflare.trace`                                        | Get the IP address by parsing the [Cloudflare debugging page](https://api.cloudflare.com/cdn-cgi/trace). **This is the default provider.**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `cloudflare.doh`                                          | Get the IP address by querying `whoami.cloudflare.` against [Cloudflare via DNS-over-HTTPS](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `local`                                                   | <p>Get the IP address via local network interfaces and routing tables. The updater will use the local address that _would have_ been used for outbound UDP connections to Cloudflare servers. (No data will be transmitted.)</p><p>⚠️ The updater needs access to the host network (such as `network_mode: host` in Docker Compose) for this provider, for otherwise the updater will detect the addresses inside [the default bridge network in Docker](https://docs.docker.com/engine/network/drivers/bridge/) instead of those in the host network.</p>                                                                                                                                                                                                                                 |
| 🧪 `local.iface:<iface>` (available since version 1.15.0) | <p>🧪 Get IP addresses via the specific local network interface `iface`. Since the unreleased version, the updater collects all matching global unicast addresses of the selected IP family (IPv4 or IPv6) instead of just the first one, then reconciles DNS records and WAF lists against that full set.</p><p>⚠️ The updater needs access to the host network (such as `network_mode: host` in Docker Compose) for this provider, for otherwise the updater cannot access host network interfaces.</p><p>🤖 The updater ignores the prefix length reported by the interface, because it commonly describes its local subnet, not the range the updater should claim. The updater uses the default prefix lengths from `IP4_DEFAULT_PREFIX_LEN` or `IP6_DEFAULT_PREFIX_LEN` instead.</p> |
| `url:<url>`                                               | <p>Fetch the IP address from a URL. The provider format is `url:` followed by the URL itself. For example, `IP4_PROVIDER=url:https://api4.ipify.org` fetches the IPv4 address from <https://api4.ipify.org>. Currently, only HTTP(S) is supported.</p><p>The updater connects over IPv4 for `IP4_PROVIDER` and over IPv6 for `IP6_PROVIDER`. The intention is to query a public IP detection server with the correct IP family. If you want to override that, use `IP4_PROVIDER=url.via6:<url>` or `IP6_PROVIDER=url.via4:<url>` instead.</p><p>🧪 The response may also contain multiple addresses or addresses in CIDR notation, using the line-based text format described after this table.</p><p>🕰️ Before version 1.15.0, `url:<url>` did not enforce the matching IP family.</p>    |
| `url.via4:<url>` (unreleased)                             | <p>Fetch the IP address from a URL while always connecting to that URL over IPv4. 🧪 Same text format as `url:`.</p><p>The intention is to get an IPv6 address over IPv4 with `IP6_PROVIDER=url.via4:<url>`. In comparison, `IP6_PROVIDER=url:<url>` will get an IPv6 address over the matching IP family (IPv6).</p>                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `url.via6:<url>` (unreleased)                             | <p>Fetch the IP address from a URL while always connecting to that URL over IPv6. 🧪 Same text format as `url:`.</p><p>The intention is to get an IPv4 address over IPv6 with `IP4_PROVIDER=url.via6:<url>`. In comparison, `IP4_PROVIDER=url:<url>` will get an IPv4 address over the matching IP family (IPv4).</p>                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 🧪 `file:<absolute-path>` (unreleased)                    | <p>🧪 Read IP addresses from a local file using the line-based text format described after this table. The path must be absolute.</p><p>The file is re-read on every detection cycle, so you can update it without restarting the updater.</p><p>⚠️ The file must be readable by the user configured by `user: "UID:GID"`.</p>                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `static:<ip1>,<ip2>,...` (unreleased)                     | <p>Use one or more explicit IP addresses (or 🧪 addresses in CIDR notation) as a fixed set, separated by commas. This is an advanced provider for tests, debugging, and special fixed-input setups.</p><p>⚠️ Most users should not use it for normal long-running DDNS.</p><p>🤖 The entries are parsed, deduplicated, sorted, and validated for the selected IP family via the same normalization pipeline used by other providers.</p>                                                                                                                                                                                                                                                                                                                                                   |
| `static.empty` (unreleased)                               | <p>Clear existing managed content for the selected IP family. In contrast, `none` preserves existing managed content for that family.</p><p>⚠️ Most users should not use it for normal long-running DDNS.</p>                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `none`                                                    | <p>Stop managing the specified IP family for this run. For example `IP4_PROVIDER=none` stops managing IPv4. Existing managed DNS records of that IP family are preserved.</p><p>🧪 Existing managed WAF list items of that IP family are preserved too, because that family is out of scope. Use `static.empty` if you want to clear managed content for that family. As the support of WAF lists is still experimental, please [provide feedback](https://github.com/favonia/cloudflare-ddns/issues/new/choose) if this does not match your needs.</p>                                                                                                                                                                                                                                    |

> 🧪 The `url`, `url.via4`, `url.via6`, and `file` providers share the following line-based text format. Each line is one IP address or an address in CIDR notation (e.g., `198.51.100.1/24`). Blank lines are ignored and `#` starts a comment. All entries must belong to the selected IP family; mismatched entries are rejected. Entries are deduplicated and sorted. There must be at least one entry.
>
> ```txt
> # Bare addresses
> 198.51.100.1
> 198.51.100.2
>
> # Addresses in CIDR notation (experimental)
> 198.51.100.0/24
> 198.51.100.128/25  # inline comments are supported
> ```

</details>

<details>
<summary>📅 Update Schedule and Lifecycle <sup><em>click to expand</em></sup></summary>

| Name               | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Default Value                 |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- |
| `CACHE_EXPIRATION` | The expiration of cached Cloudflare API responses. It can be any positive time duration accepted by [time.ParseDuration](https://pkg.go.dev/time#ParseDuration), such as `1h` or `10m`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | `6h0m0s` (6 hours)            |
| `DELETE_ON_STOP`   | <p>Whether managed DNS records and managed WAF content are deleted when the updater exits. It accepts any boolean value supported by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1`.</p><p>DNS cleanup applies only to the IP families this updater is managing in that run.</p><p>🧪 For WAF lists, the updater deletes the whole list only when the current configuration is enough to recreate it safely. Otherwise shutdown cleanup keeps the list and deletes only managed items in the in-scope families.</p>                                                                                                                                           | `false`                       |
| `TZ`               | <p>The timezone used for logging messages and parsing `UPDATE_CRON`. It can be any timezone accepted by [time.LoadLocation](https://pkg.go.dev/time#LoadLocation), including any IANA Time Zone.</p><p>🤖 The pre-built Docker images come with the embedded timezone database via the [time/tzdata](https://pkg.go.dev/time/tzdata) package.</p>                                                                                                                                                                                                                                                                                                                                                              | `UTC`                         |
| `UPDATE_CRON`      | <p>The schedule to re-check IP addresses and update DNS records and WAF lists (if needed). The format is [any cron expression accepted by the `cron` library](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format) or the special value `@once`. The special value `@once` means the updater will terminate immediately after updating the DNS records or WAF lists, effectively disabling the scheduling feature.</p><p>🤖 The update schedule _does not_ take the time to update records into consideration. For example, if the schedule is `@every 5m`, and if the updating itself takes 2 minutes, then the actual interval between adjacent updates is 3 minutes, not 5 minutes.</p> | `@every 5m` (every 5 minutes) |
| `UPDATE_ON_START`  | Whether to check IP addresses (and possibly update DNS records and WAF lists) _immediately_ on start, regardless of the update schedule specified by `UPDATE_CRON`. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1`.                                                                                                                                                                                                                                                                                                                                                                                                   | `true`                        |

</details>

<details>
<summary>⏳️ Operation Timeouts <sup><em>click to expand</em></sup></summary>

| Name                | Meaning                                                                                                                                                                                                                                  | Default Value      |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `DETECTION_TIMEOUT` | The timeout of each attempt to detect IP address, per IP version (IPv4 and IPv6). It can be any positive time duration accepted by [time.ParseDuration](https://pkg.go.dev/time#ParseDuration), such as `1h` or `10m`.                   | `5s` (5 seconds)   |
| `UPDATE_TIMEOUT`    | The timeout of each attempt to update DNS records, per domain and per record type, or per WAF list. It can be any positive time duration accepted by [time.ParseDuration](https://pkg.go.dev/time#ParseDuration), such as `1h` or `10m`. | `30s` (30 seconds) |

</details>

<a id="dns-and-waf-fallback-values"></a>

<details>
<summary>🛟 DNS and WAF Fallback Values <sup><em>click to expand</em></sup></summary>

> The updater preserves existing attributes (such as TTL and proxy status) when possible. 🤖 It keeps existing attribute values when old content agrees on them; otherwise, it uses the fallback values in this table when the values conflict.

| Name                                                       | Meaning                                                                                                                                                                                                                                                                                                                                           | Default Value                              |
| ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `PROXIED`                                                  | <p>Fallback proxy setting for DNS records managed by the updater. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1`.</p><p>🤖 Advanced usage: it can also be a domain-dependent boolean expression, as described in the examples later in this section.</p> | `false`                                    |
| `TTL`                                                      | Fallback TTL (in seconds) for DNS records managed by the updater.                                                                                                                                                                                                                                                                                 | `1` (This means “automatic” to Cloudflare) |
| `RECORD_COMMENT`                                           | Fallback [record comment](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) for DNS records managed by the updater.                                                                                                                                                                                          | `""`                                       |
| 🧪 `WAF_LIST_DESCRIPTION` (available since version 1.14.0) | <p>🧪 Fallback description for WAF lists managed by the updater.</p><p>🤖 This matters only when the updater needs to create a new WAF list, because a WAF list has only one description.</p>                                                                                                                                                     | `""`                                       |
| 🧪 `WAF_LIST_ITEM_COMMENT` (unreleased)                    | 🧪 Fallback comment for WAF list items managed by the updater.                                                                                                                                                                                                                                                                                    | `""`                                       |

> 🤖 For DNS records, the updater recycles existing records when it can (instead of delete-then-create). Cloudflare does not support updating one WAF list item in place, so WAF changes always use delete-then-create.
>
> 🤖 For advanced users: `PROXIED` can also be a domain-dependent boolean expression. This lets you enable Cloudflare proxying for some managed domains but not others. Here are some example expressions:
>
> - `PROXIED=is(example.org)`: proxy only the domain `example.org`
> - `PROXIED=is(example1.org) || sub(example2.org)`: proxy only the domain `example1.org` and subdomains of `example2.org`
> - `PROXIED=!is(example.org)`: proxy every managed domain _except for_ `example.org`
> - `PROXIED=is(example1.org) || is(example2.org) || is(example3.org)`: proxy only the domains `example1.org`, `example2.org`, and `example3.org`
>
> A boolean expression can take one of the following forms (all whitespace is ignored):
>
> | Syntax                                                                                                                 | Meaning                                                                                                                                             |
> | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
> | Any string accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1` | Logical truth or falsehood                                                                                                                          |
> | `is(d)`                                                                                                                | Matching the domain `d`. Note that `is(*.a)` only matches the wildcard domain `*.a`; use `sub(a)` to match all subdomains of `a` (including `*.a`). |
> | `sub(d)`                                                                                                               | Matching subdomains of `d`, including `a.d`, `b.c.d`, and wildcard domains like `*.d` and `*.a.d`, but not `d` itself.                              |
> | `! e`                                                                                                                  | Logical negation of the boolean expression `e`                                                                                                      |
> | <code>e1 \|\| e2</code>                                                                                                | Logical disjunction of the boolean expressions `e1` and `e2`                                                                                        |
> | `e1 && e2`                                                                                                             | Logical conjunction of the boolean expressions `e1` and `e2`                                                                                        |
>
> One can use parentheses to group expressions, such as `!(is(a) && (is(b) || is(c)))`. For convenience, the parser also accepts these short forms:
>
> | Short Form             | Equivalent Full Form                                    |
> | ---------------------- | ------------------------------------------------------- |
> | `is(d1, d2, ..., dn)`  | <code>is(d1) \|\| is(d2) \|\| ... \|\| is(dn)</code>    |
> | `sub(d1, d2, ..., dn)` | <code>sub(d1) \|\| sub(d2) \|\| ... \|\| sub(dn)</code> |
>
> For example, these two settings are equivalent:
>
> - `PROXIED=is(example1.org) || is(example2.org) || is(example3.org)`
> - `PROXIED=is(example1.org,example2.org,example3.org)`

</details>

<details>
<summary>👁️ Logging <sup><em>click to expand</em></sup></summary>

| Name    | Meaning                                                                                                                                                                                        | Default Value |
| ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `EMOJI` | Whether the updater should use emojis in the logging. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1`. | `true`        |
| `QUIET` | Whether the updater should reduce the logging. It can be any boolean value accepted by [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool), such as `true`, `false`, `0`, or `1`.        | `false`       |

</details>

<details>
<summary>📣 Notifications <sup><em>click to expand</em></sup></summary>

> 💡 If your network doesn’t support IPv6, set `IP6_PROVIDER=none` to stop managing IPv6. This will prevent the updater from reporting failures in detecting IPv6 addresses to monitoring services. Similarly, set `IP4_PROVIDER=none` if your network doesn’t support IPv4.

| Name                                           | Meaning                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `HEALTHCHECKS`                                 | <p>The [Healthchecks ping URL](https://healthchecks.io/docs/) to ping when the updater successfully updates IP addresses, such as `https://hc-ping.com/<uuid>` or `https://hc-ping.com/<project-ping-key>/<name-slug>`</p><p>⚠️ The ping schedule should match the update schedule specified by `UPDATE_CRON`.<br/>🤖 The updater can work with _any_ server following the [same Healthchecks protocol](https://healthchecks.io/docs/http_api/), including self-hosted instances of [Healthchecks](https://github.com/healthchecks/healthchecks). Both UUID and Slug URLs are supported, and the updater works regardless whether the POST-only mode is enabled.</p> |
| `UPTIMEKUMA`                                   | <p>The Uptime Kuma’s Push URL to ping when the updater successfully updates IP addresses, such as `https://<host>/push/<id>`. You can directly copy the “Push URL” from the Uptime Kuma configuration page.</p><p>⚠️ The “Heartbeat Interval” should match the update schedule specified by `UPDATE_CRON`.</p>                                                                                                                                                                                                                                                                                                                                                       |
| 🧪 `SHOUTRRR` (available since version 1.12.0) | <p>Newline-separated [shoutrrr URLs](https://containrrr.dev/shoutrrr/latest/services/overview/) to which the updater sends notifications of IP address changes and other events. In other words, put one URL on each line. Each shoutrrr URL represents a notification service; for example, `discord://<token>@<id>` means sending messages to Discord. If one URL needs spaces, percent-encode them to help the updater parse URLs.</p><p>If you configure this value via YAML, prefer <a href="https://yaml-multiline.info/">literal block style <code>\|</code></a> over <a href="https://yaml-multiline.info/">folded style <code>></code></a>.</p>             |

</details>

### 🔂 Restarting the Container

If you are using Docker Compose, run `docker-compose up --detach` to reload settings.

## 🚵 Migration Guides

<details>
<summary>I am migrating from oznu/cloudflare-ddns (now archived) <sup><em>click to expand</em></sup></summary>

⚠️ [oznu/cloudflare-ddns](https://github.com/oznu/docker-cloudflare-ddns) relies on the insecure DNS protocol to obtain public IP addresses; a malicious hacker could more easily forge DNS responses and trick it into updating your domain with any IP address. In comparison, we use only verified responses from Cloudflare, which makes the attack much more difficult. See the [network security design note](docs/designs/features/network-security-model.markdown) for more information.

| Old Parameter                          |     | Note                                                                                                                                                                                                                                                                                                                                                                                                        |
| -------------------------------------- | --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `API_KEY=<key>`                        | ⚠️  | Legacy global API keys are not supported. Please [generate a scoped API token](#cloudflare-api-token) and use `CLOUDFLARE_API_TOKEN=<token>`.                                                                                                                                                                                                                                                               |
| `API_KEY_FILE=/path/to/key-file`       | ⚠️  | Legacy global API keys are not supported. Please [generate a scoped API token](#cloudflare-api-token), save it, and use `CLOUDFLARE_API_TOKEN_FILE=/path/to/token-file`.                                                                                                                                                                                                                                    |
| `ZONE=example.org` and `SUBDOMAIN=sub` | ✔️  | Use `DOMAINS=sub.example.org` directly                                                                                                                                                                                                                                                                                                                                                                      |
| `PROXIED=true`                         | ✔️  | Same (`PROXIED=true`)                                                                                                                                                                                                                                                                                                                                                                                       |
| `RRTYPE=A`                             | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP6_PROVIDER=none` to stop managing IPv6                                                                                                                                                                                                                                                                                                                    |
| `RRTYPE=AAAA`                          | ✔️  | Both IPv4 and IPv6 are enabled by default; use `IP4_PROVIDER=none` to stop managing IPv4                                                                                                                                                                                                                                                                                                                    |
| `DELETE_ON_STOP=true`                  | ✔️  | Same (`DELETE_ON_STOP=true`)                                                                                                                                                                                                                                                                                                                                                                                |
| `INTERFACE=<iface>`                    | ✔️  | To automatically select the local address, use `IP4/6_PROVIDER=local`. 🧪 To select addresses of a specific network interface, use `IP4/6_PROVIDER=local.iface:<iface>` (available since version 1.15.0). Since the unreleased version, the updater collects all matching global unicast addresses instead of just the first one, then reconciles DNS records and WAF lists against that full detected set. |
| `CUSTOM_LOOKUP_CMD=cmd`                | ❌️  | Custom commands are not supported because there are no other programs in the minimal Docker image                                                                                                                                                                                                                                                                                                           |
| `DNS_SERVER=server`                    | ❌️  | For DNS-based IP detection, the updater only supports secure DNS queries using Cloudflare’s DNS over HTTPS (DoH) server. To enable this, set `IP4/6_PROVIDER=cloudflare.doh`. To detect IP addresses via HTTPS by querying other servers, use `IP4/6_PROVIDER=url:<url>`                                                                                                                                    |

</details>

<details>
<summary>I am migrating from timothymiller/cloudflare-ddns <sup><em>click to expand</em></sup></summary>

Since [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) 2.0.0, many setting names and features look very close to this updater. However, similar names do not necessarily mean identical semantics. There are too many settings to list every difference here, and most mismatches will produce a clear startup error, but some differences are silent. If you only set a few options, checking the [documentation for those specific settings](#all-settings) should be quick and worthwhile.

**Known silent semantic differences:**

- ⚠️ **`sub()` in `PROXIED` expressions:** In this updater, `sub(example.com)` matches _strict subdomains only_—it does **not** match `example.com` itself. In timothymiller/cloudflare-ddns, `sub(example.com)` matches `example.com` _and_ all its subdomains. Copying a `PROXIED` expression verbatim may silently change which domains are proxied. If you need to match a domain and all its subdomains, use `is(example.com) || sub(example.com)`.

**Known naming differences that will cause startup errors:**

- `literal:` (timothymiller/cloudflare-ddns) is called `static:` in this updater (_e.g._, `IP4_PROVIDER=static:1.2.3.4`).

> 📜 Some historical notes: This updater was originally written as a Go clone of the Python program [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) because the Python program purged unmanaged DNS records back then and it was not configurable via environment variables on its default branch. Eventually, the Python program became configurable via environment variables and later was rewritten in Rust, but this Go updater had already gone its own way. My opinions are biased, so please check the technical details by yourself. 😉

</details>

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new/choose).

## 📜 License

The code is licensed under [Apache 2.0 with LLVM exceptions](./LICENSE). (The LLVM exceptions provide better compatibility with GPL 2.0 and other license exceptions.)

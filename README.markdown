# 🌟 Cloudflare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/favonia/cloudflare-ddns/Building%20and%20Pushing)](https://github.com/favonia/cloudflare-ddns/actions/workflows/build.yaml)
[![Codecov](https://img.shields.io/codecov/c/github/favonia/cloudflare-ddns)](https://app.codecov.io/gh/favonia/cloudflare-ddns)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/favonia/cloudflare-ddns)](https://golang.org/doc/install)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)

A small and fast DDNS updater for Cloudflare.

```
🔇 Quiet mode enabled
🌟 Cloudflare DDNS
🥷 Priviledges after dropping:
   🔸 Effective UID:      1000
   🔸 Effective GID:      1000
   🔸 Supplementary GIDs: (empty)
🐣 Added a new A record of "……" (ID: ……)
🐣 Added a new AAAA record of "……" (ID: ……)
```

## 📜 Highlights

* Ultra-small Docker images (about 2.5 MB) for all common architectures.
* Ability to update multiple domains across different zones.
* Ability to enable or disable IPv4 and IPv6 individually.
* Support of internationalized domain names.
* Support of wildcard domain names (_e.g._, `*.example.org`).
* Ability to remove stale records or choose to remove records on exit/stop.
* Ability to obtain IP addresses from Cloudflare, ipify, or local network interfaces.
* Support of timezone and Cron expressions.
* Full configurability via environment variables.
* Ability to pass API tokens via a file instead of an environment variable.
* Local caching to reduce Cloudflare API usage.
* Integration with [Healthchecks.io](https://healthchecks.io).

## 🕵️ Privacy

By default, public IP addresses are obtained using the [Cloudflare debugging page](https://1.1.1.1/cdn-cgi/trace). This minimizes the impact on privacy because we are already using the Cloudflare API to update DNS records. Moreover, if Cloudflare servers are not reachable, chances are you could not update DNS records anyways. You can also configure the tool to use [ipify](https://www.ipify.org), which claims not to log any visitor information.

## 🛡️ Security

<details><summary>🚷 The superuser privilege is immediately dropped after the updater starts.</summary>

The updater honors `PGID` and `PUID` and will drop Linux capabilities (divided superuser privileges).
</details>

<details><summary>🔌 The source code depends on six external libraries (outside the Go project).</summary>

- [cap](https://sites.google.com/site/fullycapable):\
  Manipulation of Linux capabilities.
- [cloudflare-go](https://github.com/cloudflare/cloudflare-go):\
  The official Go binding of Cloudflare API v4. It provides robust handling of pagination, rate limiting, and other tricky details.
- [cron](https://github.com/robfig/cron):\
  Parsing of Cron expressions.
- [go-cache](https://github.com/patrickmn/go-cache):\
  Essentially `map[string]interface{}` with expiration times.
- [mock](https://github.com/golang/mock) (for testing only):\
  A comprehensive, semi-official framework for mocking.
- [testify](https://github.com/stretchr/testify) (for testing only):\
  A comprehensive tool set for testing Go programs.
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
<summary>🧬 Directly run the updater from its source.</summary>

You need the [Go tool](https://golang.org/doc/install) to run the updater from its source.

```bash
CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN \
  DOMAINS=www.example.org \
  PROXIED=true \
  go run ./cmd/*.go
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
    network_mode: host
    restart: always
    security_opt:
      - no-new-privileges:true
    environment:
      - PGID=1000
      - PUID=1000
      - CF_API_TOKEN
      - DOMAINS
      - PROXIED=true
```

<details>
<summary>📡 Use <code>network_mode: host</code> to enable IPv6 (or read more).</summary>

The easiest way to enable IPv6 is to use `network_mode: host` so that the tool can access the host IPv6 network directly. If you wish to keep the tool isolated from the host network, check out the [official documentation on IPv6](https://docs.docker.com/config/daemon/ipv6/) and [this GitHub issue about IPv6.](https://github.com/favonia/cloudflare-ddns/issues/119) If your host OS is Linux, here’s the tl;dr:

1. Use `network_mode: bridge` instead of `network_mode: host`.
2. Edit or create `/etc/docker/daemon.json` with the following content:
   ```json
   {
     "ipv6": true,
     "fixed-cidr-v6": "fd00::/8",
     "experimental": true,
     "ip6tables": true
   }
   ```
3. Restart the Docker daemon (if you are using systemd):
   ```sh
   systemctl restart docker.service
   ```
</details>

<details>
<summary>🔁 Use <code>restart: always</code> to automatically restart the updater on system reboot.</summary>

Docker’s default restart policies should prevent excessive logging when there are configuration errors.
</details>

<details>
<summary>🛡️ Use <code>no-new-privileges:true</code>, <code>PUID</code>, and <code>PGID</code> to protect yourself.</summary>

Change `1000` to the user or group IDs you wish to use to run the updater. The setting `no-new-privileges:true` provides additional protection, especially when you run the container as a non-superuser. The updater itself will read <code>PUID</code> and <code>PGID</code> and attempt to drop all those privileges as much as possible.
</details>

<details>
<summary>🎭 Use <code>PROXIED=true</code> to hide your IP addresses.</summary>

The setting `PROXIED=true` instructs Cloudflare to cache webpages on your machine and hide your actual IP addresses. If you wish to bypass that and expose your actual IP addresses, simply remove `PROXIED=true`. (The default value of `PROXIED` is `false`.)
</details>

### 🪧 Step 2: Updating the Environment File

Add these lines to your environment file (typically `.env`):
```bash
CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
DOMAINS=example.org,www.example.org,example.io
```

<details>
<summary>🔑 <code>CF_API_TOKEN</code> is your Cloudflare API token.</summary>

The value of `CF_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Use the **Edit zone DNS** template to create and copy a token into the environment file. ⚠️ The less secure API key authentication is deliberately _not_ supported.
</details>

<details>
<summary>📍 <code>DOMAINS</code> contains the domains to update.</summary>

The value of `DOMAINS` should be a list of fully qualified domain names separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs the tool to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to be in the same zone---the tool will identify their zones automatically.
</details>

### 🚀 Step 3: Building the Container

```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```

## ☸️ Deployment with Kubernetes

Kubernetes offers great flexibility in assembling different objects together. The following shows a minimum setup.

### 📝 Step 1: Creating a YAML File

Save the following configuration as `cloudflare-ddns.yaml`.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cloudflare-ddns
  labels:
    app: cloudflare-ddns
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: cloudflare-ddns
  template:
    metadata:
      name: cloudflare-ddns
      labels:
        app: cloudflare-ddns
    spec:
      restartolicy: Always
      containers:
        - name: cloudflare-ddns
          image: favonia/cloudflare-ddns:latest
          securityContext:
            allowPrivilegeEscalation: false
            runAsUser: 1000
            runAsGroup: 1000
          env:
            - name: "IP6_PROVIDER"
              value: "none"
            - name: "PROXIED"
              value: "true"
            - name: "CF_API_TOKEN"
              value: YOUR-CLOUDFLARE-API-TOKEN
            - name: "DOMAINS"
              value: "example.org,www.example.org,example.io"
```

<details>
<summary>🔁 Use <code>restartPolicy: Always</code> to automatically restart the updater on system reboot.</summary>

Kubernetes’s default restart policies should prevent excessive logging when there are configuration errors.
</details>

<details>
<summary>🛡️ Use <code>runAsUser</code>, <code>runAsGroup</code>, and <code>allowPrivilegeEscalation: false</code> to protect yourself.</summary>

Kubernetes comes with built-in support to drop superuser privileges. The updater itself will also attempt to drop all of them.
</details>

<details>
<summary>📡 Use <code>IP6_PROVIDER: "none"</code> to disable IPv6 management.</summary>

The support of IPv6 in Kubernetes has been improving, but a working setup still takes effort. Since Kubernetes 1.21+, the [IPv4/IPv6 dual stack](https://kubernetes.io/docs/concepts/services-networking/dual-stack/) is enabled by default, but a setup which allows IPv6 egress traffic (_e.g.,_ to reach Cloudflare servers to detect public IPv6 addresses) still requires deep understanding of Kubernetes and is beyond this simple guide. The popular tool [minicube](https://minikube.sigs.k8s.io/), which implements a simple local Kubernetes cluster, unfortunately [does not support IPv6 yet.](https://minikube.sigs.k8s.io/docs/faq/#does-minikube-support-ipv6) Until there is an easy way to enable IPv6 in Kubernetes, the template here will have IPv6 disabled.

If you manage to enable IPv6, congratulations. Feel free to remove `IP6_PROVIDER: "none"` to detect and update both `A` and `AAAA` records. There is almost no danger in enabling IPv6 even when the IPv6 setup is not working. In the worst case, the updater will remove all `AAAA` records associated with the domains in `DOMAINS` and `IP6_DOMAINS` because those records will appear to be “stale.” The deleted records will be recreated once the updater correctly detects the IPv6 addresses.
</details>

<details>
<summary>🎭 Use <code>PROXIED: "true"</code> to hide your IP addresses.</summary>

The setting `PROXIED: "true"` instructs Cloudflare to cache webpages on your machine and hide your actual IP addresses. If you wish to bypass that and expose your actual IP addresses, simply remove `PROXIED: "true"`. (The default value of `PROXIED` is `false`.)
</details>

<details>
<summary>🔑 <code>CF_API_TOKEN</code> is your Cloudflare API token.</summary>

The value of `CF_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained from the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Use the **Edit zone DNS** template to create and copy a token into the environment file. ⚠️ The less secure API key authentication is deliberately _not_ supported.
</details>

<details>
<summary>📍 <code>DOMAINS</code> contains the domains to update.</summary>

The value of `DOMAINS` should be a list of fully qualified domain names separated by commas. For example, `DOMAINS=example.org,www.example.org,example.io` instructs this tool to manage the domains `example.org`, `www.example.org`, and `example.io`. These domains do not have to be in the same zone---this tool will identify their zones automatically.
</details>

### 🚀 Step 2: Creating the Deployment

```sh
kubectl create -f cloudflare-ddns.yaml
```

## 🎛️ Further Customization

### ⚙️ All Settings

<details>
<summary>🔑 Cloudflare accounts and API tokens</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CF_ACCOUNT_ID` | Cloudflare Account IDs | The account ID used to distinguish multiple zone IDs with the same name | No | `""` (unset) |
| `CF_API_TOKEN_FILE` | Paths to files containing Cloudflare API tokens | A file that contains the token to access the Cloudflare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `CF_API_TOKEN` | Cloudflare API tokens | The token to access the Cloudflare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |

In most cases, `CF_ACCOUNT_ID` is not needed.
</details>

<details>
<summary>📍 Domains and IP providers</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `DOMAINS` | Comma-separated fully qualified domain names or wildcard domain names | The domains this tool should manage for both `A` and `AAAA` records | (See below) | N/A
| `IP4_DOMAINS` | Comma-separated fully qualified domain names or wildcard domain names | The domains this tool should manage for `A` records | (See below) | N/A
| `IP6_DOMAINS` | Comma-separated fully qualified domain names or wildcard domain names | The domains this tool should manage for `AAAA` records | (See below) | N/A
| `IP4_PROVIDER` | `cloudflare.doh`, `cloudflare.trace`, `ipify`, `local`, and `none` | How to detect IPv4 addresses. (See below) | No | `cloudflare.trace`
| `IP6_PROVIDER` | `cloudflare.doh`, `cloudflare.trace`, `ipify`, `local`, and `none` | How to detect IPv6 addresses. (See below) | No | `cloudflare.trace`

> <details>
> <summary>📍 At least one of <code>DOMAINS</code> and <code>IP4/6_DOMAINS</code> must be non-empty.</summary>
>
> At least one domain should be listed in `DOMAINS`, `IP4_DOMAINS`, or `IP6_DOMAINS`. Otherwise, if all of them are empty, then this updater has nothing to do. It is fine to list the same domain in both `IP4_DOMAINS` and `IP6_DOMAINS`, which is equivalent to listing it in `DOMAINS`. Internationalized domain names are supported using the non-transitional processing that is fully compatible with IDNA2008.
> </details>

> <details>
> <summary>📜 Available providers for <code>IP4_PROVIDER</code> and <code>IP6_PROVIDER</code></summary>
>
> - `cloudflare.doh`\
>  Get the public IP address by querying `whoami.cloudflare.` against [Cloudflare via DNS-over-HTTPS](https://developers.cloudflare.com/1.1.1.1/dns-over-https) and update DNS records accordingly.
> - `cloudflare.trace`\
>  Get the public IP address by parsing the [Cloudflare debugging page](https://1.1.1.1/cdn-cgi/trace) and update DNS records accordingly.
> - `ipify`\
>   Get the public IP address via [ipify’s public API](https://www.ipify.org/) and update DNS records accordingly.
> - `local`\
>   Get the address via local network interfaces and update DNS records accordingly. When multiple local network interfaces or in general multiple IP addresses are present, the tool will use the address that would have been used for outbound UDP connections to Cloudflare servers. ⚠️ You need access to the host network (such as `network_mode: host` in Docker Compose or `hostNetwork: true` in Kubernetes) for this policy, for otherwise the tool will detect the addresses inside the [bridge network in Docker](https://docs.docker.com/network/bridge/) or the [default namespaces in Kubernetes](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/) instead of those in the host network.
> - `none`\
>   Stop the DNS updating completely. Existing DNS records will not be removed.
>
> The option `IP4_PROVIDER` is governing IPv4 addresses and `A`-type records, while the option `IP6_PROVIDER` is governing IPv6 addresses and `AAAA`-type records. The two options act independently of each other.
> </details>

</details>

<details>
<summary>⏳ Schedules, timeouts, and parameters of new DNS records</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CACHE_EXPIRATION` | Positive time durations with a unit, such as `1h` and `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The expiration of cached Cloudflare API responses | No | `6h0m0s` (6 hours)
| `DELETE_ON_STOP` | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) | Whether managed DNS records should be deleted on exit | No | `false`
| `DETECTION_TIMEOUT` | Positive time durations with a unit, such as `1h` and `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The timeout of each attempt to detect IP addresses | No | `5s` (5 seconds)
| `PROXIED` | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) | Whether new DNS records should be proxied by Cloudflare | No | `false`
| `TTL` | Time-to-live (TTL) values in seconds | The TTL values used to create new DNS records | No | `1` (This means “automatic” to Cloudflare)
| `TZ` | Recognized timezones, such as `UTC` | The timezone used for logging and parsing `UPDATE_CRON` | No | `UTC`
| `UPDATE_CRON` | Cron expressions. See the [documentation of cron](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format) | The schedule to re-check IP addresses and update DNS records (if necessary) | No | `@every 5m` (every 5 minutes)
| `UPDATE_ON_START` | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) | Whether to check IP addresses on start regardless of `UPDATE_CRON` | No | `true`
| `UPDATE_TIMEOUT` | Positive time durations with a unit, such as `1h` and `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The timeout of each attempt to update DNS records, per domain, per record type | No | `30s` (30 seconds)

Note that the update schedule _does not_ take the time to update records into consideration. For example, if the schedule is “for every 5 minutes”, and if the updating itself takes 2 minutes, then the actual interval between adjacent updates is 3 minutes, not 5 minutes.
</details>

<details>
<summary>🛡️ Dropping superuser privileges</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `PGID` | Non-zero POSIX group ID | The group ID this tool should assume | No | Effective group ID; if it is zero, then the real group ID; if it is still zero, then `1000`
| `PUID` | Non-zero POSIX user ID | The user ID this tool should assume | No | Effective user ID; if it is zero, then the real user ID; if it is still zero, then `1000`

The updater will also try to drop supplementary group IDs.
</details>

<details>
<summary>👁️ Monitoring the tool</summary>

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `QUIET` | Boolean values, such as `true`, `false`, `0` and `1`. See [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) | Whether the updater should reduce the logging to the standard output | No | `false`
| `HEALTHCHECKS` | [Healthchecks.io ping URLs,](https://healthchecks.io/docs/) such as `https://hc-ping.com/<uuid>` or `https://hc-ping.com/<project-ping-key>/<name-slug>` | If set, the tool will ping Healthchecks.io when it successfully updates IP addresses | No | N/A
</details>

### 🔂 Restarting the Container

If you are using Docker Compose, run `docker-compose up --detach` after changing the settings.

If you are using Kubernetes, run `kubectl replace -f cloudflare-ddns.yaml` after changing the settings.

## 🚵 Migration Guides

<details>
<summary>I am migrating from <a href="https://hub.docker.com/r/oznu/cloudflare-ddns/">oznu/cloudflare-ddns</a>.</summary>

⚠️ [oznu/cloudflare-ddns](https://hub.docker.com/r/oznu/cloudflare-ddns/) relies on unverified DNS responses to obtain public IP addresses; a malicious hacker could potentially manipulate or forge DNS responses and trick it into updating your domain with any IP address. In comparison, we use only verified responses from Cloudflare or ipify.

| Old Parameter |  | New Paramater |
| ------------- | - | ------------- |
| `API_KEY=key` | ✔️ | Use `CF_API_TOKEN=key` |
| `API_KEY_FILE=file` | ✔️ | Use `CF_API_TOKEN_FILE=file` |
| `ZONE=example.org` and `SUBDOMAIN=sub` | ✔️ | Use `DOMAINS=sub.example.org` directly |
| `PROXIED=true` | ✔️ | Same |
| `RRTYPE=A` | ✔️ | Both IPv4 and IPv6 are enabled by default; use `IP6_PROVIDER=none` to disable IPv6 |
| `RRTYPE=AAAA` | ✔️ | Both IPv4 and IPv6 are enabled by default; use `IP4_PROVIDER=none` to disable IPv4 |
| `DELETE_ON_STOP=true` | ✔️ | Same |
| `INTERFACE=iface` | ✔️ | Not required for `local` providers; we can handle multiple network interfaces |
| `CUSTOM_LOOKUP_CMD=cmd` | ❌ | _There is not even a shell in the minimum Docker image._ |
| `DNS_SERVER=server` | ❌ | _Only the Cloudflare server is supported._ |

</details>

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new).

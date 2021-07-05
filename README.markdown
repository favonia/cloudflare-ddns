# 🌟 CloudFlare DDNS

[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/favonia/cloudflare-ddns/Building%20and%20Pushing)](https://github.com/favonia/cloudflare-ddns/actions/workflows/build.yaml) [![Docker Pulls](https://img.shields.io/docker/pulls/favonia/cloudflare-ddns)](https://hub.docker.com/r/favonia/cloudflare-ddns) [![Docker Image Size (latest)](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)

An extremely small and fast tool to use CloudFlare as a DDNS service. The tool was originally inspired by [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) which has a similar goal.

## 📜 Highlights

* Ultra-small docker images (~2MB) with tiny footprints for all popular architectures.
* Ability to update multiple domains across different zones.
* Ability to remove stale records or remove records on exit (the latter is configurable).
* Ability to obtain IP addresses from either public servers or local network interfaces.
* Ability to enable or disable IPv4 and IPv6 individually.
* Full configurability via environment variables.
* Ability to pass API tokens via environment variables or files.
* Local caching to reduce CloudFlare API usage.

## 🛡️ Privacy and Security

* Public IP addresses are obtained via the [CloudFlare debugging interface](https://1.1.1.1/cdn-cgi/trace). This minimizes the impact on privacy as we will use the CloudFlare API to update DNS records anyways.
* The root privilege is immediately dropped after the program starts.
* The only two external dependencies (other than the Go standard library):
  1. [cloudflare/cloudflare-go](https://github.com/cloudflare/cloudflare-go): the official Go binding for CloudFlare API v4.
  2. [patrickmn/go-cache](https://github.com/patrickmn/go-cache): in-memory caching.

The CloudFlare binding provides robust handling of pagination and other tricky cases when using the CloudFlare API, and the in-memory caching reduces the API usage.

## 🐋 Deployment with Docker Compose

### Step 1: Updating the Compose File

Incorporate the following fragment into the compose file (typically `docker-compose.y[a]ml`).

```yaml
version: "3"
services:
  cloudflare-ddns:
    image: favonia/cloudflare-ddns:latest
    security_opt:
      - no-new-privileges:true
    network_mode: host
    environment:
      - PUID=1000
      - PGID=1000
      - CF_API_TOKEN
      - DOMAINS
      - PROXIED=true
```

⚠️ The setting `network_mode: host` is for IPv6. If you wish to keep the network separated from the host network, check out the proper way to [enable IPv6 support](https://docs.docker.com/config/daemon/ipv6/).

⚠️ The setting `PROXIED=true` instructs CloudFlare to cache webpages and hide your actual IP addresses. If you wish to bypass that, remove `PROXIED=true`. (The default value of `PROXIED` is `false`.)

💡 There is no need to use automatic restart (_e.g.,_ `restart: unless-stopped`) because the tool exits only when non-recoverable errors happen or when you manually stop it.

### Step 2: Updating the Environment File

Add these lines to your environment file (typically `.env`):
```bash
CF_API_TOKEN=<YOUR-CLOUDFLARE-API-TOKEN>
DOMAINS=www.example.org,www2.example.org
```

- The value of `CF_API_TOKEN` should be an API **token** (_not_ an API key), which can be obtained via the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Create a token with the **Zone - DNS - Edit** permission and copy the token into `.env`.

  ⚠️ The legacy API key authentication is intentionally _not_ supported. Please use the more secure API tokens.

- The value of `DOMAINS` should be a list of fully qualified domain names (without the final dots) separated by commas. For example, `a.org,www.a.org` means the tool should update the IP addresses of both the domains `a.org` and `www.a.org`. The domains do not have to be in the same zone---the tool will identify their zones automatically.

The tool should be up and running after these commands:
```bash
docker-compose pull cloudflare-ddns
docker-compose up --detach --build cloudflare-ddns
```
However, you might wish to follow the next step to customize it further.

### Step 3: Further Customization

Here are all the environment variables the tool recognizes.

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CF_API_TOKEN_FILE` | File paths | The path to the file that contains the token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `CF_API_TOKEN` | CloudFlare API tokens with the `DNS:Edit` permission | The token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `DELETE_ON_EXIT` | Boolean values | Whether managed DNS records should be deleted on exit | No | `false`
| `DOMAINS` | Comma-separated fully qualified domain names (but without the final periods) | All the domains this tool should update | Yes, and the list cannot be empty | N/A
| `IP4_POLICY` | `cloudflare`, `local`, and `unmanaged` | `cloudflare` means getting the public IP address via CloudFlare. `local` means getting the address via local network interfaces. `unmanaged` means leaving `A` records alone. | No | `cloudflare`
| `IP6_POLICY` | `cloudflare`, `local`, and `unmanaged` | (As above, but for IPv6 and `AAAA` records) | No | `cloudflare`
| `PGID` | POSIX group ID | The effective group ID the tool should assume (instead of being the `root`) | No | 1000
| `PROXIED` | Boolean values | Whether new DNS records should be proxied by CloudFlare | No | `false`
| `PUID` | POSIX user ID | The effective user ID the tool should assume (instead of being the `root`) | No | 1000
| `QUIET` | Boolean values | Whether the tool should reduce the logging | No | `false`
| `REFRESH_INTERVAL` | Any positive time duration, with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The refresh interval for the tool to re-check IP addresses and update DNS records (if necessary) | No | `5m0s` (5 minutes)
| `TTL` | Time-to-live (TTL) values | The TTL values used to create new DNS records | No | `1` (this means “automatic” to CloudFlare)

⚠️ In the above table, “boolean values” include `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False`. Other values will lead to errors. See [strconv.ParseBool](https://golang.org/pkg/strconv/#ParseBool).

⚠️ You will need `network_mode: host` for `IP4_POLICY=local` or `IP6_POLICY=local`, for otherwise the tool will detect the addresses inside the [bridge network set up by Docker](https://docs.docker.com/network/bridge/) instead of those in the host network.

After customizing the tool, run the following command to recreate the container:
```bash
docker-compose up --detach --build cloudflare-ddns
```

### Alternative Setup with Docker Secret

The tool can work with [Docker secrets](https://docs.docker.com/engine/swarm/secrets/) if you wish to provide the API token via `docker secret`. Pass the secret via `CF_API_TOKEN_FILE=/run/secrets/<secret_name>` instead of using the `CF_API_TOKEN` variable.

## 🛠️ Running without Docker Compose

[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/favonia/cloudflare-ddns)](https://golang.org/doc/install)

You will need the Go compiler, which can be installed via package managers in most Linux distros or the [official Go install page](https://golang.org/doc/install). After setting up the compiler, run the following command at the root of the source repository:
```bash
go run ./cmd/ddns.go
```
The program does not take arguments directly. Instead, it reads in environment variables. See the above section for the detailed explanation of those variables.

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Feel free to [open a GitHub issue](https://github.com/favonia/cloudflare-ddns/issues/new).

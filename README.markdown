# 🔁 `cloudflare-ddns` Reimplementation

This is a reimplementation of [timothymiller/cloudflare-ddns](https://github.com/timothymiller/cloudflare-ddns) (called “original tool” below). The main motivation was to have an implementation that (1) will not delete `A` and `AAAA` records that are not listed and (2) is configurable via only environment variables. After my DNS records have been purged a few times, and the pull requests to the upstream (by others) seem to be stalled, I decided to re-implement the tool.

## 🚧 Status of the Project

The code is working well for me, but the project is young and the design is subject to changes. That said, the compatible mode is intended to mimic the original tool and it is considered a bug if it does not. I should also point out that once the upstream has all the features I am looking for, I might archive this project in favor of using the original tool.

## 📜 Notable Changes

1. It will not delete any `A` or `AAAA` records unless the domains are explicitly listed.
2. It is configured primarily via environment variables.
3. It can also mimic the behavior of the original tool. See below.
4. It will respect `PGID` and `PUID` and drop the root privilege.
5. It can take fully qualified domain names and find the correct zone IDs via the CloudFlare API.
6. It can be configured to obtain IP addresses using local network interfaces.
7. It has a few technical improvements under the hood, such as the handling of pagination in the CloudFlare API (via the official Go binding [cloudflare/cloudflare-go](https://github.com/cloudflare/cloudflare-go)), (still incomplete) timeout mechanism, in-memory caching (via [patrickmn/go-cache](https://github.com/patrickmn/go-cache)), etc.

## 🛡️ Privacy and Security

The new implementation uses the same CloudFlare servers to determine the public IP addresses, and it drops the root privilege. That said, it does depend on the following two external libraries:
1. [patrickmn/go-cache](https://github.com/patrickmn/go-cache) for in-memory caching to reduce CloudFlare API usage.
2. [cloudflare/cloudflare-go](https://github.com/cloudflare/cloudflare-go) as the official Go library for CloudFlare API v4.

## 🛠️ Setup with Docker Compose

### 🤝 Compatible Mode

Use this option if you already have a working JSON configuration for the original tool and wish to keep it.

#### 🥾 Migration Step 1: Updating `docker-compose.yml`

1. Change `timothyjmiller/cloudflare-ddns:latest` to `ghcr.io/favonia/cloudflare-ddns-go:latest`.
2. Add `COMPATIBLE=true` to `environment`.

Here is a possible configuration after the migration:

```yaml
version: "3"
services:
  cloudflare-ddns-go:
    image: ghcr.io/favonia/cloudflare-ddns-go:latest
    security_opt:
      - no-new-privileges:true
    network_mode: host
    environment:
      - PUID=1000
      - PGID=1000
      - COMPATIBLE=true
    volumes:
      - ${PWD}/cloudflare-ddns/config.json:/config.json
```

⚠️ You should not need automatic restart (_e.g.,_ `restart: unless-stopped`) because the program should exit only when non-recoverable errors happen or when you manually stop it. Please consider reporting the bug if it exits for any other reasons.

⚠️ If you have a custom container name (_e.g.,_ `container_name: cloudflare-ddns`), it is recommended to change it so that Docker Compose will not be confused.

⚠️ The setting `network_mode: host` is for IPv6. If you wish to keep the network separated from the host network, check out the proper way to [enable IPv6 support](https://docs.docker.com/config/daemon/ipv6/).

The new tool should be up and running after these commands:
```bash
docker-compose pull cloudflare-ddns-go
docker-compose up --detach --remove-orphans --build cloudflare-ddns-go
```
(The `--remove-orphans` option is to remove the original tool.) However, you might wish to follow the next step to customize it further.

#### 🥾 Migration Step 2: Further Customization

The compatible mode recognizes the following environment variables:

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `COMPATIBLE` | Boolean values | Whether the program should mimic the original tool | Must be set to `true` to use the compatible mode | `false`
| `PGID` | POSIX Group ID | The effective group ID the program should assume (instead of being the `root`) | No | 1000
| `PUID` | POSIX User ID | The effective user ID the program should assume (instead of being the `root`) | No | 1000
| `QUIET` | Boolean values | Whether the program should reduce the logging | No | `false`

⚠️ In the above table, “boolean values” include `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False`. Other values will lead to errors. See [strconv.ParseBool](https://golang.org/pkg/strconv/#ParseBool).

### 🆕 New Mode (Using Environment Variables)

Use the new mode if compatibility with the original tool is not of your concern or you want to try out other features.

⚠️ The new mode can manage domains across different zones, but currently it only accepts one API token (while you can specify multiple API tokens in the compatible mode, each for a different zone). As a workaround, you can create one single API token with the permission to update DNS records in all the relevant zones.

#### Step 1: Updating `docker-compose.yml`

Incorporate the following fragment into your `docker-compose.yml` (or other equivalent files).

```yaml
version: "3"
services:
  cloudflare-ddns-go:
    image: ghcr.io/favonia/cloudflare-ddns-go:latest
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

⚠️ The setting `PROXIED=true` enables CloudFlare to cache your webpages and hide your actual IP addresses. If you wish to bypass that, remove the entry `PROXIED=true`. (The default value of `PROXIED` is `false`.)

#### Step 2: Updating `.env`

You should then add these lines in your `.env` file:
```bash
CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN
DOMAINS=www.example.org,www2.example.org
```

- The value of `CF_API_TOKEN` should be an API **token** (_not_ API key), which can be obtained via the [API Tokens page](https://dash.cloudflare.com/profile/api-tokens). Create an API token (_not_ API key) with the **Zone - DNS - Edit** permission and copy the token into `.env`.

  ⚠️ The legacy API key authentication is intentionally _not_ supported by the new format. You should use the more secure API tokens even in the JSON compatible mode.

- The value of `DOMAINS` should be a list of fully qualified domain names (without the final dots) separated by commas. For example, `github.com,www.github.com`. The domains do not have to be in the same zone---the tool will identify the correct zone of each domain.

The new tool should be up and running after these commands:
```bash
docker-compose pull cloudflare-ddns-go
docker-compose up --detach --build cloudflare-ddns-go
```
However, you might wish to follow the next step to customize it further.

#### Step 3: Further Customization

Here are all the environment variables the program checks. Note that, in the compatible mode (`COMPATIBLE=true`), only `PUID`, `PGID`, and `QUIET` (and `COMPATIBLE` itself) are functional; other variables are ignored.

| Name | Valid Values | Meaning | Required? | Default Value |
| ---- | ------------ | ------- | --------- | ------------- |
| `CF_API_TOKEN` | CloudFlare API tokens with the `DNS:Edit` permission | The token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `CF_API_TOKEN_FILE` | File paths | The path to the file that contains the token to access the CloudFlare API | Exactly one of `CF_API_TOKEN` and `CF_API_TOKEN_FILE` should be set | N/A |
| `COMPATIBLE` | Boolean values | Whether the program should mimic the original tool | Must be unset or set to `false` to use the new mode | `false`
| `DOMAINS` | Comma-separated fully qualified domain names (but without the final periods) | All the domains this tool should update | Yes, and the list cannot be empty | N/A
| `IP4_POLICY` | `cloudflare`, `local`, and `unmanaged` | `cloudflare` means getting the public IP address via CloudFlare. `local` means getting the address via local network interfaces. `unmanaged` means leaving `A` records alone. | No | `cloudflare`
| `IP6_POLICY` | `cloudflare`, `local`, and `unmanaged` | (As above, but for IPv6 and `AAAA` records) | No | `cloudflare`
| `PGID` | POSIX Group ID | The effective group ID the program should assume (instead of being the `root`) | No | 1000
| `PROXIED` | Boolean values | Whether new DNS records should be proxied by CloudFlare | No | `false`
| `PUID` | POSIX User ID | The effective user ID the program should assume (instead of being the `root`) | No | 1000
| `QUIET` | Boolean values | Whether the program should reduce the logging | No | `false`
| `REFRESH_INTERVAL` | Any positive time duration, with a unit, such as `1h` or `10m`. See [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration) | The refresh interval for the program to re-check IP addresses and update DNS records (if necessary) | No | `5m`
| `TTL` | Time-to-live (TTL) values | The TTL values used to create new DNS records | No | `1` (meaning automatic to CloudFlare)

⚠️ In the above table, “boolean values” include `1`, `t`, `T`, `TRUE`, `true`, `True`, `0`, `f`, `F`, `FALSE`, `false`, and `False`. Other values will lead to errors. See [strconv.ParseBool](https://golang.org/pkg/strconv/#ParseBool).

⚠️ When the policy is `unmanaged`, the tool will not remove records of the specified kinds (`A` records for IPv4 and `AAAA` records for IPv6). Those records are simply ignored and kept intact.

⚠️ You will need `network_mode: host` for `IP4_POLICY=local` or `IP6_POLICY=local`, for otherwise the program will detect the addresses in the [bridge network set up by Docker](https://docs.docker.com/network/bridge/) instead of those in the host network.

#### Alternative Setup with Docker Secret

The new mode can also work with [Docker secrets](https://docs.docker.com/engine/swarm/secrets/) if you wish to provide the API token via `docker secret`. Use `CF_API_TOKEN_FILE=/run/secrets/<secret_name>` instead of the `CF_API_TOKEN` variable to provide the token.

## 🛠️ Running without Docker Compose

You need the Go compiler, which can be installed via package managers in most Linux distros or the [official Go install page](https://golang.org/doc/install). After setting up the compiler, run the following command at the root of the source repository:
```bash
go run cmd/ddns.go
```
The program does not take arguments directly. Instead, it reads in environment variables. See the above section for the detailed explanation of those variables.

## 💖 Feedback

Questions, suggestions, feature requests, and contributions are all welcome! Please [open a new GitHub issue](https://github.com/favonia/cloudflare-ddns-go/issues/new) to initiate the discussion.

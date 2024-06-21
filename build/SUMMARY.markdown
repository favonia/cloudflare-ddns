<!-- This file is for Docker Hub and Artifact Hub -->

# 🌟 Cloudflare DDNS

[![Github Source](https://img.shields.io/badge/source-github-orange)](https://github.com/favonia/cloudflare-ddns)
[![Go Reference](https://pkg.go.dev/badge/github.com/favonia/cloudflare-ddns/.svg)](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/)
[![Codecov](https://img.shields.io/codecov/c/github/favonia/cloudflare-ddns)](https://app.codecov.io/gh/favonia/cloudflare-ddns)
[![Docker Image Size](https://img.shields.io/docker/image-size/favonia/cloudflare-ddns/latest)](https://hub.docker.com/r/favonia/cloudflare-ddns)
[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/6680/badge)](https://bestpractices.coreinfrastructure.org/projects/6680)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/favonia/cloudflare-ddns/badge)](https://securityscorecards.dev/viewer/?uri=github.com/favonia/cloudflare-ddns)

A small, feature-rich, and robust Cloudflare DDNS updater. [Read the full documentation on GitHub.](https://github.com/favonia/cloudflare-ddns/blob/main/README.markdown)

## Supported Tags

- `latest`: the latest released version
- `edge`: the latest development version
- `X`: the latest released version where the major version is `X` (e.g. `1`)
- `X.Y.Z` (where `X.Y.Z.` is a released version): a specific released version

🚨 If you are using [LXC (Linux Containers)](https://linuxcontainers.org/), it is known that the standard build may hang or halt (see [issue #707](https://github.com/favonia/cloudflare-ddns/issues/707)). If you encounter this problem, as a workaround, please use the following `-nocapdrop` variants to disable the explicit dropping of capabilities:

> - `latest-nocapdrop`: the latest released version
> - `edge-nocapdrop`: the latest development version
> - `X-nocapdrop`: the latest released version where the major version is `X` (e.g. `1`)
> - `X.Y.Z-nocapdrop` (where `X.Y.Z.` is a released version): a specific released version

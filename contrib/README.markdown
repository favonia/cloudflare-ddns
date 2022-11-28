# Community Contributions

## 📦 Sample systemd unit file

_(contributed by [Thomas @sumgryph](https://github.com/symgryph))_

⚠️ Docker, by default, enforces better isolation than systemd. Moreover, the sample systemd service unit file intentionally turns off several protections for efficiency and convenience. Using the official Docker images (along with its [restart policy](https://docs.docker.com/engine/reference/run/#restart-policies---restart)) is recommended for better security. You are at your own risks to use the following systemd service unit file.

- See [cloudflare-ddns.service](./cloudflare-ddns.service) for a sample systemd service unit file.
- See [cloudflare-ddns.service.env](./cloudflare-ddns.service.env) for a sample systemd environment file.

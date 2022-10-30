# Community Contributions

## 🧑‍💻 Sample Systemd unit file

_(contributed by [Thomas @sumgryph](https://github.com/symgryph) in 2022, licensed under Apache 2.0)_

⚠️ Favonia’s note: Docker, by default, enforces better isolation than Systemd. Moreover, the sample Systemd service unit file intentionally turns off several protections for efficiency and convenience. Therefore, using Docker (along with its [restart policy](https://docs.docker.com/engine/reference/run/#restart-policies---restart)) is recommended for better security if you can afford it.

- See [cloudflare-ddns.service](./cloudflare-ddns.service) for a sample Systemd service unit file.
- See [cloudflare-ddns.service.env](./cloudflare-ddns.service.env) for a sample Systemd environment file.

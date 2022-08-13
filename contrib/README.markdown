# Community Contributions

## ▶️ Start the service at boot

### 🐋 Docker-based

Docker's `always` restart policy will start the updater at boot along with the Docker daemon. You can specify `restart: always` in Docker Compose as in the [main README](../README.markdown) or use the [option `--restart always`](https://docs.docker.com/engine/reference/run/#restart-policies---restart) with `docker run`. Make sure the Docker daemon is started at boot.

### 🧑‍💻 Systemd-based

⚠️ Docker, by default, enforces better isolation than Systemd. Moreover, the sample Systemd service unit file intentionally turns off several protections for efficiency and convenience. Using the Docker-based method above is recommended for better security.

- See [cloudflare.service](cloudflare.service) for a sample Systemd service unit file.
- See [cloudflare.service.env](cloudflare.service.env) for a sample Systemd environment file.

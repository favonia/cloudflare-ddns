# Community Contributions

## ▶️ Start the service at boot

### 🐋 Docker-based

Docker's `always` restart policy (using `restart: always` as in the README or the [option `--restart always`](https://docs.docker.com/engine/reference/run/#restart-policies---restart) with `docker run`) will run the service at boot along with the Docker daemon. Make sure that the Docker daemon is started at boot.

### 🧑‍💻 Systemd-based

*Warnings:* Docker, by default, enforces better isolation than Systemd. Moreover, the sample Systemd service unit file intentionally turns off several protections for efficiency and convenience. Using the Docker-based method above is recommended for better security.

- See `cloudflare.service` for a sample Systemd service unit file.
- See `cloudflare.service.env` for a sample Systemd environment file.

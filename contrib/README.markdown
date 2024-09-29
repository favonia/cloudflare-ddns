# Community Contributions

## 📦 Sample systemd unit file

_(contributed by [Thomas @sumgryph](https://github.com/symgryph))_

⚠️ Docker, by default, enforces better isolation than systemd. Moreover, the sample systemd service unit file intentionally turns off several protections for efficiency and convenience. Using the official Docker images (along with its [restart policy](https://docs.docker.com/engine/reference/run/#restart-policies---restart)) is recommended for better security. You are at your own risks to use the following systemd service unit file.

- See [cloudflare-ddns.service](./linux/cloudflare-ddns.service) for a sample systemd service unit file.
- See [cloudflare-ddns.service.env](./linux/cloudflare-ddns.service.env) for a sample systemd environment file.

## 🐡 OpenBSD

_(contributed by [Brandon @skarekrow](https://github.com/skarekrow))_

To use:
- Copy the shipped [rc.d script](./openbsd/cloudflare_ddns) into `/etc/rc.d/`
- The rc.d script assumes a user called `_cloudflare_ddns` will be used. This is easily created by doing `useradd -s /sbin/nologin -d /var/empty _cloudflare_ddns`
- Create a `/etc/login.conf` entry for the daemon specifying the environment variables you wish to use:

```sh
cloudflare_ddns:\
        :setenv=CF_API_TOKEN=YOUR_TOKEN,DOMAINS=THE_DOMAINS_YOU_WISH_TO_USE,EMOJI=false:\
        :tc=daemon:

```

An important note is not to quote any of the values, as those will be literally interpreted. In this example `EMOJI` is false as the emojis clutter up the logs you will find of the daemon at `/var/log/daemon`
- Enable the daemon with `rcctl`, `rcctl enable cloudflare_ddns`

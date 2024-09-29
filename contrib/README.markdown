# Community Contributions

## 🆗 systemd

_(contributed by [Thomas @sumgryph](https://github.com/symgryph))_

- See [cloudflare-ddns.service](./systemd/cloudflare-ddns.service) for a sample systemd service unit file.
- See [cloudflare-ddns.service.env](./systemd/cloudflare-ddns.service.env) for a sample systemd environment file.

## 🐡 OpenBSD

_(contributed by [Brandon @skarekrow](https://github.com/skarekrow))_

To use:

1. Copy the shipped [rc.d script](./openbsd/cloudflare_ddns) into `/etc/rc.d/`
2. The `rc.d` script assumes a user called `_cloudflare_ddns` will be used. This is easily created by using `useradd`
   ```sh
   useradd -s /sbin/nologin -d /var/empty _cloudflare_ddns
   ```
3. Create a `/etc/login.conf` entry for the daemon, specifying the environment variables you wish to use:

   ```sh
   cloudflare_ddns:\
           :setenv=EMOJI=false,CF_API_TOKEN=YOUR-CLOUDFLARE-API-TOKEN,DOMAINS=YOUR-DOMAINS:\
           :tc=daemon:
   ```

   An important note is not to quote any of the values, as those will be literally interpreted. In this example `EMOJI` is false as the emojis clutter up the logs you will find of the daemon at `/var/log/daemon`

4. Enable the daemon with `rcctl`, `rcctl enable cloudflare_ddns`

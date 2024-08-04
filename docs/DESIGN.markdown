# 📄 Design and Roadmap

## Principles and Priorities

Be the 🌟 best DDNS updater 🌟 that [favonia](mailto:favonia+github@gmail.com) wants to use.

1. Support all features [favonia](mailto:favonia+github@gmail.com) wants, including emojis.

2. Be secure as much as possible and practical.

   - Prefer open security over security through obscurity.
   - Use code analysis, unit testing, fuzzing, and other techniques to discover bugs.
   - Detect common misconfigurations.

3. Be resilient; in particular, automatically recover from temporary errors, such as:

   1. Network outage or still being set up.
   2. Cloudflare servers being down.

4. Be efficient in network usage, CPU usage, memory usage, and whatnot.

5. Support other useful features that are easily maintainable.

### Architecture

The source code follows the [standard Go project layout](https://github.com/golang-standards/project-layout), where `/cmd/` holds the command-line interface and `/internal/` holds the internal packages. The updater is factored into many internal packages, each in charged of a small part of the program logic. See the [Go Reference](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/) for a detailed documentation of the code structure.

### Roadmap

See [Issues](https://github.com/favonia/cloudflare-ddns/issues) and [Milestones](https://github.com/favonia/cloudflare-ddns/milestones).

### Logging Message Convention

1. Cloudflare IDs (zone IDs, DNS record IDs, etc.) are already designed to use only “very safe” characters and should not be quoted. The formatter `%s` should be used instead of `%q`.

## Network Security Threat Model

### Assumptions

1. The adversary cannot access or tamper with end devices (your machine and Cloudflare’s servers) or their local networks.
2. The adversary can forge IP packets but cannot monitor, modify, or significantly delay existing IP packets.
3. The adversary may know your machine’s exact hardware, software, and configurations (except authentication credentials).

The goal is to stop the adversary from tricking the updater into updating managed domains with an incorrect IP address.

### Protection

The connection to Cloudflare’s servers is always protected by HTTPS, making it more resistant to forged IP packets. Many other DDNS updaters use simple DNS lookups to detect public IP addresses, which is less secure because of [DNS spoofing](https://en.wikipedia.org/wiki/DNS_spoofing).

### Unsafe Scenarios

Public IP addresses, by their own definition, depend on how other machines (in our case, Cloudflare’s servers) see the current machine over the internet. Therefore, if the adversary can control the network the updater can access, then it is impossible to secure it. This means one should avoid using this updater (or any DDNS updater) in the following scenarios:

1. Your machine is connected to the internet via unsafe Wi-Fi. This can include WPA2 Enterprise if your machine does not verify the identity of the RADIUS servers. In general, it is much more challenging to protect Wi-Fi connections.
2. The adversary can access networks close to Cloudflare’s servers and intercept your IP packets. Note that the adversary does not need to break HTTPS---HTTPS does not protect the source and target IP addresses.
3. The adversary can access the cable between your machine and the internet, or that you are within a country-scale firewall. (Although they can already redirect the traffic in this case, and thus whether the updater is secure or not is no longer relevant.)

There is no way to securely detect the intended public IP address in these scenarios. If you wish to be immune to these attacks, it is recommended to buy static IP addresses instead of using this updater (or any DDNS updater).

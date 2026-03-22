# Design Note: Network Security Model

Read when: changing public-IP detection security behavior or making security claims in docs.

Defines: the attacker model for public IP detection.

Does not define: general internet threat modeling beyond public-IP detection.

## Goal

Establish the attacker model and trust boundary for public-IP detection in this project.

## Core Model

The security model is about provider correctness at the raw-data boundary, not broad application security.

- The protected outcome is: the updater should not publish or enforce an attacker-chosen result in managed DNS records or managed WAF content.
- The relevant trust boundary is the network path used by public-IP detection.
- The updater aims to resist `off-path` attacks on public-IP detection.
- If an attacker is `on-path` for the route that determines how Cloudflare sees the machine, no DDNS updater can recover security through application-layer logic alone.

## Assumptions

This note uses `off-path` and `on-path` as in [RFC 3552, Section 3.5](https://www.rfc-editor.org/rfc/rfc3552.html): an `off-path` attacker can inject traffic without controlling the route, while an `on-path` attacker can observe or alter traffic on that route.

1. The adversary cannot access or tamper with the local machine or Cloudflare's servers.
2. The adversary is `off-path` with respect to the public-IP detection route.
3. The adversary may know the machine's exact hardware, software, and configuration, except authentication credentials.

## Protection

Connections to Cloudflare use HTTPS. Compared with public-IP detection based on ordinary DNS lookups, this is more resistant to `off-path` packet forgery and DNS spoofing.

## Unsafe Scenarios

If the adversary is `on-path` for the network path that determines how Cloudflare sees the machine, secure public-IP detection is impossible. Do not rely on the updater for protection in these scenarios:

1. The machine uses unsafe Wi-Fi, including WPA2 Enterprise without server identity verification, so the adversary can become `on-path`.
2. The adversary can intercept traffic near Cloudflare's servers and is therefore `on-path` for the relevant route.
3. The adversary can access the cable or broader network path between the machine and the internet, including a country-scale firewall, and is therefore `on-path`.

HTTPS does not protect source or destination IP addresses, so it does not remove these limits.

Under those conditions, any DDNS updater is fundamentally insecure. If immunity to these attacks is required, use static IP addresses instead.

## Scope Boundary

This note applies only to public-IP detection trust assumptions.

It does not define:

- Cloudflare account or credential security
- local host compromise
- shutdown cleanup semantics
- reconciliation behavior after raw-data detection succeeds

## Extension Points

- If future provider work changes which external systems are trusted for raw-data detection, update this note before expanding security claims elsewhere.
- If future documentation makes stronger guarantees about route selection, source-address binding, or hostile-network behavior, those claims must be checked against this attacker model.

# Design Note: Network Security Model

Read when: changing public-IP detection security behavior or making security claims in docs.

Defines: the attacker model for public IP detection.

Does not define: general internet threat modeling beyond public-IP detection.

## Goal

Prevent an adversary from causing the updater to publish an incorrect IP address for managed domains.

## Assumptions

1. The adversary cannot access or tamper with end devices, including the local machine and Cloudflare's servers, or their local networks.
2. The adversary can forge IP packets but cannot monitor, modify, or materially delay existing IP packets.
3. The adversary may know the machine's exact hardware, software, and configuration, except authentication credentials.

## Protection

Connections to Cloudflare use HTTPS. Compared with public-IP detection based on ordinary DNS lookups, this is more resistant to packet forgery and DNS spoofing.

## Unsafe Scenarios

If the adversary controls the network path that determines how Cloudflare sees the machine, secure public-IP detection is impossible. Do not rely on the updater for protection in these scenarios:

1. The machine uses unsafe Wi-Fi, including WPA2 Enterprise without server identity verification.
2. The adversary can intercept traffic near Cloudflare's servers.
3. The adversary can access the cable or broader network path between the machine and the internet, including a country-scale firewall.

HTTPS does not protect source or destination IP addresses, so it does not remove these limits.

## Consequence

Under those conditions, any DDNS updater is fundamentally insecure. If immunity to these attacks is required, use static IP addresses instead.

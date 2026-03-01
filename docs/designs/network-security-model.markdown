# Design Note: Network Security Model

This document records the network threat model for public IP detection.

## Goal

Prevent an adversary from tricking the updater into updating managed domains with an incorrect IP address.

## Assumptions

1. The adversary cannot access or tamper with end devices, including the local machine and Cloudflare's servers, or their local networks.
2. The adversary can forge IP packets but cannot monitor, modify, or significantly delay existing IP packets.
3. The adversary may know the machine's exact hardware, software, and configuration, except authentication credentials.

## Protection

Connections to Cloudflare are protected by HTTPS. This is more resistant to forged IP packets than public-IP detection approaches based on ordinary DNS lookups, which are more exposed to DNS spoofing.

## Unsafe Scenarios

If the adversary can control the network path that determines how Cloudflare sees the machine, then secure public-IP detection is impossible. In particular, this updater should not be relied on for protection in these scenarios:

1. The machine uses unsafe Wi-Fi, including WPA2 Enterprise without server identity verification.
2. The adversary can intercept traffic near Cloudflare's servers.
3. The adversary can access the cable or broader network path between the machine and the internet, including a country-scale firewall.

HTTPS does not protect source and destination IP addresses, so it does not remove these limits.

## Consequence

In those unsafe scenarios, using any DDNS updater is fundamentally insecure. If immunity to these attacks is required, use static IP addresses instead.

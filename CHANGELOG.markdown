# [1.14.0](https://github.com/favonia/cloudflare-ddns/compare/v1.13.2...v1.14.0) (2024-08-25)

This is a major release with many improvements! The most significant new feature is the ability to maintain a [WAF list](https://developers.cloudflare.com/waf/tools/lists/) of detected IP addresses; you can then refer to the list in your firewall rules. Please consult the [README](./README.markdown). The second most important update is to use a variant of [the Happy Eyeballs (Fast Fallback) algorithm](https://en.wikipedia.org/wiki/Happy_Eyeballs) to detect the blockage of 1.1.1.1. As the name of the new algorithm suggests, you should not notice any delay due to the detection, being happy. 😄

As a reminder, since 1.13.0, **the updater will no longer drop superuser privileges and `PUID` and `PGID` will be ignored.** Please use Docker’s built-in mechanism to drop privileges. The old Docker Compose template may grant the new updater unneeded privileges, which is not recommended. Please review the new template in [README](./README.markdown) that is simpler and more secure when combined with the new updater. In a nutshell, **remove the `cap_add` attribute and replace the environment variables `PUID` and `PGID` with the [`user: "UID:GID"` attribute](https://docs.docker.com/reference/compose-file/services/#user)**. If you are not using Docker Compose, chances are your system supports similar options under different names.

Other notable changes:

1. The global Cloudflare account ID will no longer be used when searching for DNS zones. `CF_ACCOUNT_ID` will be ignored.
2. To reduce network traffic and delay, the Cloudflare API token will no longer be additionally verified via the Cloudflare’s token verification API. Instead, the updater will locally check whether it looks like a valid [Bearer Token](https://oauth.net/2/bearer-tokens/).
3. Many parts of the [README](./README.markdown) have been rewritten to improve clarity and to document the support of WAF lists.

### Bug Fixes

- **api:** decouple account IDs from operations on DNS records ([#875](https://github.com/favonia/cloudflare-ddns/issues/875)) ([0fa1085](https://github.com/favonia/cloudflare-ddns/commit/0fa108504fed7d7e9bd6fce866c6983eaf420f2e))
- **api:** eliminate potential memory leak in caching ([#854](https://github.com/favonia/cloudflare-ddns/issues/854)) ([b9c7327](https://github.com/favonia/cloudflare-ddns/commit/b9c7327c84910d65b41f68dc74b413cd49b55f7d))
- **api:** make the updating algorithm more deterministic ([#864](https://github.com/favonia/cloudflare-ddns/issues/864)) ([b557c41](https://github.com/favonia/cloudflare-ddns/commit/b557c41e8873be4132992273356600662f32922f))
- **api:** remove global account ID and remote token verification ([#877](https://github.com/favonia/cloudflare-ddns/issues/877)) ([5a40ea7](https://github.com/favonia/cloudflare-ddns/commit/5a40ea7c21fd75b3829227b49362b886168dd107))
- **monitor:** retry connections to Uptime Kuma ([#890](https://github.com/favonia/cloudflare-ddns/issues/890)) ([8236410](https://github.com/favonia/cloudflare-ddns/commit/823641046c62e6a81838fa4e15fa57d4b15995a8))
- **setter:** do not quote DNS record IDs ([#851](https://github.com/favonia/cloudflare-ddns/issues/851)) ([fc8accb](https://github.com/favonia/cloudflare-ddns/commit/fc8accb45ec17fd4111a5920fccc30fdf5130cbe))
- **updater:** delete unmanaged IP addresses from WAF lists ([#885](https://github.com/favonia/cloudflare-ddns/issues/885)) ([bf0361c](https://github.com/favonia/cloudflare-ddns/commit/bf0361c85d449cdb703f9565f68dc8aaefb03323))
- **updater:** show the hint to disable a network when IP detection timeouts ([#859](https://github.com/favonia/cloudflare-ddns/issues/859)) ([bdf154c](https://github.com/favonia/cloudflare-ddns/commit/bdf154c1d7519d5a86ddeb0aa9fd8811bfe1f5d6)) ([#862](https://github.com/favonia/cloudflare-ddns/issues/862)) ([397e722](https://github.com/favonia/cloudflare-ddns/commit/397e722562257523716c37d81709289a126d4636))

### Features

- **api:** ability to update WAF lists ([#797](https://github.com/favonia/cloudflare-ddns/issues/797)) ([180bcd7](https://github.com/favonia/cloudflare-ddns/commit/180bcd7b48104b5bc779ffdad9dc08cdf32c4529))
- **provider:** Happy Eyeballs for 1.1.1.1 v.s. 1.0.0.1 ([#883](https://github.com/favonia/cloudflare-ddns/issues/883)) ([be0109b](https://github.com/favonia/cloudflare-ddns/commit/be0109b931c3dabebe73694f5205bba2ed22dda3))

# [1.13.2](https://github.com/favonia/cloudflare-ddns/compare/v1.13.1...v1.13.2) (2024-07-23)

This is a quick release to change the default user/group IDs of the shipped Docker images to 1000 (instead of 0, the `root`). The change will help _many_ people use the Docker images more safely. You are still encouraged to review whether the default ID 1000 is what you want. If you have already adopted the new recommended Docker template (in [README](./README.markdown)) with `user: ...` (not `PUID` or `PGID`) to explicitly set the user and group IDs, this release does not affect you.

# [1.13.1](https://github.com/favonia/cloudflare-ddns/compare/v1.13.0...v1.13.1) (2024-07-19)

This is a very minor release that improves the error messages produced by the new API token verifier (introduced in 1.13.0). See [#813](https://github.com/favonia/cloudflare-ddns/issues/813).

### Bug Fixes

- **domain:** fix incorrect parsing of `*.*.foo.bar` ([#809](https://github.com/favonia/cloudflare-ddns/issues/809)) ([9ccf9df](https://github.com/favonia/cloudflare-ddns/commit/9ccf9dfbf3d3ce0211c5af8c5345e809b1d7d266))

# [1.13.0](https://github.com/favonia/cloudflare-ddns/compare/v1.12.0...v1.13.0) (2024-07-16)

This is a major release that no longer drops superuser privileges. Please review the instructions in [README](./README.markdown) for the new recommended setup.

### BREAKING CHANGES

- **The updater will no longer drop superuser privileges and `PUID` and `PGID` will be ignored.** Please use Docker’s built-in mechanism to drop privileges. The old, hacky Docker Compose template will grant the new updater unneeded privileges, which is less secure and not recommended. Please review the new template in [README](./README.markdown) that is simpler and more secure when combined with the new updater. In a nutshell, **remove `cap_add` completely and add `user: ...`** as

  ```yaml
  user: "1000:1000"
  # Run the updater with a specific user ID and group ID (in that order).
  # You should change the two numbers based on your setup.
  ```

  If you have not, please add `cap_drop: [all]` to drop all Linux capabilities. You should probably remove `PUID` and `PGID` as well because they are now useless.

- In case you are using the `*-nocapdrop` Docker tags, they will no longer be maintained. The updater will no longer drop superuser privileges, and thus the `nocapdrop` builds are identical to the regular ones. Just use the regular Docker tags such as `latest`.

- The older versions used to add the comment “Created by cloudflare-ddns” to all newly created DNS records. Since this version, the comment has become configurable, but by default it is empty. To restore the old behavior, add the configuration `RECORD_COMMENT=Created by cloudflare-ddns` (or any comment you want to use).

### Features

- **api:** make record comment of new DNS records configurable using `RECORD_COMMENT` ([#783](https://github.com/favonia/cloudflare-ddns/issues/783)) ([b10c9a3](https://github.com/favonia/cloudflare-ddns/commit/b10c9a3653d01f16ebbdbce0bdee881b15329e71))
- **api:** recheck tokens if the network is temporarily down ([#790](https://github.com/favonia/cloudflare-ddns/issues/790)) ([15d1a5a](https://github.com/favonia/cloudflare-ddns/commit/15d1a5af7f5a95ee90d8c8eb9589cc23e9ba1c4b))
- **api:** smarter sanity checking ([#796](https://github.com/favonia/cloudflare-ddns/issues/796)) ([80dc7f4](https://github.com/favonia/cloudflare-ddns/commit/80dc7f4b7a28431aebe81630cdb2b7ace6f08d88))
- **cron:** show dates when needed ([#795](https://github.com/favonia/cloudflare-ddns/issues/795)) ([d1850b1](https://github.com/favonia/cloudflare-ddns/commit/d1850b17e797f1d9b9a06de5f28b4fbe25b32f33))
- **config:** recheck 1.1.1.1 and 1.0.0.1 some time later when probing fails (possibly because the network is temporarily down) ([#788](https://github.com/favonia/cloudflare-ddns/issues/788)) ([0983b06](https://github.com/favonia/cloudflare-ddns/commit/0983b06b4b308be5e0bfd16f2b101114d9008d56))
- **updater:** bail out faster when it times out ([#784](https://github.com/favonia/cloudflare-ddns/issues/784)) ([3b42131](https://github.com/favonia/cloudflare-ddns/commit/3b42131ab5afc8ba021677ba9325b05fde7c5243))

# [1.12.0](https://github.com/favonia/cloudflare-ddns/compare/v1.11.0...v1.12.0) (2024-06-28)

This is a major release with two significant improvements:

1. The updater can **send general updates via [shoutrrr](https://containrrr.dev/shoutrrr) now.**
2. The updater **supports non-Linux platforms now.** Linux capabilities are not supported on other platforms, but all other features should run as expected at least on Unix-like platforms.

There are also two notable improvements to the stock Docker images. Starting from this version:

1. Annotations are properly added to the Docker images, thanks to the updates to the upstream Docker toolchain.
2. A new Docker tag, `1`, is introduced to track the latest version with the major version `1`. I plan to develop `2.0.0` that may contain larger breaking changes. Sticking to `1` instead of `latest` now can avoid unexpected breakage in the future.

Note that the notification system was revamped to integrate [shoutrrr](https://containrrr.dev/shoutrrr). As a result, messages may have been reworded.

### Bug Fixes

- add annotations to Docker images ([#651](https://github.com/favonia/cloudflare-ddns/issues/651)) ([dd04d0d](https://github.com/favonia/cloudflare-ddns/commit/dd04d0d287abe313fa1c446e129da1281a0e1362)) ([#652](https://github.com/favonia/cloudflare-ddns/issues/652)) ([fe2ed00](https://github.com/favonia/cloudflare-ddns/commit/fe2ed0037ebba39bb7f2a4c594f58d462439a76f)) ([#653](https://github.com/favonia/cloudflare-ddns/issues/653)) ([56748eb](https://github.com/favonia/cloudflare-ddns/commit/56748eb00753abaac2b725ffafb80bfd4cb59fd8)) ([#659](https://github.com/favonia/cloudflare-ddns/issues/659)) ([687ccaa](https://github.com/favonia/cloudflare-ddns/commit/687ccaa7a8606f06d4d9e203603791b51f9bee98)), closes [#454](https://github.com/favonia/cloudflare-ddns/issues/454)
- limit the number of bytes read from an HTTP response (for extra security) ([#629](https://github.com/favonia/cloudflare-ddns/issues/629)) ([d64e8d4](https://github.com/favonia/cloudflare-ddns/commit/d64e8d4da44fb1d497cc871385061fb009e5ead8))
- **monitor:** force non-empty error messages for Uptime Kuma ([#624](https://github.com/favonia/cloudflare-ddns/issues/624)) ([a9bce5c](https://github.com/favonia/cloudflare-ddns/commit/a9bce5c56df6dbabe9ca4ae973a92cadfef6735b)) ([#774](https://github.com/favonia/cloudflare-ddns/issues/774)) ([df565b9](https://github.com/favonia/cloudflare-ddns/commit/df565b94199ad97642438dd4eb5f9168193c981f))
- **provider:** trim the response of `url:URL` (generic provider) before parsing it ([#709](https://github.com/favonia/cloudflare-ddns/issues/709)) ([48edb15](https://github.com/favonia/cloudflare-ddns/commit/48edb15b4be0b3c9e74cfe712fc9f1e01c4ef537))

### Features

- **cron:** show the far start time during countdown ([#761](https://github.com/favonia/cloudflare-ddns/issues/761)) ([39c659a](https://github.com/favonia/cloudflare-ddns/commit/39c659a29ff358dee7927148c13c45f2eea90265))
- **droproot:** support non-Linux platforms ([#733](https://github.com/favonia/cloudflare-ddns/issues/733)) ([a93b6ab](https://github.com/favonia/cloudflare-ddns/commit/a93b6abca56ab0809dc56a84e64e665d1fdede12))
- **monitor:** prioritize error messages ([#622](https://github.com/favonia/cloudflare-ddns/issues/622)) ([2f653ca](https://github.com/favonia/cloudflare-ddns/commit/2f653caddbb9d948110e79988bfb8523fe7cfccc))
- **monitor:** send `Failed to detect IPv4/6 address` to monitors ([#620](https://github.com/favonia/cloudflare-ddns/issues/620)) ([f1793ad](https://github.com/favonia/cloudflare-ddns/commit/f1793addc44f28f060732c2bd9add08d7d23018e))
- **notifier:** embed shoutrrr ([#633](https://github.com/favonia/cloudflare-ddns/issues/633)) ([61f42a0](https://github.com/favonia/cloudflare-ddns/commit/61f42a04b665ffb710b3cc9fb326dbe6ada53125)) ([#640](https://github.com/favonia/cloudflare-ddns/issues/640)) ([817125e](https://github.com/favonia/cloudflare-ddns/commit/817125ef46511d24372f54afb67a90b7547bb532)) ([#762](https://github.com/favonia/cloudflare-ddns/issues/762)) ([c09e2b2](https://github.com/favonia/cloudflare-ddns/commit/c09e2b2ed965b9028a37d21d6a318fca48f539ca)) ([#768](https://github.com/favonia/cloudflare-ddns/issues/768)) ([9cdfec3](https://github.com/favonia/cloudflare-ddns/commit/9cdfec393a3ef24803fc7a1280515c1fda72102e)) ([#772](https://github.com/favonia/cloudflare-ddns/issues/772)) ([b8d4604](https://github.com/favonia/cloudflare-ddns/commit/b8d4604109ae6521266032cd5fc81fd05578fc7a)), closes [#532](https://github.com/favonia/cloudflare-ddns/issues/532)
- **setter:** print `(cached)` for results based on cached API responses ([#776](https://github.com/favonia/cloudflare-ddns/issues/776)) ([1bcbbf0](https://github.com/favonia/cloudflare-ddns/commit/1bcbbf058741594e13ce6ec382edc908b383f112))

# [1.11.0](https://github.com/favonia/cloudflare-ddns/compare/v1.10.1...v1.11.0) (2023-10-23)

This release adds the experimental support of Uptime Kuma.

### BREAKING CHANGES

- `UPDATE_CRON=@disabled` is deprecated; use `UPDATE_CRON=@once` instead

### Features

- add support of Uptime Kuma ([#600](https://github.com/favonia/cloudflare-ddns/issues/600)) ([c68eeeb](https://github.com/favonia/cloudflare-ddns/commit/c68eeeb8472a8e6cc61e3ffb6dd5925d008ffa81)) ([#605](https://github.com/favonia/cloudflare-ddns/issues/605)) ([e65531a](https://github.com/favonia/cloudflare-ddns/commit/e65531ae09e08a1b0f25e0d4d8287eb136cacf52))
- introduce `UPDATE_CRON=@once` ([#607](https://github.com/favonia/cloudflare-ddns/issues/607)) ([aa57602](https://github.com/favonia/cloudflare-ddns/commit/aa57602626c2f9b4bccbab330a61643d8fd0b2e8))

# [1.10.1](https://github.com/favonia/cloudflare-ddns/compare/v1.10.0...v1.10.1) (2023-09-17)

### Bug Fixes

- The updater will now keep existing [record comments](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) when updating IP addresses. Previously, it would incorrectly erase them. This was a known bug in 1.10.0, and was fixed by [fixing the upstream library `cloudflare-go`.](https://github.com/cloudflare/cloudflare-go/pull/1393)

# [1.10.0](https://github.com/favonia/cloudflare-ddns/compare/v1.9.4...v1.10.0) (2023-09-10)

### Features

- **provider:** implement the new custom provider `url:URL` ([#560](https://github.com/favonia/cloudflare-ddns/issues/560)) ([6318512](https://github.com/favonia/cloudflare-ddns/commit/63185129ab33329cc77e2aac3a9e8a393db7b8cd)) and ([#576](https://github.com/favonia/cloudflare-ddns/issues/576)) ([d80784e](https://github.com/favonia/cloudflare-ddns/commit/d80784e50ff4b07a35aa00e98492db2ccb9678e5))

### KNOWN BUGS

- The current updater will erase existing [record comments](https://developers.cloudflare.com/dns/manage-dns-records/reference/record-attributes/) when updating the IP address due to an unfortunate design in an upstream library. This bug seems to affect all updaters of version 1.8.3 or later (I didn’t really check them). I am attempting to address the bug by fixing the upstream library, but if that does not work, a hack to keep existing record comments will be added to the updater. The bug is tracked by [GitHub issue #559](https://github.com/favonia/cloudflare-ddns/issues/559).

# [1.9.4](https://github.com/favonia/cloudflare-ddns/compare/v1.9.3...v1.9.4) (2023-06-07)

This is a minor update that comes with a [nice bugfix from go-retryablehttp 0.7.4](https://github.com/hashicorp/go-retryablehttp/pull/194).

# [1.9.3](https://github.com/favonia/cloudflare-ddns/compare/v1.9.2...v1.9.3) (2023-06-06)

This version will automatically switch to 1.0.0.1 when 1.1.1.1 appears to be blocked or intercepted by your ISP or your router. The blockage and interception should not happen, but many ISPs and routers were misconfigured to use 1.1.1.1 as a private IP. The new updater tries to work around it by switching to 1.0.0.1. The long-term solution is to notify your ISP or upgrade your router.

### Bug Fixes

- **setter:** quote DNS record IDs to prevent injection attacks ([#502](https://github.com/favonia/cloudflare-ddns/issues/502)) ([d978c68](https://github.com/favonia/cloudflare-ddns/commit/d978c68e61d3d73a927c95d1aff779133b998c3b))

### Features

- **config:** display a message when 1.0.0.1 also doesn't work ([#495](https://github.com/favonia/cloudflare-ddns/issues/495)) ([5f5602d](https://github.com/favonia/cloudflare-ddns/commit/5f5602d5965350f5a7ef2a3eefc136002d73a2a4))
- **config:** check 1.1.1.1 only when IPv4 is used ([#494](https://github.com/favonia/cloudflare-ddns/issues/494)) ([d0db1be](https://github.com/favonia/cloudflare-ddns/commit/d0db1be82e6f9df0e894f9e6ea001b83701dfd81))
- **config:** use 1.0.0.1 when 1.1.1.1 is blocked ([#491](https://github.com/favonia/cloudflare-ddns/issues/491)) ([8b9d160](https://github.com/favonia/cloudflare-ddns/commit/8b9d1603c92bf42995a9bd4febfa5506086ea190))

# [1.9.2](https://github.com/favonia/cloudflare-ddns/compare/v1.9.1...v1.9.2) (2023-04-11)

### Bug Fixes

- a better quiet mode notice ([#430](https://github.com/favonia/cloudflare-ddns/issues/430)) ([1248527](https://github.com/favonia/cloudflare-ddns/commit/124852774ba8dc497158e1b47c03d8496d71cfde))

# [1.9.1](https://github.com/favonia/cloudflare-ddns/compare/v1.9.0...v1.9.1) (2023-03-15)

This version is a hotfix for running the updater in quiet mode in a system (_e.g.,_ Portainer) that expects _some_ output from the updater. Unfortunately, the new quiet mode introduced in 1.9.0 was _too_ quiet for those systems. This version will print out something to make them happy.

### Bug Fixes

- print out something in the quiet mode ([#427](https://github.com/favonia/cloudflare-ddns/issues/427)) ([a1f7d07](https://github.com/favonia/cloudflare-ddns/commit/a1f7d074fe6a485858a84ede54352475d59d358d))

# [1.9.0](https://github.com/favonia/cloudflare-ddns/compare/v1.8.4...v1.9.0) (2023-03-15)

### Features

- **cron:** add the option `UPDATE_CRON=@disabled` to disable cron ([#411](https://github.com/favonia/cloudflare-ddns/issues/411)) ([a381c5a](https://github.com/favonia/cloudflare-ddns/commit/a381c5a5d6df12a1d10cafeb74fe63cce7f18558))

### BREAKING CHANGES

- the quiet mode will no longer print the version and the information about superuser privileges (unless there are errors) ([#415](https://github.com/favonia/cloudflare-ddns/issues/415)) ([92a4462](https://github.com/favonia/cloudflare-ddns/commit/92a44628ab459c5eb715ecbddb9cb84ea36c927e))

### Other Notes

The feature to disable cron is experimental. The intention is to use another mechanism to manage the update schedule and run the updater. The quiet mode was made quieter so that repeated execution of the updater will not lead to excessive logging with non-errors.

# [1.8.4](https://github.com/favonia/cloudflare-ddns/compare/v1.8.3...v1.8.4) (2023-03-03)

This release comes with no user-visible changes. It was compiled by version 1.20.1 of Go (instead of 1.20) and was shipped with version 0.62.0 of the [cloudflare-go library](https://github.com/cloudflare/cloudflare-go/) that [fixed a bug about proxy settings](https://github.com/cloudflare/cloudflare-go/pull/1222). I believe the bug does not affect the updater, but there's no reason not to use the fixed version. 😄

# [1.8.3](https://github.com/favonia/cloudflare-ddns/compare/v1.8.2...v1.8.3) (2023-02-11)

### Bug Fixes

- **api:** optimize network traffic for UpdateRecord ([#358](https://github.com/favonia/cloudflare-ddns/issues/358)) ([64bd670](https://github.com/favonia/cloudflare-ddns/commit/64bd670602d031745bd168ee22e57e7ea7e525b3))

### Features

- **api:** annotate newly created DNS records ([#366](https://github.com/favonia/cloudflare-ddns/issues/366)) ([09bbaf4](https://github.com/favonia/cloudflare-ddns/commit/09bbaf4bcb8be0fd0865e7f5f998e53f6dcb0741)): this uses the newly available [DNS record comments](https://blog.cloudflare.com/dns-record-comments/)

### Other Notes

Upgraded Go to version 1.20.

# [1.8.2](https://github.com/favonia/cloudflare-ddns/compare/v1.8.1...v1.8.2) (2023-01-02)

This release is shipped with a newer [golang.org/x/net/http2](https://pkg.go.dev/golang.org/x/net/http2) that fixes [CVE-2022-41717](https://pkg.go.dev/vuln/GO-2022-1144). The updater should not be affected by the CVE, but a vulnerability scanner might still mark the updater or the image as insecure. This release should shut those scanners. No new features are added.

# [1.8.1](https://github.com/favonia/cloudflare-ddns/compare/v1.8.0...v1.8.1) (2022-12-05)

A minor update with internal refactoring and insignificant UI adjustments.

# [1.8.0](https://github.com/favonia/cloudflare-ddns/compare/v1.7.2...v1.8.0) (2022-11-25)

### Bug Fixes

- **provider:** deprecate possibly unmaintained ipify ([#270](https://github.com/favonia/cloudflare-ddns/issues/270)) ([69b5d70](https://github.com/favonia/cloudflare-ddns/commit/69b5d706cf0c1e6696685d1569934c67676242d1))
- **monitor:** correct printf format string ([#265](https://github.com/favonia/cloudflare-ddns/issues/265)) ([0740d61](https://github.com/favonia/cloudflare-ddns/commit/0740d6186d870a9a77159c0e52454ba4f82fb08a))
- **setter:** improve monitor messages ([#273](https://github.com/favonia/cloudflare-ddns/issues/273)) ([c0599f6](https://github.com/favonia/cloudflare-ddns/commit/c0599f6b45975a7cf6607211c878e348b5f110a0))

### Features

- **monitor:** improve Healthchecks integration ([#272](https://github.com/favonia/cloudflare-ddns/issues/272)) ([b24cce6](https://github.com/favonia/cloudflare-ddns/commit/b24cce669f4625a566320e102490402f18d49c58))
- **pp:** add an option to disable emojis ([#280](https://github.com/favonia/cloudflare-ddns/issues/280)) ([95d0c67](https://github.com/favonia/cloudflare-ddns/commit/95d0c6723116b86870cf73427f109716d486e27e))
- **provider:** auto retry IP detection ([#290](https://github.com/favonia/cloudflare-ddns/issues/290)) ([de4d730](https://github.com/favonia/cloudflare-ddns/commit/de4d73070c04ab8ead9e05457c2f8d8bec871b94))
- **provider:** warn about the use of weak PRNGs ([#254](https://github.com/favonia/cloudflare-ddns/issues/254)) ([ae2c866](https://github.com/favonia/cloudflare-ddns/commit/ae2c8664dc7caaf06558f224a37c608495e4ac78))

### BREAKING CHANGES

- The `ipify` provider is deprecated.

# [1.7.2](https://github.com/favonia/cloudflare-ddns/compare/v1.7.1...v1.7.2) (2022-11-07)

- This version was published to retract all prior versions on <https://pkg.go.dev>. There are no observable changes.

# [1.7.1](https://github.com/favonia/cloudflare-ddns/compare/v1.7.0...v1.7.1) (2022-10-23)

### Features

- replace `text/template` with an in-house parser ([#222](https://github.com/favonia/cloudflare-ddns/issues/222) and [#233](https://github.com/favonia/cloudflare-ddns/issues/233)) ([21301de](https://github.com/favonia/cloudflare-ddns/commit/21301dec842f52db51c7af54ed8a48a5ad16082e) and [0b34720](https://github.com/favonia/cloudflare-ddns/commit/0b34720c1cddd537e1133b2d4f1f902e4c04821c))

### BREAKING CHANGES

- `TTL` no longer supports templates; only `PROXIED` supports them
- existing templates that worked for 1.7.0 will stop working; see README.markdown for detailed documentation

# [1.7.0](https://github.com/favonia/cloudflare-ddns/compare/v1.6.1...v1.7.0) (2022-09-06)

### Features

- **config:** accept templates for PROXIED and TTL ([#214](https://github.com/favonia/cloudflare-ddns/issues/214)) ([a78b96b](https://github.com/favonia/cloudflare-ddns/commit/a78b96bf44dcbdbc2cfcd82eee18c4baffba6d77))
- warn about incorrect TTL values ([#206](https://github.com/favonia/cloudflare-ddns/issues/206)) ([c6a7ea8](https://github.com/favonia/cloudflare-ddns/commit/c6a7ea89e3651b5d770d9348e99aec8e34120356))

### BREAKING CHANGES

- experimental `PROXIED_DOMAINS` and `NON_PROXIED_DOMAINS` introduced in 1.6.0 are no longer supported; they are replaced by the new experimental template system

# [1.6.1](https://github.com/favonia/cloudflare-ddns/compare/v1.6.0...v1.6.1) (2022-08-13)

### Bug Fixes

- **api:** accept non-active zones ([#203](https://github.com/favonia/cloudflare-ddns/issues/203)) ([06a8af6](https://github.com/favonia/cloudflare-ddns/commit/06a8af6e712635aae97540c230fd5a60a1100818))
- prefer shorter messages ([#204](https://github.com/favonia/cloudflare-ddns/issues/204)) ([7212559](https://github.com/favonia/cloudflare-ddns/commit/7212559496f7583325ee2d59a1c69bfa9bd7a5eb))

# [1.6.0](https://github.com/favonia/cloudflare-ddns/compare/v1.5.1...v1.6.0) (2022-08-12)

### Bug Fixes

- **config:** don't print "Monitors: (none)" ([#201](https://github.com/favonia/cloudflare-ddns/issues/201)) ([472aef4](https://github.com/favonia/cloudflare-ddns/commit/472aef46bca4c3599e1c75fed9c09419fd43c04d))
- **config:** print wildcard domains with prefix `*.` ([#198](https://github.com/favonia/cloudflare-ddns/issues/198)) ([caf370c](https://github.com/favonia/cloudflare-ddns/commit/caf370c257e693b1550860486e80a5a629bdb884))
- **config:** separate printed domains with comma ([#200](https://github.com/favonia/cloudflare-ddns/issues/200)) ([d658d58](https://github.com/favonia/cloudflare-ddns/commit/d658d58845a2b56b291a7d0d3df567ebc90cc0f2))
- **setter:** print out better error messages ([#195](https://github.com/favonia/cloudflare-ddns/issues/195)) ([68007f8](https://github.com/favonia/cloudflare-ddns/commit/68007f803d819653610d0932db84ca2a9d710f6c))

### Features

- add systemd unit file for non-Docker users ([#139](https://github.com/favonia/cloudflare-ddns/issues/139)) ([bbe48ae](https://github.com/favonia/cloudflare-ddns/commit/bbe48ae14ca36c1e6dac877550211af384f17f87))
- per-domain proxy settings ([#202](https://github.com/favonia/cloudflare-ddns/issues/202)) ([8b456cf](https://github.com/favonia/cloudflare-ddns/commit/8b456cfc407d43b5389a62952c3a5aad9f5c4756))

### Others

- use Go 1.19 ([#193](https://github.com/favonia/cloudflare-ddns/issues/193)) ([889a7c2](https://github.com/favonia/cloudflare-ddns/commit/889a7c25314921b40191ece578958bb28cb000af))

# [1.5.1](https://github.com/favonia/cloudflare-ddns/compare/v1.5.0...v1.5.1) (2022-06-23)

### Bug Fixes

- **file:** fix arguments of pp.Errorf ([55c5988](https://github.com/favonia/cloudflare-ddns/commit/55c598831b15094b7edd9928bc89bba0cc1a048b))

# [1.5.0](https://github.com/favonia/cloudflare-ddns/compare/v1.4.0...v1.5.0) (2022-06-18)

### Bug Fixes

- **file:** accept absolute paths ([#173](https://github.com/favonia/cloudflare-ddns/issues/173)) ([79bcd9b](https://github.com/favonia/cloudflare-ddns/commit/79bcd9b6b48f1557652459d6156a75503b8bc462))
- always ping "starting" before "exiting" ([c05082a](https://github.com/favonia/cloudflare-ddns/commit/c05082a60cb959ece83a28de4f357d40941ac377))

### BREAKING CHANGES

- rename `IP4/6_POLICY` to `IP4/6_PROVIDER` ([#167](https://github.com/favonia/cloudflare-ddns/issues/167)) ([1dcd4e4](https://github.com/favonia/cloudflare-ddns/commit/1dcd4e4a23148bf2ba163dbb823cf60dad8e7e8f))

# [1.4.0](https://github.com/favonia/cloudflare-ddns/compare/v1.3.0...v1.4.0) (2022-05-09)

### Bug Fixes

- **api:** revise the token verification message ([#104](https://github.com/favonia/cloudflare-ddns/issues/104)) ([209afdc](https://github.com/favonia/cloudflare-ddns/commit/209afdcc52b95bf10f1f077b6ffdd5bfcee62a0b))
- updating was wrongly restricted by detection timeout ([#159](https://github.com/favonia/cloudflare-ddns/issues/159)) ([b3fc809](https://github.com/favonia/cloudflare-ddns/commit/b3fc8091f75617659f8463a0748317a1048b8d39))

### Features

- **monitor:** support healthchecks.io ([#160](https://github.com/favonia/cloudflare-ddns/issues/160)) ([f83f5fb](https://github.com/favonia/cloudflare-ddns/commit/f83f5fbf26855d41e1beb3efe77f2a3476bab541))

# [1.3.0](https://github.com/favonia/cloudflare-ddns/compare/v1.2.0...v1.3.0) (2021-11-15)

### Bug Fixes

- **api:** keep leading dots after the beginning `*` is removed ([#97](https://github.com/favonia/cloudflare-ddns/issues/97)) ([bb2da38](https://github.com/favonia/cloudflare-ddns/commit/bb2da3845e0ac9a6d1b48c2242755e40d0fab944))

### Features

- **detector:** re-implement the cdn-cgi/trace parser and make it the new default policy; deprecate “cloudflare” in favor of “cloudflare.doh” or “cloudflare.trace” ([#102](https://github.com/favonia/cloudflare-ddns/issues/102)) ([ebf0639](https://github.com/favonia/cloudflare-ddns/commit/ebf06395c341b97a9f2e3c8618cc21eed2365b3d))

# [1.2.0](https://github.com/favonia/cloudflare-ddns/compare/v1.1.0...v1.2.0) (2021-10-18)

### Bug Fixes

- **api:** remove all trailing dots ([#95](https://github.com/favonia/cloudflare-ddns/issues/95)) ([f4ec041](https://github.com/favonia/cloudflare-ddns/commit/f4ec041372e1dd4839106124b241f7b4a9aa0b15))

### Features

- **api:** support wildcard domains ([#94](https://github.com/favonia/cloudflare-ddns/issues/94)) ([feafcf4](https://github.com/favonia/cloudflare-ddns/commit/feafcf47a7b1bad8be44235d04c3804babb67c51))

# [1.1.0](https://github.com/favonia/cloudflare-ddns/compare/v1.0.0...v1.1.0) (2021-08-23)

### Bug Fixes

- **api:** always use ASCII forms of domains ([#61](https://github.com/favonia/cloudflare-ddns/issues/61)) ([befb0a9](https://github.com/favonia/cloudflare-ddns/commit/befb0a92b9f1578c27112902eb61ff5d93499a13)) ([#58](https://github.com/favonia/cloudflare-ddns/issues/58)) ([55da36f](https://github.com/favonia/cloudflare-ddns/commit/55da36fbc238b24944bd066a9cdb892b4c68f29f))
- **api:** cache results of ListRecords ([8680b4b](https://github.com/favonia/cloudflare-ddns/commit/8680b4ba05886efe10a4201ca4b7e023f2befe53))
- **api:** more robust splitter for domains ([#42](https://github.com/favonia/cloudflare-ddns/issues/42)) ([12648db](https://github.com/favonia/cloudflare-ddns/commit/12648db232fe104cd8d37e141c29e44314554285))
- **cmd:** actually display version ([d619c02](https://github.com/favonia/cloudflare-ddns/commit/d619c02f7fb3d27aaf90b3e575f512984bbf5633))
- **config:** fix indentation in ReadEnv ([7c615a7](https://github.com/favonia/cloudflare-ddns/commit/7c615a715b10b59a8f8944a7ec82056d0ac40cf4))
- **config:** redo parsing ([#36](https://github.com/favonia/cloudflare-ddns/issues/36)) ([0801a45](https://github.com/favonia/cloudflare-ddns/commit/0801a4553d56039fe6b535df8518b4f5bdf0ba9a))
- **pp:** use less angry emojis for non-fatal errors ([020d326](https://github.com/favonia/cloudflare-ddns/commit/020d32638e08726a0c10d29682f723364b3035ec))

### Features

- **config:** display timezone ([#40](https://github.com/favonia/cloudflare-ddns/issues/40)) ([7bec30e](https://github.com/favonia/cloudflare-ddns/commit/7bec30ea44c7fae0e50c5e78992af57bb49ccc3b))
- **config:** re-enable UPDATE_TIMEOUT ([#72](https://github.com/favonia/cloudflare-ddns/issues/72)) ([805c62c](https://github.com/favonia/cloudflare-ddns/commit/805c62c82cab3f93590cbd8831f680bb18bfbed3)), closes [#34](https://github.com/favonia/cloudflare-ddns/issues/34)

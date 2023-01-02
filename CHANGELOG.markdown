# [1.8.2](https://github.com/favonia/cloudflare-ddns/compare/v1.8.1...1.8.2) (2023-01-02)

This release is shipped with the updated [golang.org/x/net/http2](https://pkg.go.dev/golang.org/x/net/http2) which fixes [CVE-2022-41717](https://pkg.go.dev/vuln/GO-2022-1144). The updater should not be affected by the CVE, but a vulnerability scanner might still mark the updater or the image as insecure. No new features are added in this release.

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

## [1.7.2](https://github.com/favonia/cloudflare-ddns/compare/v1.7.1...v1.7.2) (2022-11-07)

- This version was published to retract all prior versions on <https://pkg.go.dev>. There are no observable changes.

## [1.7.1](https://github.com/favonia/cloudflare-ddns/compare/v1.7.0...v1.7.1) (2022-10-23)

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

## [1.6.1](https://github.com/favonia/cloudflare-ddns/compare/v1.6.0...v1.6.1) (2022-08-13)

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

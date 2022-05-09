# [1.4.0](https://github.com/favonia/cloudflare-ddns/compare/v1.3.0...v1.4.0) (2022-05-09)


### Bug Fixes

* **api:** revise the token verification message ([#104](https://github.com/favonia/cloudflare-ddns/issues/104)) ([209afdc](https://github.com/favonia/cloudflare-ddns/commit/209afdcc52b95bf10f1f077b6ffdd5bfcee62a0b))
* updating was wrongly restricted by detection timeout ([#159](https://github.com/favonia/cloudflare-ddns/issues/159)) ([b3fc809](https://github.com/favonia/cloudflare-ddns/commit/b3fc8091f75617659f8463a0748317a1048b8d39))


### Features

* **monitor:** support healthchecks.io ([#160](https://github.com/favonia/cloudflare-ddns/issues/160)) ([f83f5fb](https://github.com/favonia/cloudflare-ddns/commit/f83f5fbf26855d41e1beb3efe77f2a3476bab541))



# [1.3.0](https://github.com/favonia/cloudflare-ddns/compare/v1.2.0...v1.3.0) (2021-11-15)


### Bug Fixes

* **api:** keep leading dots after the beginning `*` is removed ([#97](https://github.com/favonia/cloudflare-ddns/issues/97)) ([bb2da38](https://github.com/favonia/cloudflare-ddns/commit/bb2da3845e0ac9a6d1b48c2242755e40d0fab944))


### Features

* **detector:** re-implement the cdn-cgi/trace parser ([#102](https://github.com/favonia/cloudflare-ddns/issues/102)) ([ebf0639](https://github.com/favonia/cloudflare-ddns/commit/ebf06395c341b97a9f2e3c8618cc21eed2365b3d))



# [1.3.0](https://github.com/favonia/cloudflare-ddns/compare/v1.2.0...v1.3.0) (2021-11-15)


### Bug Fixes

* **api:** keep leading dots after the beginning `*` is removed ([#97](https://github.com/favonia/cloudflare-ddns/issues/97)) ([bb2da38](https://github.com/favonia/cloudflare-ddns/commit/bb2da3845e0ac9a6d1b48c2242755e40d0fab944))


### Features

* **detector:** re-implement the cdn-cgi/trace parser and make it the new default policy; deprecate “cloudflare” in favor of “cloudflare.doh” or “cloudflare.trace” ([#102](https://github.com/favonia/cloudflare-ddns/issues/102)) ([ebf0639](https://github.com/favonia/cloudflare-ddns/commit/ebf06395c341b97a9f2e3c8618cc21eed2365b3d))



# [1.2.0](https://github.com/favonia/cloudflare-ddns/compare/v1.1.0...v1.2.0) (2021-10-18)


### Bug Fixes

* **api:** remove all trailing dots ([#95](https://github.com/favonia/cloudflare-ddns/issues/95)) ([f4ec041](https://github.com/favonia/cloudflare-ddns/commit/f4ec041372e1dd4839106124b241f7b4a9aa0b15))


### Features

* **api:** support wildcard domains ([#94](https://github.com/favonia/cloudflare-ddns/issues/94)) ([feafcf4](https://github.com/favonia/cloudflare-ddns/commit/feafcf47a7b1bad8be44235d04c3804babb67c51))



# [1.1.0](https://github.com/favonia/cloudflare-ddns/compare/v1.0.0...v1.1.0) (2021-08-23)


### Bug Fixes

* **api:** always use ASCII forms of domains ([#61](https://github.com/favonia/cloudflare-ddns/issues/61)) ([befb0a9](https://github.com/favonia/cloudflare-ddns/commit/befb0a92b9f1578c27112902eb61ff5d93499a13)) ([#58](https://github.com/favonia/cloudflare-ddns/issues/58)) ([55da36f](https://github.com/favonia/cloudflare-ddns/commit/55da36fbc238b24944bd066a9cdb892b4c68f29f))
* **api:** cache results of ListRecords ([8680b4b](https://github.com/favonia/cloudflare-ddns/commit/8680b4ba05886efe10a4201ca4b7e023f2befe53))
* **api:** more robust splitter for domains ([#42](https://github.com/favonia/cloudflare-ddns/issues/42)) ([12648db](https://github.com/favonia/cloudflare-ddns/commit/12648db232fe104cd8d37e141c29e44314554285))
* **cmd:** actually display version ([d619c02](https://github.com/favonia/cloudflare-ddns/commit/d619c02f7fb3d27aaf90b3e575f512984bbf5633))
* **config:** fix indentation in ReadEnv ([7c615a7](https://github.com/favonia/cloudflare-ddns/commit/7c615a715b10b59a8f8944a7ec82056d0ac40cf4))
* **config:** redo parsing ([#36](https://github.com/favonia/cloudflare-ddns/issues/36)) ([0801a45](https://github.com/favonia/cloudflare-ddns/commit/0801a4553d56039fe6b535df8518b4f5bdf0ba9a))
* **pp:** use less angry emojis for non-fatal errors ([020d326](https://github.com/favonia/cloudflare-ddns/commit/020d32638e08726a0c10d29682f723364b3035ec))


### Features

* **config:** display timezone ([#40](https://github.com/favonia/cloudflare-ddns/issues/40)) ([7bec30e](https://github.com/favonia/cloudflare-ddns/commit/7bec30ea44c7fae0e50c5e78992af57bb49ccc3b))
* **config:** re-enable UPDATE_TIMEOUT ([#72](https://github.com/favonia/cloudflare-ddns/issues/72)) ([805c62c](https://github.com/favonia/cloudflare-ddns/commit/805c62c82cab3f93590cbd8831f680bb18bfbed3)), closes [#34](https://github.com/favonia/cloudflare-ddns/issues/34)

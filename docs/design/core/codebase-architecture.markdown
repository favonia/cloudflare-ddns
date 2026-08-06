# Design Note: Codebase Architecture

Read when: changing package boundaries, configuration flow, or the composition root in `cmd/ddns/`.

Defines: the repository layout used by the Go code, the main internal package boundaries, the configuration carriers, and the `cmd/ddns/` composition root.

## Repository Layout

- `cmd/` holds executable entry points. `cmd/ddns/` is the updater process entry point and composition root.
- `internal/` holds non-exported application packages.
- `docs/` holds project documentation.
- `build/` holds release and packaging support files.
- `contrib/` holds integration examples and platform-specific helper files.
- `test/` holds cross-package tests such as the fuzzer.

The repository root also contains module metadata, top-level user documentation, and packaging files such as `go.mod`, `Dockerfile`, and `README.markdown`.

## Internal Package Boundaries

- `internal/config/` owns updater environment input, cross-field validation, built configuration carriers, config printing, and reporter bootstrap helpers.
- `internal/provider/` owns provider constructors and the runtime `provider.Provider` interface. Protocol-specific implementations live in `internal/provider/protocol/`.
- `internal/api/` owns Cloudflare authentication, handle construction, ownership policy, and Cloudflare-facing read and write operations.
- `internal/setter/` owns DNS-record and WAF-list reconciliation through `api.Handle`.
- `internal/updater/` owns update-round orchestration by combining `config.UpdateConfig`, `provider.Provider`, and `setter.Setter`.
- `internal/heartbeat/` and `internal/notifier/` own outbound runtime reporting services.
- `internal/domain/`, `internal/domainexp/`, and `internal/ipnet/` own shared domain and IP types and parsing logic.
- `internal/cron/`, `internal/signal/`, `internal/file/`, `internal/pp/`, and `internal/sliceutil/` provide supporting runtime utilities.
- `internal/mocks/` holds generated test doubles.

See the [Go package reference](https://pkg.go.dev/github.com/favonia/cloudflare-ddns/) for package-level API structure.

## Configuration Lifecycle

`config.DefaultRaw()` creates the baseline updater settings, `(*RawConfig).ReadEnv()` overlays updater environment variables onto that structure, and `(*RawConfig).BuildConfig()` validates cross-field invariants and derives the runtime carriers.

- `RawConfig` holds parsed environment inputs before cross-field validation and derivation.
- `BuiltConfig` groups the three runtime carriers and excludes reporter services.
- `HandleConfig` holds the validated inputs for Cloudflare handle construction: `api.Auth` plus `api.HandleOptions`, including cache expiration and handle ownership policy.
- `LifecycleConfig` holds validated schedule and shutdown settings used by the main process loop.
- `UpdateConfig` holds validated provider, domain, WAF, timeout, and write-side settings used during reconciliation.

`SetupPP()` and `SetupReporters()` are parallel bootstrap paths, not parts of `RawConfig`. Reporter services are constructed separately from `BuiltConfig` and passed as runtime dependencies.

Updater-behavior environment reads are confined to `internal/config/`. Runtime packages below that boundary consume built config values or constructed services instead of reading environment state directly.

## Composition Root

`cmd/ddns/ddns.go` is the production composition root.

- It sets up output formatting with `config.SetupPP()`.
- It constructs heartbeat and notifier services with `config.SetupReporters()` before reading updater config.
- It reads and builds updater config, prints the built settings, constructs the Cloudflare handle from `builtConfig.Handle`, and then constructs `setter.Setter`.
- The main loop keeps `LifecycleConfig` for scheduling and shutdown decisions and passes `UpdateConfig` plus `setter.Setter` into `updater.UpdateIPs()` and `updater.FinalDeleteIPs()`.
- Startup-failure reporting and steady-state reporting use the same heartbeat and notifier instances because the reporters are created before config, handle, and setter setup.

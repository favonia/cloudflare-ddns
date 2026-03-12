# ✨️ Contributing to `cloudflare-ddns`

First of all, thank you for your contribution! 🤗

## 🕵️ Security Reports

If you are reporting a security vulnerability, stop here. Do not use public issues or pull requests. See [Security Policy](https://github.com/favonia/cloudflare-ddns/security/policy) and follow the steps there.

## 🙋 Raise an Issue

If you are raising an issue, please include your configuration (environment variables). Remember to redact your Cloudflare API tokens, Healthchecks URLs, or other credentials. If you are editing out those credentials from a screenshot, please use the ugliest solid blocks instead of translucent blocks or blurring filters. (Otherwise, you should consider those credentials leaked and regenerate them!)

## ⬇️ Make a Pull Request

If you have code ready, great! Please make a pull request. Here are a few things to pay attention to.

1. Check the license.

   Roughly speaking, you agree to license your contribution under [Apache 2.0 with LLVM exceptions](../LICENSE), and you assert that you have full power to license your contribution under [Apache 2.0 with LLVM exceptions](../LICENSE). Please refer to the [license text](../LICENSE) for the precise legal terms.

2. Test your code!

   You should add tests for new features or bug fixes (to detect regression). You should improve testing coverage as much as possible and practical. You can run all the tests locally by executing `go test ./...`.

3. Follow the coding style.

   We rely on the meta-linter `golangci-lint` to enforce coding styles. Whatever `golangci-lint` says is our coding style (unless the maintainer says otherwise). You can wait for GitHub Actions (usually very fast) or run `golangci-lint run` locally. Your must pass all automatic checks cleanly, possibly except the coverage checks, unless the maintainer says your code is okay.

   If you need an inline `//nolint`, treat it as a local exception rather than a second lint policy. Broad lint policy belongs in [`.golangci.yaml`](../.golangci.yaml); repository-wide guidance for inline suppressions lives in [docs/designs/guides/lint-suppressions.markdown](designs/guides/lint-suppressions.markdown).

   For package boundaries, config flow, and composition-root wiring, follow [docs/designs/core/codebase-architecture.markdown](designs/core/codebase-architecture.markdown). Keep local message wording and code comments close to the code unless the rule is reused broadly.

4. Keep design documentation durable.

   If you edit `docs/designs/`, write for future readers who need the current intended design, not pull-request context. Describe present semantics directly, avoid rollout phrasing such as "currently" or "first implementation", anchor history to explicit versions or changes, move project-wide policy into shared design notes, and tighten wording to keep design notes high-signal and precise.

   Default to updating an existing design note or using a code comment, test, or contributor doc. Create a new design note only when no smaller home fits and the rule is durable enough to matter across unrelated future tasks. See [docs/designs/README.markdown](designs/README.markdown) for the design-note admission rule.

5. Keep README reader-friendly.

   `README.markdown` is for a broad user audience, including not-so-technical users.

   Keep examples and nearby prose simple, concrete, and beginner-friendly. Move deep details into tables, reference sections, or clearly marked technical notes.

   If you edit `README.markdown`, follow the shared guidance in [docs/designs/guides/readme-writing.markdown](designs/guides/readme-writing.markdown).

6. Prefer explicit doc scopes in commit titles.

   Use `docs(README)` for README-only documentation changes and `docs(CHANGELOG)` for changelog-only updates. Keep these scopes uppercase to make release-relevant doc commits easy to scan.

7. Be selective with exhaustive third-party struct literals.

   For API query/list parameter structs (for example, record listing filters), set only the fields that are part of the intended selector and use a tight local `//nolint:exhaustruct` if needed.

   For mutating API parameter structs (create/update/delete style requests), keep literals exhaustive so newly added upstream fields are surfaced and reviewed intentionally.

   Exhaustive literals are not a substitute for contract mapping. For mutating calls, always map each in-scope desired-state field explicitly from the method contract instead of relying on zero values. If a mutator intentionally preserves some remote fields, document that preserve-vs-mutate contract in comments at the interface and implementation entry points.

Once you make the pull request, the maintainer will check your code and decide what to do. We loosely follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) and will update your pull request’s title. Don’t worry too much about commit messages as long as it’s clear what individual commits do.

## 🧑‍⚖️ Who’s in Charge

[favonia](mailto:favonia+github@gmail.com) is currently the sole maintainer and makes all final decisions.

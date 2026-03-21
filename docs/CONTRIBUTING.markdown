# ✨️ Contributing to `cloudflare-ddns`

First of all, thank you for your contribution! 🤗

## 🕵️ Security Reports

If you are reporting a security vulnerability, stop here. Do not use public issues or pull requests. See [Security Policy](https://github.com/favonia/cloudflare-ddns/security/policy) and follow the steps there.

## 🙋 Raise an Issue

If you are raising an issue, include your configuration (environment variables) when it helps explain the problem. Redact Cloudflare API tokens, Healthchecks URLs, and other credentials carefully. If you edit a screenshot, use solid blocks instead of translucent masks or blur filters.

## ⬇️ Make a Pull Request

If you have code ready, please make a pull request. Before you do:

1. Check the license.

   Roughly speaking, you agree to license your contribution under [Apache 2.0 with LLVM exceptions](../LICENSE), and you assert that you have the right to do so. See the [license text](../LICENSE) for the precise terms.

2. Test your code.

   Add or update tests for new features and bug fixes when practical. You can run the full test suite locally with `go test ./...`.

3. Follow the coding style.

   We use `golangci-lint` to enforce the coding style. You can wait for GitHub Actions or run `golangci-lint run` locally. If you need a local `//nolint`, treat it as an exception, not a second lint policy. Repository guidance for Go inline `//nolint` lives in [docs/designs/guides/go-lint-suppressions.markdown](designs/guides/go-lint-suppressions.markdown).

4. Put documentation in the right place.

   Use [docs/README.markdown](README.markdown) as the map for public docs. If you edit `docs/designs/`, write for future developers rather than pull-request context, and prefer updating an existing note. If a detail matters only for one local change, keep it near the code or tests instead of growing `docs/designs/`. The detailed design-note rules live in [docs/designs/README.markdown](designs/README.markdown). If you edit `README.markdown`, keep it beginner-friendly and follow [docs/designs/guides/readme-writing.markdown](designs/guides/readme-writing.markdown). Maintainer release workflow lives in [docs/release-workflow.markdown](release-workflow.markdown).

5. Open the pull request.

   Keep the summary focused on behavior and include test evidence when relevant. We loosely follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/), and the maintainer may normalize the pull-request title.

## 🧑‍⚖️ Who’s in Charge

[favonia](mailto:favonia+github@gmail.com) is currently the sole maintainer and makes all final decisions.

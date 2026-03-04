# ✨ Contributing to `cloudflare-ddns`

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

4. Keep design documentation durable.

   If you edit `docs/designs/`, write for a future reader who needs to understand the current intended design rather than the current pull request. Describe present semantics directly, avoid vague rollout-relative wording such as "currently" or "first implementation", use explicit versions or changes when historical context matters, and move project-wide policy into the project-wide design notes instead of inventing a separate local policy in each feature note.

Once you make the pull request, the maintainer will check your code and decide what to do. We loosely follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) and will update your pull request’s title. Don’t worry too much about commit messages as long as it’s clear what individual commits do.

## 🧑‍⚖️ Who’s in Charge

[favonia](mailto:favonia+github@gmail.com) is currently the sole maintainer and makes all final decisions.

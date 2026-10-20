# Development and CI

## Toolchain

- Go 1.25.8 minimum; CI also tests Go 1.27.2.
- Terraform 1.11 minimum; CI covers 1.11.4 and 1.16.6.
- Node 24.18.0, as pinned in `.node-version`.
- GCC/Clang and CGO for Go race checks.

The documentation site uses **VitePress 2.0.0-alpha.20**, an explicitly pinned
v2 prerelease. `package-lock.json` locks its dependency tree. Node is only needed
for the documentation site, not for building or running the Go provider.

## Local checks

```sh
go mod download
go mod verify
go build ./...
go vet ./...
go test ./... -count=1 -timeout 10m
```

CLI acceptance runs when Terraform is on PATH or `TF_ACC_TERRAFORM_PATH` points
to an installed executable. Tests do not download Terraform. Without one, CLI
cases explicitly skip. CI supplies the executable and runs the full suite with
`go test -race` and CGO enabled.

All provider tests use loopback HTTP stubs and real SDK decoding. The test
transport rejects non-loopback requests. `TF_ACC` alone never enables live
requests, and the repository includes no live test suite. No cloud credentials
are needed in CI.

## Documentation

```sh
npm ci
npm run docs:dev
```

Use `npm run docs:build` for a production build, including dead-link checks and
the local search index. Preview it with `npm run docs:preview` at the displayed
URL, including the `/terraform-provider-hexcore/` base path.

Guides live in `docs/guide/`; VitePress configuration lives in
`docs/.vitepress/config.mts`. The schema reference is generated from the Go schema:

```sh
go generate ./...
git diff --exit-code -- docs templates
```

Edit schema descriptions, examples, or `templates/` rather than generated schema
pages. Keep Terraform on PATH for generation. The VitePress rewrite maps the
registry-compatible `docs/index.md` to `/reference/provider.html`, leaving the
registry's expected source layout intact. The authored homepage becomes the site root.

## Branches and pull requests

`develop` is the default branch. Normal PRs target it and use Conventional Commit
titles. Release/hotfix promotion targets `master`; use `develop`, `release/*` or
`hotfix/*` branches from this repository. Both protected branches require the
aggregate `CI` check and PR review-thread resolution. Solo-maintainer PRs do not
require an additional reviewer. Direct pushes and force pushes are blocked.

CI checks formatting, module consistency, build, vet, race tests, CLI acceptance,
generated docs, the VitePress build, workflow syntax and a six-platform unsigned
GoReleaser snapshot. Actions use pinned commit SHAs and read-only permissions by
default. The documentation site deploys from `develop` through GitHub Pages.

See [release process](./releasing) before preparing a version.

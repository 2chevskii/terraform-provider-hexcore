# Contributing

Open normal pull requests against **develop**. Use Conventional Commit titles,
for example `fix(provider): retain state after an ambiguous read`. `master` is
reserved for release/hotfix promotion through a merge-commit PR. Both branches
require the aggregate `CI` check; direct and force pushes are blocked.

## Provider changes

Read [.agents/SPEC.md](.agents/SPEC.md) before changing behavior. Keep HexCore
protocol handling in `github.com/2chevskii/go-hexcore`, preserve string IDs and
units, and never infer absence from a singular 403/404. Never replay an uncertain
mutation. Add focused contract or lifecycle tests when behavior changes.

```sh
go mod download
go build ./...
go vet ./...
go test ./... -count=1 -timeout 10m
```

Install Terraform >= 1.11 on PATH or set `TF_ACC_TERRAFORM_PATH` for full offline
CLI acceptance. Run `go test -race ./...` with CGO and a supported compiler.
For OpenTofu 1.12.6, set `TF_ACC_TERRAFORM_PATH` to its executable and
`TF_ACC_PROVIDER_HOST=registry.terraform.io`. CI covers both CLIs.
Tests use local stubs and need no cloud credentials. Live checks and spending
require separate, explicit authorization; do not add always-on billable tests.

## Documentation

Use Node from `.node-version`, then `npm ci` and `npm run docs:dev`. Edit authored
guides in `docs/guide/`. Schema pages in `docs/resources/`, `docs/data-sources/`
and `docs/index.md` are generated: change Go schema descriptions, `examples/` or
`templates/`, then run `go generate ./...` with Terraform on PATH. Commit generated
output and confirm a second generation leaves no diff.

`npm run docs:build` checks the VitePress v2 build and internal links. Keep its
exact prerelease and lockfile intentional. The registry-compatible source layout
is reused by the website; do not replace the schema generator with hand-edited
reference pages. See the [development guide](docs/guide/development.md).

## Releases and reports

See the [release process](docs/guide/releasing.md). Do not push release tags or
publish a version without the maintainer's release instruction. Report security
issues privately following [SECURITY.md](SECURITY.md). Public bug reports must use
synthetic configuration, IDs and names; omit credentials, state/plans, billing
details and full live responses.

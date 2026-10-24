# Terraform provider for HexCore

[![CI](https://github.com/2chevskii/terraform-provider-hexcore/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/2chevskii/terraform-provider-hexcore/actions/workflows/ci.yml)
[![Documentation](https://github.com/2chevskii/terraform-provider-hexcore/actions/workflows/pages.yml/badge.svg?branch=develop)](https://2chevskii.github.io/terraform-provider-hexcore/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A community Terraform provider for HexCore, built with Terraform Plugin Framework
(protocol 6) and the published
[`go-hexcore v1.0.0`](https://github.com/2chevskii/go-hexcore/releases/tag/v1.0.0).
Discover catalogs and manage standard or custom VDS with explicit import metadata,
bounded waits and state retention on uncertain API outcomes.

**Status:** public source and offline acceptance coverage. No Terraform Registry
version has been released, and the provider lifecycle has not been verified
against live HexCore. The intended source is `registry.terraform.io/2chevskii/hexcore`.
Requires Terraform **>= 1.11**; OpenTofu **1.12.6** is covered by the offline suite.

## Documentation

Read the **[VitePress documentation site](https://2chevskii.github.io/terraform-provider-hexcore/)**:

- [Getting started](https://2chevskii.github.io/terraform-provider-hexcore/guide/getting-started.html)
- [VDS resource and import](docs/resources/vds.md)
- [Configuration and catalogs](docs/guide/configuration.md)
- [Recovery and troubleshooting](docs/guide/troubleshooting.md)
- [Verification and limitations](docs/guide/status.md)
- [Release process](docs/guide/releasing.md)

The schema reference in `docs/` is generated with `terraform-plugin-docs` and
retains the Terraform Registry layout. VitePress v2 adds authored guides,
navigation and local search over the same reference pages. Examples are in
[`examples/`](examples/); catalog slugs in examples are placeholders.

## Local use before a registry release

Build with Go >= 1.26.10:

```sh
go build -o terraform-provider-hexcore .
```

On Windows, use `terraform-provider-hexcore.exe`. Configure a temporary
`dev_overrides` CLI file pointing to the binary directory and set
`TF_CLI_CONFIG_FILE` to it, as shown in the
[getting-started guide](docs/guide/getting-started.md). Skip `terraform init` for
this unpublished provider; the development override supplies local discovery.

Supply a raw `HEXCORE_TOKEN` through the environment and use:

```hcl
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hexcore = {
      source = "registry.terraform.io/2chevskii/hexcore"
    }
  }
}

provider "hexcore" {}
```

Explicit `token` overrides the environment. Empty or unknown configured values
never fall back. `.env` files are not loaded and Configure makes no network calls.
The full source address selects the same provider in Terraform and OpenTofu;
OpenTofu otherwise resolves a short address through `registry.opentofu.org`.

## Development

```sh
go mod download
go build ./...
go vet ./...
go test ./... -count=1 -timeout 10m
go generate ./...
npm ci
npm run docs:dev
```

The docs use the exact VitePress **2.0.0-alpha.20** prerelease and Node **24.18.0**.
Use `npm run docs:build` for a production build and `npm run docs:preview` to inspect
it locally. Terraform must be on PATH for schema generation.

Full offline CLI acceptance needs an installed Terraform executable on PATH or
`TF_ACC_TERRAFORM_PATH`. Without one those cases explicitly skip. Tests never
download Terraform and need no HexCore credentials. CI tests Go 1.26.10/Terraform
1.11.4, Go 1.27.3/Terraform 1.16.6, and Go 1.27.3/OpenTofu 1.12.6, including
CGO race checks, compiled-provider discovery, no-op plans after apply/import,
and cleanup isolation. It also verifies
generated docs, builds VitePress, checks reachable Go vulnerabilities, lints workflows
and builds six release archives.

Read [CONTRIBUTING.md](CONTRIBUTING.md) for branch and documentation conventions.
Normal PRs target `develop`; release/hotfix promotion targets `master`.
Both protected branches require CI and prohibit direct pushes.

## Lifecycle and verification boundary

Provisioning changes replace the whole VDS. Only timeout and deletion-policy
changes update local state without an API mutation. Creation retains an accepted
ID before waiting; uncertain writes are never blindly replayed. Only a successful
full VDS list omitting the exact ID proves absence. A singular 403/404 does not.
Graceful deletion never escalates to a power cut automatically.

Use an ephemeral variable for `password_wo` and increment the non-secret
`password_wo_version` to request replacement. Credential delivery when the optional
password is omitted is undocumented and unverified. Import requires original
image/region/size declarations because the SDK cannot read all provisioning selectors.

The [SDK's dated live report](https://github.com/2chevskii/go-hexcore/blob/v1.0.0/docs/live-verification.md)
is prior SDK evidence. Disk attachment/resize and graceful reboot/password-reset
completion remain unresolved, and disk deletion is undocumented. Those operations
and the other deferred features are not exposed. This repository's tests use
loopback stubs; `TF_ACC` does not authorize live requests or spending.

See [VERIFICATION.md](VERIFICATION.md) for the original offline implementation
evidence and [SECURITY.md](SECURITY.md) for private vulnerability reporting.
Distributed under the [MIT License](LICENSE).

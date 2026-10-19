# Terraform provider for HexCore

Local implementation of `registry.terraform.io/2chevskii/hexcore`, built with
Terraform Plugin Framework (protocol 6) and the published
[`go-hexcore v1.0.0`](https://github.com/2chevskii/go-hexcore/releases/tag/v1.0.0).
The registry address is a target: this provider has not been published.

Requires Go >= 1.25.8 for development and Terraform >= 1.11 for use. OpenTofu
compatibility has not been tested. The current surface is four catalog data
sources, a singular VDS data source, and a standard/custom VDS resource.

## Configuration and examples

Supply a raw token through `HEXCORE_TOKEN` and use `provider "hexcore" {}`.
Explicit `token` overrides the environment; empty/unknown values do not fall
back. `.env` files are never loaded. Use a secret manager or shell environment
to supply credentials; do not put tokens in configuration files.

See [provider configuration](docs/index.md), [VDS lifecycle and import](docs/resources/vds.md),
and [examples](examples). Example catalog slugs are placeholders: select current
available values using the data sources. `endpoint` is an SDK base URL override
for development and local testing. Provider construction makes no API requests.

Creation inputs require replacement. `delete_mode` and `timeouts` only update
local state. Defaults are 15m create, 2m read, 20m delete, and 5s polling; these
are project defaults, not vendor SLAs. Each HTTP request has the SDK's 30s bound
and the remaining operation context. Graceful shutdown never escalates to force.

The provider retains an accepted VDS ID before waiting. After an uncertain
creation response without an ID, inspect the account manually and use structured
import before applying again. Names are not idempotency keys. Only a successful
complete account VDS list omitting the exact ID establishes absence; ambiguous
403/404 responses never remove state on their own.

`password_wo` is a sensitive creation-only write-only argument. Supply it with
an ephemeral Terraform variable and increment `password_wo_version` to request
replacement. A password edit alone produces no diff. The SDK cannot retrieve
passwords; they never belong in state. Public API documentation makes the
creation password optional but does not specify credential delivery when omitted.
That behavior remains unverified; do not assume email delivery. No reset-password
operation is exposed.

## Local development

```sh
go build -o terraform-provider-hexcore .
go test ./...
go vet ./...
go generate ./...
```

On Windows, build `terraform-provider-hexcore.exe`. The CLI acceptance tests run
automatically when Terraform is on PATH or `TF_ACC_TERRAFORM_PATH` points to its
executable. Otherwise they explicitly skip; use the commands below to require
the full suite. Tests do not download Terraform or use HexCore credentials.

```powershell
$env:TF_ACC_TERRAFORM_PATH = (Get-Command terraform).Source
go test ./... -count=1 -timeout 10m
go test -race ./... -count=1 -timeout 10m # requires CGO and a supported C compiler
```

The tests use real SDK decoding against loopback HTTP stubs and protocol-6
factories. A transport guard rejects non-loopback requests. `TF_ACC` does not
enable live tests; no live test suite is included. See [verification and remaining
work](VERIFICATION.md) for the exact handoff evidence.

For local discovery, create a **temporary**, machine-specific CLI configuration:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/2chevskii/hexcore" = "/absolute/directory/containing/binary"
  }
  direct {}
}
```

Point `TF_CLI_CONFIG_FILE` to it, then run `terraform providers schema -json`
or `terraform validate` in an example directory. Development overrides bypass
installation; do not run `terraform init` to fetch this unpublished provider.
Do not commit the override. `go generate` uses the pinned `tfplugindocs` tool;
put Terraform on PATH for documentation generation.

## Scope and upstream limitations

Offline passing tests do not establish live provider compatibility. No HexCore
requests or billable resources are authorized by the implementation task.
The [SDK verification report](https://github.com/2chevskii/go-hexcore/blob/v1.0.0/docs/live-verification.md)
records earlier SDK evidence, including unresolved graceful reboot/password-reset
completion and disk attachment/resize failures. Disk deletion is undocumented;
the provider exposes no disk lifecycle, reboot, password reset, power-state,
networking, SSH-key, user-data, monitoring, or action resources.

Publication, release signing, a GitHub remote and live spending require a later
instruction. Future live tests must require explicit session authorization,
budget and catalog targets, `HEXCORE_LIVE=1`, `HEXCORE_TOKEN`, and a separate
`HEXCORE_LIVE_WRITES=1` gate for mutations. Only test-owned VDS may be cleaned up.
No credentials or state from the sibling SDK are needed or copied.

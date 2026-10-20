# Implementation handoff — 2026-10-19

This is the original offline milestone report. See the public repository's
[publication guide](docs/guide/releasing.md) and [current status](docs/guide/status.md)
for subsequent repository, CI and documentation work.

The offline implementation milestone is complete. This is not a registry release
or a claim that the provider lifecycle has been verified against live HexCore.
No HexCore API requests, billable operations, remote creation, publication or
release signing were performed during implementation. No sibling credentials,
state, plans or full live responses were read or copied.

## Delivered

- Protocol-6 Plugin Framework server, version injection and debugger flag.
- Sensitive environment/config token handling, endpoint override, immutable SDK
  client injection, versioned user agent and diagnostics that discard remote
  messages and request details.
- Typed, stable region/size/image/application catalogs and a singular VDS data
  source. Unavailable regions remain visible. Monetary and date text is preserved.
- Standard/custom VDS creation with apply-time catalog validation, GB-to-MB
  checks, partial ID retention, readiness polling and no mutation replay.
- Refresh with shared present/absent/indeterminate reconciliation. Only a
  successful complete account list omitting the exact ID proves absence.
- Graceful or explicitly forced shutdown, action completion, an observed halted
  precondition, single destroy submission and confirmed asynchronous removal.
- Replacement planning, local-only policy/timeout updates, strict structured
  import, declared unreadable provisioning metadata and optional non-secret
  local settings. Plain-ID import is deliberately rejected.
- Write-only creation passwords with replacement generations, examples,
  generated schema documentation and repeatable offline test coverage.

## Dependency and toolchain evidence

The provider declares Go **1.25.8** minimum and pins Framework **v1.19.0**,
Plugin Testing **v1.16.0**, validators **v0.19.0**, timeout helpers **v0.7.0**,
and documentation tooling **v0.25.0**. Builds consume the public
`github.com/2chevskii/go-hexcore v1.0.0` module with committed checksums and no
local `replace`/`go.work` requirement. The inspected sibling SDK protocol/model
files, README and verification report matched that downloaded release.

Validation used stable Go **1.27.2** and Terraform **1.14.7** on Windows amd64.
Terraform and the temporary WSL Go toolchain archives were checked against their
published SHA256 checksums. Race validation used WSL Ubuntu 26.04, GCC, CGO and
the same Go/Terraform versions on Linux amd64. Minimum Terraform 1.11 is declared
for write-only arguments; that exact minimum version and OpenTofu were not tested.

## Passing checks

```text
gofmt (no remaining formatting changes)
go mod tidy / go mod verify
go build -o terraform-provider-hexcore.exe .
go vet ./...
TF_ACC_TERRAFORM_PATH=<Terraform 1.14.7> go test ./... -count=1 -timeout 10m -coverprofile coverage.out
CGO_ENABLED=1 TF_ACC_TERRAFORM_PATH=<Linux Terraform 1.14.7> go test -race ./... -count=1 -timeout 10m
terraform fmt -check -recursive examples
go generate ./... (tfplugindocs; regeneration unchanged)
git diff --check
```

The Windows suite passed **99 tests/subtests** with **88.0%** total statement coverage
(the separately compiled CLI child is not instrumented). The full Linux race suite passed,
including CLI acceptance. The ordinary suite is deterministic and uses loopback
HTTP servers with real SDK decoding. A test transport guard rejects non-loopback
HTTP. Terraform CLI tests explicitly skip if no executable is installed; the
validation above supplied one, so none of those tests were skipped.

Five protocol-6 acceptance sequences cover standard/custom lifecycle, both CLI
and import-block imports, no-op plans, refresh, policy updates, provisioning-input
replacement, catalogs, and password generations. Password edits alone are
no-ops; synthetic secrets are checked against state, JSON plans, diagnostics and
captured trace logs. Contract tests also cover missing/empty/unknown configuration,
ambiguous 403/404 reads, list failures, partial state after readiness failure,
uncertain mutation responses, cancellation, action failure and shutdown/deletion
timeouts. Cleanup assertions preserve an unrelated synthetic VDS.

The compiled-provider CLI test creates a temporary `dev_overrides` configuration,
discovers the intended `registry.terraform.io/2chevskii/hexcore` schema, validates
all data-source examples composed with provider configuration, and applies both
documented VDS examples against a local stub with synthetic catalog slugs. Each
example has a no-op post-apply plan and a no-op plan after actual `state rm` plus
structured `terraform import`, followed by confirmed destroy. Local timeouts and
password generations are preserved through the optional non-secret import fields.
No machine-specific CLI configuration is committed.

## Remaining live limits and next action

The SDK's [2026-10-19 report](https://github.com/2chevskii/go-hexcore/blob/v1.0.0/docs/live-verification.md)
is prior SDK evidence, not provider live acceptance. Creation without an explicit
password has undocumented credential delivery; this remains unverified. Image
and original standard size slugs cannot be independently read through the SDK.
Custom plan bounds beyond quantities/image disk minimum are not published.

Disk attachment/resize and graceful reboot/password-reset completion remain
upstream blockers; disk deletion is undocumented. These operations and other
deferred features are not exposed. No additional disks were created. Previously
reported live leftovers were not inspected or changed; this implementation has
no live resources to clean up.

The next concrete action is a separately authorized live standard/custom VDS
provider acceptance session with a specified budget, catalog targets and a unique
test ownership prefix. Any future live harness must require `HEXCORE_LIVE=1`,
`HEXCORE_TOKEN`, an additional `HEXCORE_LIVE_WRITES=1` for mutations, and explicit
session spending authorization; `TF_ACC` alone never enables it. Never use
preexisting resources as disposable fixtures. Resolve credential-delivery behavior
with vendor documentation or a separately authorized check. Publication and signing
remain separate maintainer decisions.

# Verification and limitations

## Current status

The source repository is public and has CI, a VitePress v2 documentation site,
and provider-specific release packaging. Terraform Registry enrollment and the
first signed provider version remain pending. A live standard VDS lifecycle with
OpenTofu 1.12.6 passed on 24 October 2026; custom VDS live acceptance remains pending.

## Offline evidence

The original implementation milestone passed 99 tests/subtests and 88.0% total
statement coverage using Go 1.27.2 and Terraform 1.14.7. Windows build/vet checks,
the full Linux CGO race suite and deterministic schema regeneration passed.
The separately compiled CLI child is not included in that coverage percentage.

Coverage includes provider configuration, all catalogs, standard/custom VDS,
replacement, timeout and deletion policy, structured import, state retention,
uncertain writes, cancellation and secret-free state/plans/diagnostics/logs.
The compiled provider was discovered with a temporary development override;
both documented VDS examples had no-op plans after apply and actual import,
followed by confirmed local-stub destroy.

[CI](https://github.com/2chevskii/terraform-provider-hexcore/actions/workflows/ci.yml)
now repeats provider validation on the declared minimum and stable toolchains,
builds the site, checks generated reference pages and tests release packaging.
These are offline tests. CI has no HexCore credentials and does not create
billable infrastructure.

The same suite covers OpenTofu 1.12.6, including compiled-provider discovery,
standard/custom lifecycle, write-only passwords, import and no-op plans. Provider
configurations use the full `registry.terraform.io/2chevskii/hexcore` address so
both CLIs select the same provider. This remains local-stub evidence, not a live
HexCore acceptance result.

Regression coverage for omitted regions and the observed halted-status variants
brings the suite to 106 tests/subtests. All passed with CGO race checking on
Go 1.27.3/OpenTofu 1.12.6, Go 1.27.3/Terraform 1.16.6 and Go 1.26.10/Terraform 1.11.4.

## Live standard VDS verification: 24 October 2026

The authorized session used a local provider build and OpenTofu 1.12.6, with a
separate local state and a unique test name. The requested selectors were
`ru-msk-2`, `debian_13` and `1vcpu-1gb-10disk` (1 vCPU, 1024 MB RAM, 10 GB disk).
An explicit creation password was supplied through an ephemeral variable.

The completed sequence was creation to `Running` with an assigned IPv4, refresh,
an empty plan, structured import, another empty plan, graceful shutdown and
deletion. A successful complete API listing confirmed absence; local state was
empty and the preexisting VDS identity and configuration metadata were unchanged.

The first attempt exposed two differences from the original fixtures: both Get
and List omitted region, and shutdown reported `Halted` rather than `halted`.
The provider now preserves the configured region when it cannot observe one,
rejects reported region mismatches, and recognizes both observed halted spellings.
The interrupted first VDS and the successful retry VDS were both deleted; they
were created sequentially, and no preexisting VDS was used as a fixture.

This verifies the selected standard-size path, not custom sizing, SSH/guest OS
acceptance or credential delivery when a password is omitted. The selected
region cannot be independently verified when both API read routes omit it.

## Live boundary

The [SDK's dated verification report](https://github.com/2chevskii/go-hexcore/blob/v1.0.0/docs/live-verification.md)
documents prior SDK observations. It does not establish the provider's live
compatibility or current vendor health beyond the separately recorded provider
session above. The original implementation and publication-preparation checks
were offline.

- Original image and standard size slugs cannot be independently retrieved.
- Region can be omitted by both VDS read routes; the configured selector is retained.
- Creation without a password has undocumented credential delivery.
- Custom plan bounds beyond positive quantities and image disk minimum are unpublished.
- A singular 403/404 cannot establish resource absence.

## Deferred API surface

Disk deletion is undocumented. Disk attachment/resize and graceful reboot/
password-reset completion have unresolved upstream behavior. The provider exposes
no disk lifecycle, reboot/reset, power-state management, monitoring, SSH keys,
user-data, networking/firewalls or provider action resources.

Further live work needs explicit session authorization, a budget and catalog
targets. A future live harness must require `HEXCORE_LIVE=1` and `HEXCORE_TOKEN`,
plus `HEXCORE_LIVE_WRITES=1` for mutations. Never use preexisting resources as
disposable fixtures. `TF_ACC` alone is insufficient.

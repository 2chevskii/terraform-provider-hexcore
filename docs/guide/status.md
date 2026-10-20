# Verification and limitations

## Current status

The source repository is public and has CI, a VitePress v2 documentation site,
and provider-specific release packaging. Terraform Registry enrollment, the first
signed provider version and live provider acceptance remain pending.

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

## Live boundary

The [SDK's dated verification report](https://github.com/2chevskii/go-hexcore/blob/v1.0.0/docs/live-verification.md)
documents prior SDK observations. It does not establish the provider's live
compatibility or current vendor health. No live HexCore calls were made while
implementing the provider or preparing this repository for publication.

- Original image and standard size slugs cannot be independently retrieved.
- Creation without a password has undocumented credential delivery.
- Custom plan bounds beyond positive quantities and image disk minimum are unpublished.
- A singular 403/404 cannot establish resource absence.
- OpenTofu compatibility is untested.

## Deferred API surface

Disk deletion is undocumented. Disk attachment/resize and graceful reboot/
password-reset completion have unresolved upstream behavior. The provider exposes
no disk lifecycle, reboot/reset, power-state management, monitoring, SSH keys,
user-data, networking/firewalls or provider action resources.

Further live work needs explicit session authorization, a budget and catalog
targets. A future live harness must require `HEXCORE_LIVE=1` and `HEXCORE_TOKEN`,
plus `HEXCORE_LIVE_WRITES=1` for mutations. Never use preexisting resources as
disposable fixtures. `TF_ACC` alone is insufficient.

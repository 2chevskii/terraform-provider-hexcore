# Release process

The public GitHub repository and documentation site are available before the
first provider version. A GitHub source publication does not register the
provider with the Terraform Registry. No provider version has been released yet.

## Before the first version

Complete a separately authorized live provider acceptance session for the
advertised lifecycle, including standard/custom creation, refresh, import, no-op
plans and confirmed cleanup. Resolve or clearly scope credential delivery when
no creation password is supplied. Record remaining limits without inferring
provider readiness from the SDK's earlier live checks.

The maintainer must choose the version and configure a dedicated release signing
key. The release key has no passphrase. Store its ASCII-armored private key as
environment secret `GPG_PRIVATE_KEY` in the GitHub Actions environment
`terraform-registry`; never put signing material in the repository. The release
job references that environment, which permits tags matching `v*` and the exact
`develop` branch for manual signing checks. Replace the initial `PLACEHOLDER_...`
value before running it; the workflow rejects empty or placeholder keys.
Register the public signing key with the Terraform
Registry when enrolling the provider.

## Package validation

`.goreleaser.yml` builds Linux, macOS and Windows archives for amd64 and arm64.
It injects the version, disables CGO and emits registry-compatible filenames:

```text
terraform-provider-hexcore_<version>_<os>_<arch>.zip
terraform-provider-hexcore_<version>_manifest.json
terraform-provider-hexcore_<version>_SHA256SUMS
terraform-provider-hexcore_<version>_SHA256SUMS.sig
```

The manifest declares protocol 6. The checksum file covers the archives and
manifest. Release builds sign the checksum file with GPG. CI validates the same
packaging configuration without signing or publishing:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=sign,publish --parallelism 2
```

Before tagging, manually run **Signed provider release** on `develop` after its
push CI passes. This imports the environment key and builds a signed snapshot
without publishing. It verifies the checksum signature in a fresh GPG keyring,
checks all six archives and the manifest, and uploads only those public artifacts
and the public key for seven days. The workflow summary records the signing
fingerprint; compare it with the key registered in Terraform Registry.

## Promote and publish

1. Prepare release notes and open a release PR to `master`. Merge with a merge
   commit so the history from `develop` remains connected.
2. Wait for the exact `master` commit's full `CI` workflow to pass.
3. When the maintainer authorizes the version, create its `vX.Y.Z` tag on that
   exact tested commit. Release tags cannot be updated or deleted.
4. The signed-release workflow requires the tag to match the current `master`
   commit and verifies its successful push CI before using the signing key.
5. Review the generated **draft** GitHub release, its six archives, protocol-6
   manifest, checksums and signature. Publish the draft when ready.
6. Enroll the public `terraform-provider-hexcore` repository and signing key in the
   Terraform Registry, verify installation of the released version, then sync
   `master` back to `develop` through a PR if needed.

Nothing in the current setup automatically creates the first version or spends
HexCore funds. Follow the official
[provider publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing)
when completing enrollment.

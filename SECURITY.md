# Security policy

This provider has not had a signed Terraform Registry release. Security fixes
are made on `develop` until releases exist; pin and rebuild a reviewed commit
when using a development build.

Report vulnerabilities privately with
[GitHub private vulnerability reporting](https://github.com/2chevskii/terraform-provider-hexcore/security/advisories/new).
Include the affected commit/version, operation, impact and a minimal sanitized
reproduction. Never include API tokens, passwords, signing keys, Terraform state
or plans, complete live responses or account billing details in a public issue.

The provider deliberately avoids logging requests or forwarding raw API messages,
keeps creation passwords write-only, and retains state on ambiguous absence.
Please report any path that violates those guarantees.

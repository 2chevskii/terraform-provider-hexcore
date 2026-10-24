# Getting started

Manage HexCore VDS with explicit provisioning metadata, bounded operation waits,
and Terraform state that survives uncertain API outcomes.

The public source repository is available on
[GitHub](https://github.com/2chevskii/terraform-provider-hexcore). Install signed
releases from `registry.terraform.io/2chevskii/hexcore`.
The provider requires **Terraform 1.11 or newer**.
OpenTofu 1.12.6 is also covered by offline acceptance tests.

## Install from Terraform Registry

Save this configuration in `main.tf`:

```hcl
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hexcore = {
      source  = "registry.terraform.io/2chevskii/hexcore"
      version = "1.0.0"
    }
  }
}

provider "hexcore" {}
```

Use the full source address in both Terraform and OpenTofu. A short address
defaults to a different registry in each CLI. For OpenTofu, use `tofu` in place
of `terraform` in the CLI commands; local development overrides work in both.

```sh
terraform init
terraform validate
```

Initialization downloads the provider and verifies its signature. Remove any
local `dev_overrides` entry for this provider when checking Registry installation.

## Configure authentication

Supply the raw API token through `HEXCORE_TOKEN`, without a `Bearer` prefix.
The provider does not load `.env` files or make an account request on startup.
Installation and schema validation do not require a HexCore token.

## Discover provisioning options

Use the catalog data sources to choose current, available selectors:

```hcl
data "hexcore_regions" "catalog" {}
data "hexcore_sizes" "catalog" {}
data "hexcore_images" "catalog" {}

output "regions" {
  value = data.hexcore_regions.catalog.regions
}
```

Catalog reads do not create infrastructure. An unavailable region stays visible
with `available = false`. Prices are decimal strings and IDs are strings.

## Plan a VDS

Replace the example slugs with values from the current catalogs. Applying a VDS
configuration creates a billable server in your account.

```hcl
resource "hexcore_vds" "web" {
  name   = "web-example"
  region = "example-region"
  image  = "debian-12"
  size   = "example-size"
}

output "ipv4" {
  value = hexcore_vds.web.ipv4
}
```

Review [VDS lifecycle](./vds-lifecycle), [import and recovery](./import), and the
[verification boundary](./status) before using the provider for live workloads.

## Local development builds

Install Go 1.26.10 or newer, then:

```sh
git clone https://github.com/2chevskii/terraform-provider-hexcore.git
cd terraform-provider-hexcore
go build -o terraform-provider-hexcore .
```

On Windows, name the output `terraform-provider-hexcore.exe`.

Create a temporary Terraform CLI configuration outside the repository. Use the
absolute directory containing the binary; Windows paths can use forward slashes.

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/2chevskii/hexcore" = "/absolute/path/to/terraform-provider-hexcore"
  }
  direct {}
}
```

Set `TF_CLI_CONFIG_FILE` to that file. Development overrides bypass installation,
so skip `terraform init` for this local build. Commands such as `terraform
validate`, `plan`, and `apply` discover the local executable.

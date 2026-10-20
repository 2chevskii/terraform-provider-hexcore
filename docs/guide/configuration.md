# Configuration and catalogs

## Token precedence

`token` is optional and sensitive. A null or omitted value uses `HEXCORE_TOKEN`.
An explicitly empty, malformed, or unknown configured value never falls back to
the environment. Keep the raw token outside committed configuration.

An optional `endpoint` overrides the SDK base URL for development or testing.
Its default is `https://api.hexcore.ru/`. Use an absolute HTTP(S) URL without
credentials, a query, or a fragment. Client construction and schema validation
make no network requests.

```hcl
provider "hexcore" {
  endpoint = "http://127.0.0.1:8080/"
}
```

The client is shared immutably between resources and data sources. HTTP requests
have the SDK's 30-second limit and inherit the remaining operation deadline.

## Available catalogs

| Data source | Collection | Useful fields |
| --- | --- | --- |
| [`hexcore_regions`](../data-sources/regions) | `regions` | `slug`, `name`, `available` |
| [`hexcore_sizes`](../data-sources/sizes) | `sizes` | `slug`, `vcpu`, `memory_mb`, `disk_gb`, prices |
| [`hexcore_images`](../data-sources/images) | `images` | `id`, `slug`, version, `min_disk_gb` |
| [`hexcore_applications`](../data-sources/applications) | `applications` | image with preinstalled software |

Collections have stable ordering by slug, then string ID where present. Empty
catalogs are known empty collections. Unavailable regions are never filtered out.
Apply validates the selected region, standard size, and image disk minimum
against the current catalogs without creating infrastructure first.

## Units and nullable observations

| Value | Representation |
| --- | --- |
| Resource and image IDs | Strings, including IDs larger than machine integers |
| Observed memory | `memory_mb` |
| Custom requested memory | `custom_size.memory_gb`, compared at 1024 MB/GB |
| Disk capacity | GB, named `disk_gb` |
| Money | Unmodified decimal text |
| Dates and timestamps | Raw API text, with no inferred timezone |
| Unavailable optional fields | Null, without an invented empty string or zero |

The [`hexcore_vds` data source](../data-sources/vds) reads a single string ID.
Missing and indeterminate objects return diagnostics. Its read never removes
managed-resource state.

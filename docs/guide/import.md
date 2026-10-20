# Import and recovery

## Structured identifiers

Plain-ID import is rejected because the SDK cannot read the original image and
standard size slugs. Supply the real ID and explicitly declare the original
provisioning metadata. These declarations are not independently observed image metadata.

```sh
terraform import hexcore_vds.web '{"id":"123456","image":"debian-12","region":"example-region","size":"example-size"}'
```

For custom sizing:

```sh
terraform import hexcore_vds.web '{"id":"123456","image":"debian-12","region":"example-region","custom_size":{"vcpu":1,"memory_gb":1,"disk_gb":10}}'
```

These examples use POSIX shell quoting. In other shells, pass the same JSON as
one argument. Import checks the actual VDS, region where available, and CPU,
memory and disk shape. Matching shape cannot prove which original standard size
or image was used. Import never creates, shuts down or deletes a server.

## Preserve local settings

Omitted local settings use `delete_mode = "graceful"`, no explicit timeouts and
no password generation. To match existing configuration without a follow-up diff,
declare the optional non-secret local fields:

```json
{
  "id": "123456",
  "image": "debian-12",
  "region": "example-region",
  "size": "example-size",
  "delete_mode": "graceful",
  "timeouts": { "create": "15m", "read": "2m", "delete": "20m" },
  "password_wo_version": 1
}
```

The generation is a non-secret number and does not change guest credentials on
import. Never put a token or password in the identifier. Unknown fields, malformed
IDs, missing metadata, nonpositive quantities and invalid durations are rejected.
Run `terraform plan` after importing and resolve any differences before applying.

## Recover an uncertain creation

If creation returned an ID but readiness failed, that ID remains in state with
the last available observation. Inspect it before another apply: Terraform may
propose replacing a tainted resource.

If a transport or decoding failure prevented obtaining an ID, the POST may still
have been accepted. Inspect the account list or control panel, identify the
actual server and its original selectors, then use structured import. Names are
not unique idempotency keys. The provider never adopts or deletes a same-name
object automatically and never resubmits the uncertain creation.

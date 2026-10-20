# VDS lifecycle

## Standard or custom sizing

Configure exactly one of `size` or `custom_size`. Custom quantities must be
positive, and memory must fit in an `int64` after multiplication by 1024.

```hcl
resource "hexcore_vds" "custom" {
  name   = "custom-example"
  region = "example-region"
  image  = "debian-12"
  custom_size = {
    vcpu      = 1
    memory_gb = 1
    disk_gb   = 10
  }
  timeouts = {
    create = "15m"
    read   = "2m"
    delete = "20m"
  }
}
```

These timeouts are project defaults, not vendor SLAs. Read polling is approximately
every five seconds and observes cancellation. Independent VDS can progress in parallel.

## Creation and replacement

Creation is submitted once. The accepted resource ID enters partial state before
readiness polling, so a wait failure can still leave a tracked, billed server.
Completion requires the verified `Running` status, matching name, region and
shape, and a usable IPv4 address. Never assume an accepted response means readiness.

Changing `name`, `region`, `image`, `size`, `custom_size`, or the password generation
replaces the entire VDS. The API has no general rename or resize operation.
Names are limited to 45 UTF-8 bytes by conservative provider policy: the observed
API truncated long names at 45 characters, and its Unicode boundary is unverified.

The SDK cannot return the original image or standard size slug. Refresh preserves
those configured declarations and updates observable drift. Changing observations
such as IP and status are not frozen with `UseStateForUnknown`.

## Creation passwords

```hcl
variable "creation_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "hexcore_vds" "protected" {
  name   = "password-example"
  region = "example-region"
  image  = "debian-12"
  size   = "example-size"

  password_wo         = var.creation_password
  password_wo_version = 1
}
```

`password_wo` is read from configuration only during creation. Its value is never
stored in provider state or planned resource values. Use an ephemeral variable
to keep the caller's variable out of saved plans too. A password edit alone
produces no diff; increment `password_wo_version` to replace the server.
No password-reset request is made. Default credential delivery when the password
is omitted is undocumented and unverified.

## Refresh and absence

The provider reconciles singular Get with one complete account VDS list. It fills
fields omitted by Get from the matching list entry. A successful full list that
omits the exact ID proves absence. A singular 403 or 404 does not. If listing
fails, presence is indeterminate and state is retained with a diagnostic.

## Deletion policy

`delete_mode = "graceful"` is the default. `force` explicitly chooses a power cut.
Changing deletion policy or timeouts updates local state without an API mutation;
apply those changes before destroy.

Deletion submits shutdown once, waits for the action and an observed `halted`
state, submits destroy once, then waits for confirmed absence from a successful
full listing. Graceful shutdown never escalates to force. Errors and timeouts
retain state; uncertain writes are not replayed within the operation.

See the [complete resource schema](../resources/vds) and [recovery guide](./import).

variable "creation_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "hexcore_vds" "example" {
  name   = "example-custom-vds"
  region = "example-region"
  image  = "debian-12"
  custom_size = {
    vcpu      = 1
    memory_gb = 1
    disk_gb   = 10
  }

  password_wo         = var.creation_password
  password_wo_version = 1 # Incrementing replaces the whole VDS.
}

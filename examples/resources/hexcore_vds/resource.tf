# Select current available slugs from the catalog data sources before applying.
resource "hexcore_vds" "example" {
  name   = "example-vds"
  region = "example-region"
  image  = "debian-12"
  size   = "example-size"

  delete_mode = "graceful"
  timeouts = {
    create = "15m"
    read   = "2m"
    delete = "20m"
  }
}

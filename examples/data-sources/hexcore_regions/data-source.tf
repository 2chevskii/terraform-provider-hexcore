data "hexcore_regions" "available" {}

output "regions" {
  value = data.hexcore_regions.available.regions
}

data "hexcore_sizes" "available" {}

output "sizes" {
  value = data.hexcore_sizes.available.sizes
}

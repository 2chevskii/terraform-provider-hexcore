data "hexcore_vds" "example" {
  id = "123456"
}

output "ipv4" {
  value = data.hexcore_vds.example.ipv4
}

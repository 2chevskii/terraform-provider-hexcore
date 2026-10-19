data "hexcore_applications" "available" {}

output "applications" {
  value = data.hexcore_applications.available.applications
}

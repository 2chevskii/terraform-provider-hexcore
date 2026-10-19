data "hexcore_images" "available" {}

output "images" {
  value = data.hexcore_images.available.images
}

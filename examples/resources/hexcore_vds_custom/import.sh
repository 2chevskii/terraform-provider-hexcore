# The non-secret generation matches resource.tf; no password goes in this ID.
terraform import hexcore_vds.example '{"id":"123456","image":"debian-12","region":"example-region","custom_size":{"vcpu":1,"memory_gb":1,"disk_gb":10},"password_wo_version":1}'

# POSIX shell; use the original provisioning selectors, not inferred values.
terraform import hexcore_vds.example '{"id":"123456","image":"debian-12","region":"example-region","size":"example-size","timeouts":{"create":"15m","read":"2m","delete":"20m"}}'

terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hexcore = {
      source = "2chevskii/hexcore"
    }
  }
}

# Supply HEXCORE_TOKEN through your environment (raw token, no Bearer prefix).
provider "hexcore" {}

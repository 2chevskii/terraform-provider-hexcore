---
layout: home
title: HexCore Terraform Provider
hero:
  name: HexCore
  text: Infrastructure through Terraform.
  tagline: Discover catalogs. Provision standard or custom VDS. Track uncertain operations with explicit recovery paths.
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: Schema reference
      link: /reference/provider
    - theme: alt
      text: View source
      link: https://github.com/2chevskii/terraform-provider-hexcore
features:
  - title: Discover before you provision
    details: Typed region, size, image and application catalogs keep unavailable options visible and prices precise.
  - title: Bounded lifecycle operations
    details: Create and delete wait for observed completion. Accepted IDs stay tracked, and ambiguous reads retain state.
  - title: Explicit import and recovery
    details: Declare unreadable provisioning selectors, preserve local settings and check the next plan before changing infrastructure.
  - title: Creation secrets stay ephemeral
    details: Terraform write-only passwords use non-secret generations for replacement and stay out of provider state.
---

::: info Release status
The source is public; the provider has not been released to the Terraform Registry.
Offline acceptance and a live standard VDS lifecycle are verified.
Custom VDS live acceptance remains pending.
Read the [verification boundary](./guide/status) before planning live use.
:::

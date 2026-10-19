# Agent instructions

Read [.agents/SPEC.md](.agents/SPEC.md) before implementing the provider. It
contains the implementation scope, SDK contract, Terraform lifecycle rules,
verified API limitations, testing requirements, and handoff acceptance criteria.

Keep HexCore HTTP protocol handling in `github.com/2chevskii/go-hexcore`.
Distinguish offline test coverage from live verification. Preserve unrelated
changes and never load or copy credentials from the sibling SDK repository.

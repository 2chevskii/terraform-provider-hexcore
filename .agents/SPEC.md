# HexCore Terraform provider: implementation and agent handoff

Prepared on 2026-10-19. This repository currently contains guidance and Git
bootstrap files only. Implement the provider here; do not mistake this document
for a claim that resources, tests, or registry publication already exist.

## 1. Objective and boundaries

Build a Terraform provider for HexCore using the existing Go API client. The
first useful milestone is catalog discovery and reliable management of a VDS,
with standard or custom sizing. Favor correct Terraform state and complete
create/read/delete behavior over exposing every HTTP operation.

Project identities:

| Item | Value |
| --- | --- |
| Go module | `github.com/2chevskii/terraform-provider-hexcore` |
| Provider type | `hexcore` |
| Intended Terraform source address | `registry.terraform.io/2chevskii/hexcore` |
| Executable | `terraform-provider-hexcore` |
| SDK dependency | `github.com/2chevskii/go-hexcore v1.0.0` |
| Local SDK checkout | `../go-hexcore` |

The source address is a design target, not an existing registry registration.
This task authorizes local implementation and offline validation. Creating a
GitHub remote, publishing the provider, signing releases, or spending HexCore
funds needs a subsequent instruction. Continue independent offline work when a
live API limitation blocks one feature; record the blocker precisely.

Use focused Conventional Commits. Preserve unrelated changes. Once a remote is
configured, follow the SDK project's branch model: normal PR branches target
`develop`, release/hotfix promotion targets `master`, and work is never pushed
directly to those protected branches. This repository's initial local-only
documentation commit is bootstrap. Do not copy the SDK's release workflows
unchanged: a provider eventually needs binary distribution and registry release
metadata, whereas the SDK distributes tagged Go source.

## 2. Read these sources first

Inspect the following in the sibling SDK, and compare the checkout with the
published dependency before relying on newer code:

- `README.md` and `docs/live-verification.md`: observed behavior and blockers.
- `client.go`, `errors.go`, `vds.go`, `actions.go`, `models.go`, `catalogs.go`,
  and `disks.go`: exported contracts and units.
- `integration_test.go`, `provisioning_integration_test.go`, and sanitized
  fixtures: useful evidence, not provider code to copy wholesale.

Public references:

- [SDK v1.0.0 release](https://github.com/2chevskii/go-hexcore/releases/tag/v1.0.0)
- [SDK source at v1.0.0](https://github.com/2chevskii/go-hexcore/tree/v1.0.0)
- [SDK live verification](https://github.com/2chevskii/go-hexcore/blob/v1.0.0/docs/live-verification.md)
- [HexCore API documentation](https://dev.hexcore.ru/)
- [HexCore Swagger document](https://dev.hexcore.ru/hexcore.json?v=0.8.7)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Official framework scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding-framework)
- [Provider servers and protocol versions](https://developer.hashicorp.com/terraform/plugin/framework/provider-servers)
- [Framework create lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/create)
- [Framework read lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/read)
- [Import](https://developer.hashicorp.com/terraform/plugin/framework/resources/import)
- [Timeouts](https://developer.hashicorp.com/terraform/plugin/framework/resources/timeouts)
- [Write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)
- [Acceptance testing](https://developer.hashicorp.com/terraform/plugin/testing/acceptance-tests)

The SDK v1.0.0 release was confirmed published on 2026-10-19. The Swagger URL's
revision and its internal API version differ; live evidence in the SDK is more
specific than an inconsistent example. Recheck upstream documentation when an
unresolved contract matters; do not invent undocumented endpoints or parameters.

## 3. Implementation stack and layout

Use Go, Terraform Plugin Framework, protocol 6, `terraform-plugin-testing`, and
`terraform-plugin-docs`. Use framework validators and timeout helpers where they
reduce custom logic. Use current compatible, explicitly pinned versions and
commit `go.sum`; do not start a new implementation on SDKv2. An SDKv2 transitive
dependency brought by testing is not a reason to rewrite the provider.

The Go SDK's minimum is Go 1.22, but this does not constrain the provider's
toolchain. As checked on 2026-10-19, official scaffolding declares Go 1.25.8,
Plugin Framework v1.19.0, and Plugin Testing v1.16.0. Verify the pinned modules'
requirements at implementation time and choose a supported stable Go toolchain.

Set an initial Terraform CLI minimum of 1.11 to support the optional write-only
creation password described below. Protocol 6 alone has an earlier minimum;
the password feature is the reason for this project's higher baseline. Do not
claim OpenTofu compatibility until the selected schema and acceptance cases
have been tested with a compatible OpenTofu version.

Suggested layout, adapted as needed without adding a generic abstraction layer:

```text
main.go
go.mod / go.sum
internal/provider/provider.go
internal/provider/provider_test.go
internal/provider/vds_resource.go
internal/provider/vds_data_source.go
internal/provider/*_data_source.go
internal/provider/*_test.go
internal/provider/testdata/
internal/provider/vds_wait.go
internal/provider/diagnostics.go
examples/provider/
examples/resources/hexcore_vds/
examples/data-sources/hexcore_*/
docs/index.md
docs/resources/vds.md
docs/data-sources/*.md
```

Use `providerserver.Serve`, the intended registry address, a debug flag, and a
version string supplied at build time. Test via protocol-6 provider factories.
Use framework `types` in Terraform models and separate SDK models at the
boundary. Handle null and unknown values explicitly. Keep all HTTP encoding,
authentication, envelopes, endpoint paths, and response normalization in the SDK.

Pin the published SDK normally. A temporary `go.work` or local `replace` can be
used while developing an SDK correction, but must not be required by committed
provider builds or the handoff. Make an upstream SDK fix and consume a tagged
version rather than adding private HexCore HTTP calls to this provider.

## 4. Provider configuration

Start with:

| Attribute | Contract |
| --- | --- |
| `token` | Optional, sensitive; null uses `HEXCORE_TOKEN`; raw token without a Bearer prefix |
| `endpoint` | Optional; null uses `hexcore.DefaultBaseURL`; explicit override for testing/development |

An explicitly empty or malformed token is an error. An unknown configured
value must not silently fall back to an environment value. Validate what is
known without making network requests during schema/configuration validation.
During Configure, report unresolved required values with useful diagnostics.
Do not auto-load `.env` files, read sibling credentials, or call the account API
just to construct the provider.

Construct one immutable shared SDK client with `hexcore.NewClient` and pass it
through `ResourceData` and `DataSourceData`. Handle nil provider data during
validation and diagnose unexpected configured types. Set a user agent including
the provider version and Terraform version when available. The SDK's default
HTTP request timeout is 30 seconds; resource operation deadlines are separate.
Pass Terraform's context through every call. Do not mutate the shared client.

Tokens, passwords, Authorization headers, request bodies, complete Terraform
models, account billing details, and raw responses must not appear in logs or
diagnostics. SDK API error messages can also contain user data; redact known
secret values and return only the necessary operation/status/message details.

## 5. MVP surface

Implement in this order:

1. Provider server, schema, configuration, and client injection.
2. `hexcore_regions`, `hexcore_sizes`, `hexcore_images`, and
   `hexcore_applications` data sources using their matching SDK `List` methods.
3. Singular `hexcore_vds` data source taking a string ID.
4. Managed `hexcore_vds` resource with standard/custom creation, reliable
   refresh, bounded waits, deletion, replacement, and tested import behavior.
5. Examples, generated documentation, and meaningful offline acceptance tests.

Catalog data sources should expose typed collections with stable ordering by
slug/ID. Keep unavailable regions visible with their `available` flag. No
artificial state ID is needed for a catalog data source unless a tested framework
requirement calls for one. A missing singular data-source object is a diagnostic,
not a managed-resource `RemoveResource` call.

Add read-only `hexcore_disk` or a minimal account data source only after the MVP
works and there is a concrete use for them. Avoid exporting corporate billing
details by default. Do not expose `hexcore_disk` as a managed resource: the public
API has no documented disk deletion operation. Do not make destroy silently
forget a billed disk or invent a delete URL.

Defer disk attachment/resize resources, reboot/reset-password commands,
power-state management, monitoring, SSH keys, user-data, networking/firewalls,
and provider-specific actions until their API contracts are available and
verified. Reboots and other commands are not persistent infrastructure objects.

## 6. SDK contracts that must survive the Terraform mapping

| SDK behavior | Provider requirement |
| --- | --- |
| Resource IDs are strings | String IDs everywhere; validate IDs without converting through machine integers |
| Quantities are `int64` | Terraform integer attributes, with positive/range checks |
| VDS/size memory is reported in MB | Expose `memory_mb` for observations |
| `CustomSize.Memory` is requested in GB | Expose `custom_size.memory_gb`; 1 GB was observed as 1024 MB |
| Disk capacities are GB | Use explicit `disk_gb` names |
| Prices and balances are strings | Preserve decimal text; do not convert money to floating point |
| Timestamps are raw strings | Do not invent a timezone or parse them as UTC without evidence |
| Optional SDK fields are pointers | Preserve known-null meaning; do not substitute empty strings or zero |
| VDS.Get omits some List fields | Reconcile with one successful full list when needed, matched by exact ID |
| Creation returns a VDS ID, no action ID | Poll resource readiness; do not guess the creation action from history |
| Mutations can return accepted acknowledgements | Acceptance is not completion |
| SDK makes no implicit retries | Provider owns explicit read polling; never blindly replay a mutation |
| Actions.Wait uses context and WaitOptions | Choose bounded waits; default SDK interval is two seconds |
| Unknown action status is an error | Never treat it as completed |

Use `errors.As` for `*hexcore.APIError` and `*hexcore.ActionError`, and preserve
context cancellation with `errors.Is`. Do not parse `error.Error()` text to
classify an API status. `APIError` has StatusCode, Message, Method, Path, and
Headers; a Retry-After value does not make a non-idempotent operation retryable.

## 7. Managed VDS schema and planning

Use one resource with mutually exclusive standard and custom sizing:

| Attribute | Proposed behavior |
| --- | --- |
| `id` | Computed string; stable once created |
| `name` | Required; replacement on change; reject names exceeding the conservative 45-byte limit |
| `region` | Required slug; replacement on change |
| `image` | Required image slug; replacement on change |
| `size` | Optional standard size slug; exactly one of size/custom_size |
| `custom_size` | Optional single nested object; required positive vcpu, memory_gb, disk_gb inside it |
| `password_wo` | Optional sensitive write-only creation password, obtained from configuration |
| `password_wo_version` | Non-secret generation number; replacement on change; document its use with password_wo |
| `ipv4`, `status` | Computed string, nullable if genuinely unavailable |
| `vcpu`, `memory_mb`, `disk_gb` | Computed observed integer quantities |
| `price`, `created_at`, `next_bill_date` | Computed textual values when supported by SDK responses |
| `delete_mode` | Optional local deletion policy: graceful by default, force only when explicitly selected |
| `timeouts` | Explicit create/read/delete budgets through framework timeout support |

The 45-byte name rule is a conservative provider policy: live HexCore silently
truncated long names to 45 characters, and Unicode boundary behavior has not
been verified. Document this choice instead of claiming a verified byte-based
API rule. Do not truncate a configured name while returning different state.

All API creation inputs require replacement because the SDK/API offers no
general rename, region/image change, or VDS resize operation. Changes to local
timeout/deletion policy may update state without an API mutation; document that
distinction in Update. Use replacement plan modifiers and focused cross-field
validators. Allow unknown inputs during planning and validate once known.

For custom sizing, check overflow when comparing memory_gb * 1024 to observed
memory_mb. Do not make up plan bounds from one test account; validate current
catalog constraints during apply when needed, without creating infrastructure.

Write-only password values never belong in state or plan artifacts. A changed
write-only value alone cannot produce a diff; its generation number requests
replacement. Do not put an unconditional RequiresReplace modifier directly on
the write-only attribute. Password changes must not call ResetPassword: that
operation's completion is unverified. Test that secrets are absent from state,
plans, logs, and diagnostics. Explain where access credentials come from when
no password is provided; verify that API behavior rather than assuming it.

Do not apply UseStateForUnknown to changing observations such as status or IP
without proving consistency. Preserve configured values the API cannot return;
refresh observable drift. After apply, computed unknown values must become
known values or null. Never mutate known planned inputs just to match an API
normalization or suppress an inconsistent-state error.

## 8. Read and resource-absence policy

Implement a shared present / absent / indeterminate observation helper.

An arbitrary HTTP 403 is **not** a not-found signal. It can mean authorization,
insufficient funds, or a missing/inaccessible object. Do not remove Terraform
state on a singular GET 403, an error-message substring, or an account lookup
alone. Do not assume an unverified 404 is authoritative either.

The verified VDS deletion check is a successful complete `VDS.List` response
that omits the exact ID. Use that account-scoped list as the reconciliation
source, assuming no filters/pagination are applied; revisit this policy if the
API introduces either. If GET fails but the list contains the ID, the VDS is
present. If the complete list succeeds and lacks the ID, absence is established.
If listing fails, retain state and return an indeterminate/error diagnostic.
Successful empty arrays establish absence; an undocumented empty-list 403 does
not. Never generalize the SDK tests' special empty action/disk-list messages to
VDS deletion or arbitrary resource reads.

Only confirmed absence permits a managed Read to call RemoveResource or a
Delete to finish. Refresh status/region and other fields omitted by Get from
the matching List entry, without one list call per attribute. Leave optional
observations null when neither route supplies them. Preserve unavailable
creation-only metadata already held in state.

## 9. Creation, deletion, failure recovery, and timeouts

Creation:

1. Read the plan, validate known selectors and sizing, and select SDK Create
   or CreateCustom. Read write-only values from configuration, not plan.
2. Submit creation once. Capture the returned ID immediately and write valid
   partial state before a later wait can fail. Keep that ID on diagnostics;
   otherwise a billed server can become orphaned.
3. Poll by ID until the resource converges. Live VDS status was `Running`
   and halted status was `halted`; compare the verified variants deliberately,
   without guessing other terminal states. A useful create completion includes
   the expected resource shape and a usable assigned IPv4 where applicable.
4. Refresh state without changing known planned inputs. On timeout, return the
   last observation and resource ID, not a fresh create attempt.

If a transport/decode failure occurs before an ID is available, creation may
still have been accepted. Report uncertain completion and recovery/import
steps. Do not automatically adopt a same-name VDS or resubmit the POST; names
are not unique idempotency keys. Do not destroy unrelated matching objects.

Deletion:

1. Reconcile the exact state ID; finish if already confirmed absent.
2. If not halted, request Shutdown once for graceful mode. Force mode uses
   HardShutdown and must be explicitly selected. Do not silently escalate to
   a power cut when graceful shutdown times out.
3. Use action/resource observations within the remaining deadline and confirm
   halted state. Do not rely only on an accepted action acknowledgement or
   replay a write because its response was uncertain.
4. Call VDS.Destroy only after halted state is confirmed. It can return 202 or
   204. Poll a successful full VDS listing until the ID disappears.
5. Retain state and diagnose uncertainty on error or timeout. Avoid treating
   a failed action-history lookup as proof that the server was deleted.

Initial operation defaults: create 15 minutes, read 2 minutes, delete 20 minutes;
poll reads around every five seconds with cancellation. These are explicit
project defaults to tune from evidence, not vendor SLAs. An individual HTTP
request retains a bounded timeout. Budget nested waits from the remaining
operation context and avoid holding account-wide locks while waiting. Serialize
dependent operations on one resource; independent resources may progress in
parallel. Offline tests must use short/injected waits rather than minute sleeps.

## 10. Import: account for unreadable provisioning selectors

The SDK's VDS model does not return the original image slug or standard size
slug. Matching vcpu/memory/disk to a catalog does not prove which plan or image
was used. Do not invent those values, erase configured selectors on refresh,
or advertise a no-op ID-only import that immediately plans replacement.

Implement a documented structured import identifier containing the VDS ID and
the original provisioning metadata, for example:

```json
{"id":"123456","image":"debian-12","region":"example-region","size":"example-size"}
```

For custom VDS, replace size with the custom_size object. These are explicit
user-supplied declarations for non-readable metadata, not observations of the
remote image. Validate the identifier and retrieve the actual VDS through the
SDK. Validate observable region/shape against supplied values where possible.
Populate state with observed attributes and declared creation metadata so the
same configuration can plan without accidental replacement. Never accept a
password/token in an import identifier. Set local-only defaults consistently.

Support plain-ID import only if a tested design can preserve these semantics;
otherwise return a helpful diagnostic pointing to structured import. Test
standard and custom imports, malformed IDs, missing metadata, a missing VDS,
permission failure, and a post-import no-op plan. Document that image metadata
cannot be independently verified by the current SDK. If richer API fields
become available, fix the SDK and simplify imports in a compatible way.

## 11. Verified limitations and live-test boundaries

The SDK's 2026-10-19 report established standard/custom creation, reads,
destruction after shutdown, hard shutdown, power-on, hard reboot, and graceful
shutdown. Graceful reboot and password reset remained in progress after bounded
ten-minute waits. Their guest-side effects are unknown. Disk attachment and
resize returned `Can not get Disk status.`; attachment fields stayed null and
one control-panel disk UUID was empty. Disk deletion is undocumented.

These observations are dated evidence, not a claim about current vendor health.
Do not spend money repeating guesses at the same blocked operations. Progress
on supported features and record what vendor clarification or SDK work is needed.

No live test runs just because TF_ACC is set. Separate acceptance tests against
local HTTP stubs from live tests. Live tests must additionally require an
explicit live opt-in, HEXCORE_TOKEN, and a separate write opt-in for mutations,
with a budget and target catalog inputs authorized in that provider session.
Do not inherit spending authorization from the earlier SDK experiment.

Never use preexisting account resources as disposable fixtures. The SDK report
identifies preexisting VDS 62527 as protected; do not mutate it. The report also
records billed disks left for manual cleanup; do not assume they have since
been removed, and do not create further disks for the MVP. Track each test-owned
VDS by ID and unique name prefix, register cleanup as soon as possible, and
report leftovers after failures. If funding is insufficient, stop billable work
and ask the maintainer to add funds; continue offline work independently.

Do not copy `.env`, Terraform state, plans, real account balances, credentials,
or full live responses from the SDK checkout. Create synthetic fixtures in this
repository. Reading the public verification report is enough for the handoff.

## 12. Testing and implementation milestones

Normal `go test ./...` must be deterministic and make no HexCore requests.
Use `httptest.Server` plus the real SDK for provider-level contract tests. Do
not mock away SDK decoding for every lifecycle test or duplicate its entire
transport suite. Use narrow interfaces/fakes only where they make resource
orchestration behavior materially easier to test.

Meaningful tests must cover:

- Provider config precedence, missing/empty/unknown values, no-network Configure,
  bad injected types, and secret redaction.
- Catalog mapping, stable ordering, nulls, monetary strings, and SDK errors.
- Mutually exclusive size modes, positive quantities, long names, unknown plan
  values, replacement decisions, and state-only deletion-policy updates.
- Standard/custom create, GB-to-MB handling, readiness, partial state after a
  wait failure, and no replay after an uncertain mutation response.
- Refresh drift, omitted Get fields, present/absent/indeterminate reconciliation,
  and retaining state for ambiguous 403s and list failures.
- Graceful/force deletion, halted precondition, accepted asynchronous deletion,
  already-absent objects, cancellation, action failure, and deletion timeout.
- Structured import and a no-op next plan, plus password-free state and plans.
- Full offline Terraform acceptance sequences using protocol-6 factories,
  local stub endpoints, create/refresh/replacement/import/destroy, and assertions
  that cleanup never targets an unrelated ID. Stub acceptance needs Terraform
  CLI, but never cloud credentials; fail if an unexpected production URL is used.

Suggested implementation checkpoints:

1. Bootstrap Go module/server and provider tests; pin the SDK and framework.
2. Deliver catalog data sources and one VDS observation helper with tests.
3. Deliver a standard-sized VDS vertical slice through offline acceptance.
4. Add custom sizing, import, write-only password, and failure/recovery cases.
5. Generate docs, validate examples, and report exact remaining live blockers.

At each checkpoint run checks appropriate to what exists: gofmt, go mod tidy,
go build, go vet, focused tests, then the suite. Run race checks where CGO and
a supported compiler are available. Demonstrate local Terraform discovery using
a dev_overrides CLI config; do not commit a machine-specific override. Generate
provider docs with terraform-plugin-docs, then confirm regeneration leaves no
diff. State which tests used local stubs and which, if any, touched HexCore.

## 13. Handoff completion criteria

Before declaring the implementation milestone complete, provide:

- Buildable provider with a pinned public SDK dependency and no committed local
  replacement/workspace requirement.
- Offline passing unit/contract/acceptance tests for the advertised surface.
- A post-apply and post-import no-op plan for the documented VDS examples.
- Correct handling of ID retention and ambiguous absence; no unsupported disk
  lifecycle or reset/reboot claims.
- Generated schema documentation, environment-based provider configuration,
  import and timeout examples, replacement warnings, and clear limitations.
- Small Conventional Commits and a clean handoff describing completed work,
  verification, unresolved issues, and the next concrete action.

Do not publish a first provider version until the maintainer requests it and
the advertised lifecycle has sufficient evidence. The published SDK's v1.0.0
does not, by itself, establish production readiness of the Terraform provider.

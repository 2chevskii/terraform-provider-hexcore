# Troubleshooting

| Symptom | Next step |
| --- | --- |
| Missing or invalid token | Set a nonempty raw `HEXCORE_TOKEN`, or correct the explicit `token`; unknown/empty configured values never fall back. |
| Unpublished provider cannot be installed | Build locally and use a temporary `dev_overrides` configuration; skip registry installation. |
| Region or image rejected before creation | Refresh the catalogs and choose an available region, valid slug and sufficient disk capacity. |
| Read reports indeterminate presence | Restore access/connectivity to the complete VDS list; keep the resource tracked until absence is confirmed. |
| A 403 appears after destroy | A singular 403 is not proof of removal. Confirm the exact ID is absent from a successful full listing. |
| Creation timed out with an ID | Inspect the tracked VDS and planned replacement before applying again. |
| Creation failed without an ID | Check whether the POST was accepted, then recover with structured import; do not blindly apply again. |
| Graceful deletion timed out | Inspect shutdown/action state. If a power cut is intended, explicitly apply `delete_mode = "force"` before retrying destroy. |
| Import plans replacement | Verify original image/size declarations and the non-secret password generation match configuration. |
| Password value changed but plan is empty | Increment `password_wo_version` to request whole-server replacement. |

Diagnostics deliberately omit raw API messages, request bodies and headers because
they can contain user data. Never paste tokens, passwords, account billing details,
full live responses, Terraform state or saved plans into a public issue.

Report a minimal configuration with synthetic selectors and IDs, provider/Go/
Terraform versions, the operation and safe status details. See
[verification and limits](./status) for unsupported features and known upstream blockers.

# Unconfirmed output-budget billing production cutover

Date: 2026-10-07 (Asia/Shanghai).

Source commit `59d352031` was pushed to `fork` on
`codex/tokenpay-fixed-package-20260927`. The verified source archive SHA-256 is
`b253a9f9c5647ea9c8b375067d22e71cfb0addcd283d1ab11eb99a1de85c62a9`.

Production identity:

- Container: `yuapi-production-output-budget-59d352031`.
- Image: `yuapi:production-output-budget-fix-20261007-59d352031`.
- Image ID: `sha256:2f86f865bb33dc029dd74c41a529f9110c7839eab4cd8f1e61f50ff92b9dc01c`.
- Binary SHA-256: `e9bc1e05ab6a15fe469c195244a4f86e4865cfad2f8ba79863e6225649e043d7`.
- Runtime: `59d352031-output-budget-fix-20261007`.
- Stable private port: `127.0.0.1:13064`.

The implementation and verification are described in
`2026-10-07-unconfirmed-output-budget-fix.md`. No frontend source, price option,
schema, model/channel mapping, Sub2API, or payment configuration change is
included in the application release.

Both YuAPI Caddy entries were switched to a healthy private candidate before
waiting for the previous production instance's incoming connections to reach
zero for three consecutive ten-second checks. The previous production instance
then stopped and the final master started on the stable port. New requests were
routed to it; the candidate also drained to zero for three checks before stop.
No socket was forcibly closed and no drain deadline interrupted user requests.

The final master and task-runner identity, healthy state, zero restarts, three
public domains' runtime versions, and persisted Caddy routing were verified.
All protected pre-existing containers retained their IDs/start times/restart
counts/states; pricing snapshots were byte-identical. The frontend main asset
`index.6263f2e360.js` retained SHA-256
`7d9386c9f3302e35fd73b1a169a05125201ee0478dfd6fe9905aa4f8de968d14`.

Read-only observation after the bridge found real missing-usage stream timeout
logs on the affected channel using observed cache-input estimates and an output
placeholder, without reservation-based settlement. One observed request with
120,345 estimated input tokens settled at 289 quota rather than a maximum
output budget. This observation complements the accepted-disconnect regression
tests; it does not establish that all upstream errors are resolved.

Release files: `/opt/newapi/releases/output-budget-fix-20261007-59d352031/`.
Private deployment backup and `hot-switch.sh rollback`:
`/opt/newapi/backups/20261007-output-budget-fix-59d352031/`.
Stopped old/candidate instances and their images remain available for rollback.
No schema restore is required to roll back this application release.

Retrospective ledger corrections were separate, explicitly authorized
transactions tied to original logs, completed after the fixed final version
was verified. Their private records and backups are retained outside tracked
repository documentation. Application rollback must not revert those ledger
corrections or restore a full database backup.

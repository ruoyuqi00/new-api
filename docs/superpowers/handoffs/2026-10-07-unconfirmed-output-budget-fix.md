# Unconfirmed stream output-budget settlement fix

Date: 2026-10-07 (Asia/Shanghai).

An accepted OpenAI-compatible text stream that lost its downstream connection
without authoritative usage settled its entire frozen output-token reservation.
For a request budget of 728,000,000 output tokens, this converted a maximum
budget into an enormous final charge despite only local input estimates and a
minimal output placeholder being available. The retention branch was introduced
in commit `60898e5f0` on 2026-09-16. Database-log and binlog deletion do not change
this code, the incoming output budget, or the frozen expression.

The accepted-disconnect reservation branch now applies only to fixed per-call
expressions. Token-priced OpenAI-compatible requests settle from the existing
observed-token policy: estimated input is cache read, and positive estimated
input retains a minimum output token. Authoritative usage still settles actual
tokens; unconfirmed Claude Messages remains zero charge; fixed per-call pricing
retains the agreed request price. The existing estimate-to-reservation upper
bound remains in place to prevent charging an estimate above its reservation.

Expression pre-consume also bounds its completion estimate at 1,000,000 tokens.
This bounds the reservation only, without modifying the forwarded request's
output limit or authoritative settlement. Ordinary explicit budgets such as
8,192 and 131,072 remain unchanged, as does the 8,192 fallback when absent.

Regression tests reproduce the former maximum-reservation charge before the
production edit, and verify settlement, persisted consume logs, user usage,
ordinary request limits, exact usage, Claude Messages, and fixed per-call
accepted disconnects. Fresh verification passed `go test ./...`, targeted
`-count=1` suites in service/helper/expression/model/controller/OpenAI/common,
`go vet ./service ./relay/helper ./pkg/billingexpr`, and `git diff --check`.
Independent read-only review found no Critical or Important issues.

This source change adds no schema migration, price-setting edit, frontend edit,
refund endpoint, or retrospective money operation. Any approved billing
corrections are separate operational transactions tied to their original logs.
Refund and correction details are retained privately outside this repository's
tracked documentation.

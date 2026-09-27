# TokenPay Fixed-Package Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a disabled-by-default fixed-USDT-package recharge option to YuAPI that uses a separate TokenPay service's per-order addresses and credits only authenticated, matching, single-use payments.

**Architecture:** YuAPI persists a pending top-up and immutable package snapshot, then signs a TokenPay `POST /CreateOrder` request with a unique order key. It validates the returned fixed quote before exposing TokenPay's hosted checkout. A dedicated callback verifies TokenPay's HMAC-SHA256 signature and settles an exact payment in one database transaction. Provider funds and wallet keys remain entirely outside YuAPI.

**Tech Stack:** Go 1.22+, Gin, GORM (SQLite/MySQL/PostgreSQL), `shopspring/decimal`, `testify`, React 19, TypeScript, Bun, React Query, project i18n.

**Spec:** `docs/superpowers/specs/2026-09-27-tokenpay-fixed-package-design.md`

## Global Constraints

- Baseline is YuAPI `f4cf272e0f3c083d9f0161fa962b6db59e13fd59` plus the design commit; preserve other payment providers and all user changes.
- Provider reference is `LightCountry/TokenPay` v1.2.0 commit `d27d7637306599a315b53a50a122c0e071a8586d`.
- TokenPay is separate. YuAPI stores no wallet private keys, mnemonic, or static receiving address.
- Only USDT on TRON/TRC20, BSC/BEP20, and Polygon is in scope.
- Fixed package USDT amounts are positive integers; site-balance entitlements are positive exact decimals; no global EPay price change.
- Provider mode is dynamic address, USD base, fixed USDT rate 1, HMAC-SHA256; received amount must equal the package amount.
- No production deployment, hot cutover, live wallet transfer, other-project mutation, or unapproved collection.
- Use `common.Marshal`, `common.Unmarshal`, or `common.DecodeJson` for Go JSON operations; use GORM-compatible SQLite/MySQL/PostgreSQL patterns.
- New Go backend tests use `testify/require` and `testify/assert`. New frontend text is translated in all six supported locales.

## File Map

| Unit | Files | Responsibility |
| --- | --- | --- |
| Settings | `setting/operation_setting/tokenpay_payment.go`, `controller/topup_tokenpay_admin.go` | Independent, write-only TokenPay configuration and fixed packages |
| Provider client | `service/tokenpay.go` | Sign and send create/query requests; verify callback signatures; no database access |
| Accounting | `model/tokenpay_topup.go`, `model/main.go`, `model/topup.go` | Immutable order snapshot, unique transaction ownership, atomic credit |
| HTTP | `controller/topup_tokenpay.go`, `router/api-router.go`, `controller/topup.go` | User order, owner-scoped status/hash submission, signed webhook, public payment options |
| Frontend | `web/default/src/features/wallet/**`, `web/default/src/features/system-settings/integrations/**`, locale JSON | Branded package/network selection and administrator configuration |
| Documentation | `docs/**` | Separate service setup, callback, funds/key handling, and release acceptance |

## Task 1: Payment Settings and Package Rules

**Files:** Create `setting/operation_setting/tokenpay_payment.go` and `setting/operation_setting/tokenpay_payment_test.go`.

**Interfaces:** Produce `TokenPayPaymentSetting`, `TokenPayTopUpPackage`, `GetTokenPayPaymentSetting()`, `(*TokenPayPaymentSetting).Normalize()`, `(*TokenPayPaymentSetting).FindPackage(usdt int64)`, and `TokenPayPackageQuota(balance decimal.Decimal) (int64, error)`. Keep the persisted secret write-only at the controller boundary.

- [ ] **Step 1:** Write table tests for duplicate/zero USDT packages, invalid or zero balances, unsupported networks, unsafe URLs, positive quota, max user quota, and an enabled setting missing URL/secret/packages.
- [ ] **Step 2:** Run `go test ./setting/operation_setting -run TokenPay -count=1`; confirm the new tests fail before implementation.
- [ ] **Step 3:** Implement normalization using the existing SHKeeper package validation as a business-rule reference, but store independent TokenPay settings with `enabled`, `base_url`, `api_token`, `packages`, `enabled_networks`, and `allow_private_url`.
- [ ] **Step 4:** Run the focused test and `go test ./setting/operation_setting -count=1`; commit as `feat: define TokenPay fixed-package settings`.

The package invariant under test is:

```go
setting := TokenPayPaymentSetting{Packages: []TokenPayTopUpPackage{{USDT: 10, Balance: "66"}}, EnabledNetworks: []string{"USDT_TRC20"}}
require.NoError(t, setting.Normalize())
assert.Equal(t, "66", setting.Packages[0].Balance)
```

## Task 2: Exact Provider Client and Signatures

**Files:** Create `service/tokenpay.go` and `service/tokenpay_test.go`.

**Interfaces:** Produce `NewTokenPayClient(baseURL, secret string, allowPrivateURL bool, httpClient *http.Client)`, `CreateOrder(ctx context.Context, request TokenPayCreateRequest) (TokenPayCreateResult, error)`, `QueryOrder(ctx context.Context, providerID string) (TokenPayQueryResult, error)`, and `VerifyTokenPayCallback(secret string, rawBody []byte) (TokenPayCallback, error)`.

- [ ] **Step 1:** Add deterministic signing tests using the official TokenPay HMAC-SHA256 vector from `Wiki/docs.md`: canonical fields sorted with ordinal/ASCII ordering, omitted empty values, monetary text retained, and constant-time verification. Test tampering and repeated keys.
- [ ] **Step 2:** Run `go test ./service -run TokenPay -count=1`; confirm the missing client/signature fails.
- [ ] **Step 3:** Implement a bounded HTTP client for `POST /CreateOrder` and signed `GET /Query`. Request fields are `OutOrderId`, `OrderUserKey`, `ActualAmount`, `Currency`, `NotifyUrl`, `RedirectUrl`, and `Signature`; use a unique order ID for `OrderUserKey`. The service base URL is SSRF-validated and never logs the secret.
- [ ] **Step 4:** Parse `success`, hosted checkout URL, provider order ID, chain amount, receiving address, and expiry. Reject HTTP 200 with `success=false`, invalid decimals, non-HTTPS checkout, wrong origin, and oversized responses. Use `common` JSON wrappers, not `encoding/json` marshal/unmarshal calls.
- [ ] **Step 5:** Test `httptest.Server` create/query responses, exact request body/signature, TLS URL policy, timeouts, and JSON errors. Run service package tests and commit as `feat: add signed TokenPay provider client`.

Example request fixture:

```go
request := TokenPayCreateRequest{OutOrderID: "USR1NOabc", OrderUserKey: "USR1NOabc-TRON", ActualAmount: "10", Currency: "USDT_TRC20", NotifyURL: "https://api.example.com/api/tokenpay/webhook", RedirectURL: "https://api.example.com/wallet"}
result, err := client.CreateOrder(ctx, request)
require.NoError(t, err)
assert.Equal(t, "10", result.PayAmountUSDT)
```

## Task 3: Additive Order Ledger and Atomic Settlement

**Files:** Create `model/tokenpay_topup.go`, `model/tokenpay_topup_test.go`; modify `model/main.go`, `model/topup.go`.

**Interfaces:** Produce `TokenPayTopUpOrder` and `TokenPayCreditedTransaction`; `CreateTokenPayTopUp(topUp *TopUp, order *TokenPayTopUpOrder) error`; `SaveTokenPayInvoice(...) error`; `SettleTokenPayTopUp(input TokenPaySettlementInput) (TokenPaySettlementResult, error)`; `GetTokenPayOrder(userID int, tradeNo string)`. Add provider/method constants without changing existing ones.

- [ ] **Step 1:** Add explicit SQLite fixture tests for immutable package and quota snapshots, same-amount distinct order identities, malformed provider identity, duplicate callback, reused transaction hash across orders, quota overflow, transaction rollback on affiliate/credit failure, and mismatch with other providers.
- [ ] **Step 2:** Run `go test ./model -run TokenPay -count=1`; confirm tests fail before implementation.
- [ ] **Step 3:** Add two GORM models and additive AutoMigrate entries. Store exact decimal strings (`RequestedUSDT`, `PackageBalance`, `ProviderAmountUSDT`), frozen integer quota, provider ID, address, network, top-up ID, status, and transaction hash; unique indexes constrain external order and `(network, tx_hash)`.
- [ ] **Step 4:** Implement credit in one `DB.Transaction`: lock top-up/order, verify frozen fields and pending status, insert unique transaction ownership, update top-up/order, call `CreditUserQuotaWithAffiliateRewardTx`. Duplicate identical completion returns no delta; a conflicting transaction fails without quota change.
- [ ] **Step 5:** Extend top-up history projection for TokenPay's decimal balance without changing ordinary provider JSON. Run focused model tests and commit as `feat: settle TokenPay packages atomically`.

Settlement contract:

```go
result, err := SettleTokenPayTopUp(TokenPaySettlementInput{
    TradeNo: "USR1NOabc", ProviderOrderID: "provider-1", Network: "USDT_TRC20",
    ReceiveAddress: "TKGTx4pCKiKQbk8evXHTborfZn754TGViP", ReceivedUSDT: "10", TransactionID: "tx-1",
})
require.NoError(t, err)
assert.True(t, result.CreditedQuotaDelta > 0)
again, err := SettleTokenPayTopUp(TokenPaySettlementInput{
    TradeNo: "USR1NOabc", ProviderOrderID: "provider-1", Network: "USDT_TRC20",
    ReceiveAddress: "TKGTx4pCKiKQbk8evXHTborfZn754TGViP", ReceivedUSDT: "10", TransactionID: "tx-1",
})
require.NoError(t, err)
assert.Equal(t, int64(0), again.CreditedQuotaDelta)
```

## Task 4: HTTP Purchase, Callback, and Recovery Claim

**Files:** Create `controller/topup_tokenpay.go`, `controller/topup_tokenpay_admin.go`, tests; modify `router/api-router.go`, `controller/topup.go`.

**Interfaces:** User `POST /api/user/tokenpay/pay`, `GET /api/user/tokenpay/order/:trade_no`, `POST /api/user/tokenpay/order/:trade_no/transaction`; provider `POST /api/tokenpay/webhook`; administrator `GET /api/option/tokenpay/status`, `POST /api/option/tokenpay/save`, `POST /api/option/tokenpay/test`.

- [ ] **Step 1:** Add Gin tests that assert disabled provider is absent; owner-only order lookup; unknown package/network rejected; exact USD/USDT quote accepted; different quote, address, identity, or URL rejected; repeated or forged callbacks add zero quota; valid paid callback adds one frozen entitlement; pending callback never credits.
- [ ] **Step 2:** Run `go test ./controller -run TokenPay -count=1`; confirm failure before adding routes.
- [ ] **Step 3:** Implement admin save/test with write-only secret semantics; re-use current option guard and payment compliance gating. Add user endpoints that create the local pending order before provider I/O; on a timeout, preserve the same trade number for reconciliation instead of creating a new order.
- [ ] **Step 4:** Implement callback using raw-body HMAC verification before trusting values. Match local order, provider ID, currency/network, exact amount, address, and transaction ID, then settle. Respond `ok` only after durable success; fail on DB or signature errors for provider retry.
- [ ] **Step 5:** Accept a bounded owner-scoped transaction hash claim as review-only state. Do not call a nonexistent TokenPay rescan API, mark the order paid, or credit on submission. Document TokenPay administrator's confirmed-on-chain expired-order supplement path.
- [ ] **Step 6:** Extend `GetTopUpInfo` with TokenPay packages/networks only when enabled; run controller and router tests; commit as `feat: expose TokenPay recharge and callback endpoints`.

## Task 5: Branded User and Admin UI

**Files:** Modify `web/default/src/features/wallet/**` and `web/default/src/features/system-settings/integrations/**`; add focused tests and locale entries in `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`.

**Interfaces:** Match Task 4 API routes. The payment page shows only package, selected chain, exact USDT amount, address/payment URL, status, and transaction-hash recovery; never displays a local exchange-rate derivation or wallet secret.

- [ ] **Step 1:** Read `web/default/AGENTS.md`, `shadcn-ui`, `vercel-react-best-practices`, and `i18n-translate` skills before UI edits. Add behavior tests for disabled state, package/network selection, link launch, expiry, owned order resume, and non-crediting hash submission.
- [ ] **Step 2:** Run focused Bun tests to confirm failure; implement with existing YuCore/SHKeeper layout conventions and responsive dialog scrolling, without changing global renderer/effects/styles.
- [ ] **Step 3:** Add administrator enable toggle, base URL, write-only secret, network toggles, package editor, connection test, and visible save state. Translate new strings in all six locales.
- [ ] **Step 4:** Run `bun test` focused files, `bun run typecheck`, `bun run build`, i18n checks, and touched-file lint. Capture desktop/mobile and light/dark screenshots for user review. Commit as `feat: configure and use TokenPay packages in YuAPI`.

## Task 6: Integration Verification and Handoff

**Files:** Add targeted docs under `docs/`; tests as needed.

- [ ] **Step 1:** Run `go test ./... -count=1`, focused Bun tests, typecheck, lint, and both frontend builds according to project scripts. Check `git diff --check` and compare with baseline to confirm unrelated UI effects and payment providers are unchanged.
- [ ] **Step 2:** With a local fake TokenPay server, execute full request -> invoice -> signed callback -> history flow and negative amount/duplicate/other-user cases. Reconcile any tests that fail; do not claim real three-chain acceptance.
- [ ] **Step 3:** Write setup instructions for operator-provided TokenPay URL/HMAC secret, `UseDynamicAddress`, fixed rate 1, network credentials, SQLite encrypted backup, collection disabled, manual hash review, and independent deployment. Document that BSC and Polygon remain unavailable until the separate scanner plan passes production RPC acceptance.
- [ ] **Step 4:** Start only the local YuAPI dev server needed to show the reviewed UI; report its URL. Do not start Docker or any production service. Commit verified docs/tests, then show branch/commit IDs and remaining real-money acceptance gates.

## Scope Split

The independent TokenPay scanner adaptation has its own plan,
`docs/superpowers/plans/2026-09-27-tokenpay-evm-rpc-scanner.md`. The YuAPI UI
must keep BSC/Polygon hidden until that scanner's real test confirms them.
The two plans share only TokenPay v1.2.0's order/callback contract and the
fixed-package design spec; neither repository imports the other's source.

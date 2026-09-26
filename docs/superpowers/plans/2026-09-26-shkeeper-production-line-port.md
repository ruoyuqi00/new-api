# SHKeeper Production-Line Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the reviewed SHKeeper fixed-package USDT recharge feature onto the current YuAPI production lineage while keeping the production YuCore UI, motion system, and global styling byte-for-byte unchanged.

**Architecture:** Reimplement the validated payment contracts from `codex/shkeeper-usdt@79ac694b3` semantically against production baseline `27023295d`; never merge or cherry-pick the old branch wholesale. Backend configuration, provider integration, order ledger, and settlement are ported first, then native production wallet/settings components consume the new APIs. Final acceptance compares an untouched baseline build and the ported build with identical fixture data and enforces a zero-diff guard over all YuCore motion and global style files.

**Tech Stack:** Go 1.22+, Gin, GORM v2, shopspring/decimal, React 19, TypeScript, TanStack Query, React Hook Form, Zod, Base UI, Tailwind CSS v4, Bun, Playwright/Edge

**Spec:** `docs/superpowers/specs/2026-09-26-shkeeper-production-line-port-design.md`

## Global Constraints

- Target baseline is `27023295d`; application source is `dc6f0bd01`; accepted renderer-aware UI ancestry is `a2099648d`.
- The old `codex/shkeeper-usdt` branch at `79ac694b3` is a logic reference only. Do not merge or cherry-pick it wholesale.
- Do not modify `web/default/src/features/yucore-brand/**` or `web/default/src/styles/**`.
- Do not modify global theme presets, entrance loaders, Canvas/WebGL renderers, render-loop scheduling, global navigation, or unrelated routes.
- Support SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6 with additive GORM-compatible schema changes.
- Use `common.Marshal`, `common.Unmarshal`, and `common.DecodeJson` for Go JSON operations.
- Fixed package USDT amounts are positive integers; balance values must produce quota in `[1, 2147483647]` with at most six decimal places.
- Snapshot requested USDT, balance, and quota entitlement on every new fixed-package order.
- Partial payment credits zero; full/late-full/overpayment credits the fixed entitlement once; excess value never increases credit.
- Generic manual top-up completion must reject SHKeeper orders.
- Use pinned SHKeeper v2.5.32 response shapes: quote `amount_crypto`; lookup only `external_id`, fiat fields, status, and transaction rows.
- API and backend keys are write-only. Callback fields alone never authorize balance credit.
- Frontend visible copy uses literal `t('English key')`; locale writes go only through `scripts/add-missing-keys.mjs`, followed by `bun run i18n:sync`.
- The payment UI must use current production wallet/settings patterns. Do not copy the old branch's black marketing panel or add a second brand system.
- Do not start Docker, touch Sub2API, call a live provider, move funds, modify production data, or hot-switch production.
- Keep `http://127.0.0.1:13035` and the old worktree only until the new candidate passes all gates. Delete them in Task 6 as explicitly authorized by the user.

---

## File Structure

### Backend

- `setting/operation_setting/shkeeper_payment.go`: supported networks, package/settings schema, URL validation, package normalization, and lookup.
- `setting/operation_setting/shkeeper_quota.go`: decimal-safe package quota conversion and signed `INT` ceiling.
- `model/shkeeper_topup.go`: provider order/transaction ledger, scheduling, fixed and legacy settlement.
- `model/main.go`: additive AutoMigrate registration.
- `model/topup.go`: reject SHKeeper from generic manual completion.
- `service/shkeeper.go`: v2.5.32 client, create/lookup DTO separation, webhook authentication, SSRF/redirect protection, transaction rescan.
- `service/shkeeper_reconcile.go`: provider lookup validation, confirmed transaction normalization, and settlement orchestration.
- `controller/topup_shkeeper.go`: authenticated invoice/order/recovery endpoints and anonymous signed webhook.
- `controller/topup_shkeeper_admin.go`: write-only settings CRUD and exact-amount connection test.
- `controller/topup.go`: expose enabled networks/packages without secrets or legacy rate fields.
- `controller/system_task_handlers.go`: scheduled reconciliation with bounded batches.
- `model/system_task.go`: SHKeeper task type when the production task framework requires a named type.
- `router/api-router.go`: user, webhook, and root settings routes.

### Frontend

- `web/default/src/features/system-settings/types.ts`: package/settings/connection API contracts.
- `web/default/src/features/system-settings/api.ts`: dedicated SHKeeper status/save/test requests.
- `web/default/src/features/system-settings/integrations/shkeeper-settings-model.ts`: Zod form schema and null-safe defaults.
- `web/default/src/features/system-settings/integrations/shkeeper-package-editor.tsx`: native production form rows and compact inline mapping preview.
- `web/default/src/features/system-settings/integrations/shkeeper-settings-section.tsx`: compose SHKeeper into production payment settings.
- `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`: mount the new section without changing the page shell.
- `web/default/src/features/wallet/types.ts`: public package/invoice types.
- `web/default/src/features/wallet/api.ts`: create/status/recovery requests.
- `web/default/src/features/wallet/hooks/use-shkeeper-payment.ts`: same-order polling, retained invoice state, retry, and credit refresh.
- `web/default/src/features/wallet/lib/shkeeper-payment-model.ts`: status/network/package helpers and business-error validation.
- `web/default/src/features/wallet/components/dialogs/shkeeper-payment-dialog.tsx`: native production dialog coordinator.
- `web/default/src/features/wallet/components/dialogs/shkeeper-package-picker.tsx`: compact package/network selection matching the existing recharge dialog.
- `web/default/src/features/wallet/components/dialogs/shkeeper-invoice-view.tsx`: exact amount, QR, address, status, and fixed credit.
- `web/default/src/features/wallet/components/dialogs/shkeeper-transaction-recovery.tsx`: hash rescan form and transaction list.
- `web/default/src/features/wallet/components/recharge-form-card.tsx`: open the independent fixed-package flow without weakening ordinary integer recharge.
- `web/default/src/features/wallet/index.tsx`: own dialog state and refresh user quota after credit.
- `web/default/src/i18n/static-keys.ts` and six locale JSON files: only actual new payment strings.

### Verification

- `docs/superpowers/handoffs/2026-09-26-shkeeper-production-line-local-verification.md`: source identity, automated evidence, visual/motion comparison, local URLs, cleanup record, and production gate.
- `.local-tests/shkeeper-production-line/`: ignored binaries, isolated databases, provider fixtures, screenshots, pixel samples, logs, and Playwright scripts.

---

### Task 1: Fixed-Package Configuration and Financial Ledger

**Files:**
- Create: `setting/operation_setting/shkeeper_payment.go`
- Create: `setting/operation_setting/shkeeper_payment_test.go`
- Create: `setting/operation_setting/shkeeper_quota.go`
- Create: `setting/operation_setting/shkeeper_quota_test.go`
- Create: `model/shkeeper_topup.go`
- Create: `model/shkeeper_topup_test.go`
- Create: `model/shkeeper_final_regression_test.go`
- Modify: `model/main.go`
- Modify: `model/topup.go`

**Interfaces:**
- Produces: `operation_setting.SHKeeperTopUpPackage`, `SHKeeperPaymentSetting`, `GetSHKeeperPaymentSetting()`, `FindPackage(int64)`, `SHKeeperPackageQuota(decimal.Decimal)`, `SHKeeperTopUpOrder`, `SettleSHKeeperTopUp(SHKeeperSettlementInput)`, and reconciliation scheduling queries.
- Consumes: existing `TopUp`, `User`, `lockForUpdate`, `common.QuotaPerUnit`, and GORM database globals.

- [ ] **Step 1: Write failing package normalization and quota-boundary tests**

Create table tests with the intended public contract:

```go
func TestSHKeeperPaymentSettingNormalizePackages(t *testing.T) {
	setting := SHKeeperPaymentSetting{
		Packages: []SHKeeperTopUpPackage{
			{USDT: 50, Balance: "330.000000", Label: " Common "},
			{USDT: 10, Balance: "66", Label: "Starter"},
		},
		EnabledNetworks: []string{"USDT", "BNB-USDT", "USDT"},
	}
	require.NoError(t, setting.Normalize())
	assert.Equal(t, []SHKeeperTopUpPackage{
		{USDT: 10, Balance: "66", Label: "Starter"},
		{USDT: 50, Balance: "330", Label: "Common"},
	}, setting.Packages)
	assert.Equal(t, []string{"BNB-USDT", "USDT"}, setting.EnabledNetworks)
}

func TestSHKeeperPackageQuotaBoundaries(t *testing.T) {
	original := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = original })

	_, err := SHKeeperPackageQuota(decimal.RequireFromString("0.004999"))
	require.ErrorContains(t, err, "at least one quota")
	assert.EqualValues(t, 1, mustPackageQuota(t, "0.005"))
	assert.EqualValues(t, SHKeeperMaxUserQuota, mustPackageQuota(t, "21474836.474999"))
	_, err = SHKeeperPackageQuota(decimal.RequireFromString("21474836.475"))
	require.ErrorContains(t, err, "quota limit")
}
```

Also cover duplicate/zero USDT, nonpositive balance, more than six decimal
places, unsupported networks, private URL opt-in, enabled-without-package, and
legacy `Rate`/`MinTopUp`/`MaxTopUp` values being ignored by fixed-package
normalization.

- [ ] **Step 2: Run the setting tests and observe RED**

Run:

```powershell
go test ./setting/operation_setting -run SHKeeper -count=1
```

Expected: compile failure because the SHKeeper types/functions do not exist.

- [ ] **Step 3: Implement normalized settings and decimal-safe quota conversion**

Use these exact interfaces:

```go
const (
	SHKeeperCryptoBNBUSDT     = "BNB-USDT"
	SHKeeperCryptoUSDT        = "USDT"
	SHKeeperCryptoPolygonUSDT = "POLYGON-USDT"
	SHKeeperInvoiceFiat       = "USD"
	SHKeeperMaxUserQuota int64 = 2147483647
)

type SHKeeperTopUpPackage struct {
	USDT    int64  `json:"usdt"`
	Balance string `json:"balance"`
	Label   string `json:"label,omitempty"`
}

type SHKeeperPaymentSetting struct {
	Enabled                  bool                   `json:"enabled"`
	BaseURL                  string                 `json:"base_url"`
	APIKey                   string                 `json:"api_key"`
	BackendKey               string                 `json:"backend_api_key"`
	Packages                 []SHKeeperTopUpPackage `json:"packages"`
	EnabledNetworks          []string               `json:"enabled_networks"`
	InvoiceExpiryMinutes     int                    `json:"invoice_expiry_minutes"`
	ReconcileIntervalSeconds int                    `json:"reconcile_interval_seconds"`
	AllowPrivateURL          bool                   `json:"allow_private_url"`
	Rate                     string                 `json:"rate"`
	MinTopUp                 int                    `json:"min_top_up"`
	MaxTopUp                 int                    `json:"max_top_up"`
}
```

Normalize into non-nil sorted slices. Parse balances with
`decimal.NewFromString`, enforce precision before canonicalizing with
`decimal.String()`, and call `SHKeeperPackageQuota` during normalization. Keep
legacy pricing fields stored but never validate or use them for fixed packages.

- [ ] **Step 4: Write failing fixed settlement tests before the model implementation**

Protect threshold, replay, conversion snapshot, capacity, and manual completion:

```go
func TestSettleSHKeeperFixedPackageCreditsOnlyOnceAtThreshold(t *testing.T) {
	user, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	partial := settleSHKeeper(t, order, user, "9", SHKeeperOrderStatusPartial, "tx-1")
	assert.Zero(t, partial.CreditedQuotaDelta)
	assert.Equal(t, "0", partial.CreditedBalance)

	paid := settleSHKeeper(t, order, user, "10", SHKeeperOrderStatusPaid, "tx-2")
	assert.EqualValues(t, 6600, paid.CreditedQuotaDelta)

	common.QuotaPerUnit = 200
	replayed := settleSHKeeper(t, order, user, "12", SHKeeperOrderStatusOverpaid, "tx-3")
	assert.Zero(t, replayed.CreditedQuotaDelta)
	assert.EqualValues(t, 6600, replayed.CreditedQuota)
}

func TestManualCompleteTopUpRejectsSHKeeper(t *testing.T) {
	_, order := setupFixedSHKeeperOrder(t, "10", "66", 6600)
	err := ManualCompleteTopUp(order.TradeNo, "127.0.0.1")
	require.ErrorContains(t, err, "SHKeeper")
}
```

Add deterministic tests for duplicate/conflicting transaction IDs, concurrent
replay, user quota exactly at/one over the signed INT ceiling, first completion
timestamp, underfunded expiry, final-order aging, pre-snapshot fail-closed fixed
orders, and empty-mode legacy rate settlement.

- [ ] **Step 5: Run model tests and observe RED**

Run:

```powershell
go test ./model -run 'SHKeeper|ManualCompleteTopUpRejectsSHKeeper' -count=1
```

Expected: compile failures for missing models and failing manual-completion
behavior.

- [ ] **Step 6: Implement additive models and transactional settlement**

Use the reviewed order fields:

```go
const SHKeeperSettlementModeFixedPackage = "fixed_package"

type SHKeeperTopUpOrder struct {
	ID                int64  `json:"id" gorm:"primaryKey"`
	TopUpID           int    `json:"top_up_id" gorm:"index"`
	TradeNo           string `json:"trade_no" gorm:"size:255;uniqueIndex"`
	UserID            int    `json:"user_id" gorm:"index"`
	ExternalID        string `json:"external_id" gorm:"size:255;uniqueIndex"`
	Crypto            string `json:"crypto" gorm:"size:32;index"`
	SettlementMode    string `json:"settlement_mode" gorm:"size:32;index"`
	RequestedUSDT     string `json:"requested_usdt" gorm:"size:64"`
	PackageBalance    string `json:"package_balance" gorm:"size:64"`
	PackageQuota      int64  `json:"package_quota" gorm:"type:bigint"`
	RequestedBalance  string `json:"requested_balance" gorm:"size:64"`
	LockedRate        string `json:"locked_rate" gorm:"size:64"`
	QuotedUSDT        string `json:"quoted_usdt" gorm:"size:64"`
	InvoiceAddress    string `json:"invoice_address" gorm:"size:255"`
	CallbackURL       string `json:"-" gorm:"size:512"`
	ProviderInvoiceID string `json:"provider_invoice_id" gorm:"size:255"`
	ReceivedUSDT      string `json:"received_usdt" gorm:"size:64"`
	CreditedBalance   string `json:"credited_balance" gorm:"size:64"`
	CreditedQuota     int64  `json:"credited_quota" gorm:"type:bigint"`
	Status            string `json:"status" gorm:"size:32;index"`
	ProviderSummary   string `json:"-" gorm:"type:text"`
	CreatedAt         int64  `json:"created_at"`
	ExpiresAt         int64  `json:"expires_at"`
	CompletedAt       int64  `json:"completed_at"`
	LastReconciledAt  int64  `json:"last_reconciled_at" gorm:"index"`
}
```

Keep the credited-transaction unique key `(crypto, tx_id, order_id)`. Perform
deduplication before summing and reject one normalized ID with different decimal
amounts. In the locked transaction, conditionally update user quota only when
`quota <= SHKeeperMaxUserQuota - packageQuota`. Preserve the first funded
classification and `CompletedAt`; repair coherent already-credited zero-snapshot
records without adding quota and reject uncredited zero-snapshot funding.

Register both models in every existing AutoMigrate list in `model/main.go`.
Reject SHKeeper provider or method in `ManualCompleteTopUp` immediately after
locking/loading the generic top-up and before its success/idempotency branch.

- [ ] **Step 7: Run focused and full model tests**

Run:

```powershell
gofmt -w setting/operation_setting/shkeeper_payment.go setting/operation_setting/shkeeper_payment_test.go setting/operation_setting/shkeeper_quota.go setting/operation_setting/shkeeper_quota_test.go model/shkeeper_topup.go model/shkeeper_topup_test.go model/shkeeper_final_regression_test.go model/main.go model/topup.go
go test ./setting/operation_setting ./model -run 'SHKeeper|ManualCompleteTopUpRejectsSHKeeper' -count=1
go test ./model -count=1
git diff --check
```

Expected: PASS.

- [ ] **Step 8: Verify immutable UI paths remain untouched**

Run:

```powershell
git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles
```

Expected: no output and exit `0`.

- [ ] **Step 9: Commit the ledger foundation**

```powershell
git add setting/operation_setting/shkeeper_payment.go setting/operation_setting/shkeeper_payment_test.go setting/operation_setting/shkeeper_quota.go setting/operation_setting/shkeeper_quota_test.go model/shkeeper_topup.go model/shkeeper_topup_test.go model/shkeeper_final_regression_test.go model/main.go model/topup.go
git commit -m "feat: add fixed-package SHKeeper ledger"
```

---

### Task 2: Provider API, Reconciliation, and HTTP Contracts

**Files:**
- Create: `service/shkeeper.go`
- Create: `service/shkeeper_test.go`
- Create: `service/shkeeper_reconcile.go`
- Create: `service/shkeeper_reconcile_test.go`
- Create: `controller/topup_shkeeper.go`
- Create: `controller/topup_shkeeper_test.go`
- Create: `controller/topup_shkeeper_admin.go`
- Create: `controller/topup_shkeeper_admin_test.go`
- Create: `controller/shkeeper_system_task_test.go`
- Modify: `controller/topup.go`
- Modify: `controller/system_task_handlers.go`
- Modify: `controller/payment_webhook_availability_test.go`
- Modify: `router/api-router.go`
- Modify if required by the current framework: `model/system_task.go`

**Interfaces:**
- Consumes: Task 1 settings, quota snapshot, ledger models, settlement, and the production system-task framework.
- Produces: v2.5.32 client methods, reconciliation, user/admin/webhook APIs, top-up info packages, and scheduled repair of missed callbacks.

- [ ] **Step 1: Write exact v2.5.32 client tests before creating the client**

Use provider fixtures copied from the pinned serializer:

```json
{
  "status": "success",
  "amount_crypto": "10",
  "exchange_rate": "1",
  "fiat": "USD",
  "amount_fiat": "10",
  "crypto": "USDT"
}
```

```json
{
  "status": "success",
  "invoices": [{
    "external_id": "USDT1abc",
    "fiat": "USD",
    "amount_fiat": "10",
    "balance_fiat": "10",
    "status": "PAID",
    "txs": [{
      "amount": "10",
      "crypto": "USDT",
      "addr": "TAddress",
      "txid": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "status": "CONFIRMED"
    }]
  }]
}
```

Tests must assert API-key headers, backend-key rescan headers, quote decoding,
creation payload, lookup shape, transaction-hash formats, raw-body HMAC with
five-minute replay window, cross-host redirect rejection, metadata/link-local
blocking, response size limit, and secret redaction from provider errors.

- [ ] **Step 2: Run the client tests and observe RED**

Run:

```powershell
go test ./service -run 'SHKeeperClient|VerifySHKeeperWebhook|NormalizeSHKeeper' -count=1
```

Expected: compile failure because the client is absent.

- [ ] **Step 3: Implement the provider client with separate create/lookup DTOs**

Use these method signatures:

```go
func NewSHKeeperClient(config SHKeeperClientConfig, client *http.Client) (*SHKeeperClient, error)
func (client *SHKeeperClient) ListCrypto(ctx context.Context) ([]SHKeeperCrypto, error)
func (client *SHKeeperClient) Quote(ctx context.Context, crypto string, fiat string, amount decimal.Decimal) (*SHKeeperQuote, error)
func (client *SHKeeperClient) CreatePaymentRequest(ctx context.Context, input SHKeeperPaymentRequest) (*SHKeeperInvoice, error)
func (client *SHKeeperClient) GetInvoiceByExternalID(ctx context.Context, crypto string, externalID string) (*SHKeeperInvoiceLookup, error)
func (client *SHKeeperClient) NotifyTransaction(ctx context.Context, crypto string, transactionID string) error
func VerifySHKeeperWebhook(apiKey string, timestamp string, signature string, rawBody []byte, now time.Time) error
```

`SHKeeperQuote.CryptoAmount` uses `json:"amount_crypto"`. The create DTO has
provider `Amount` and `Wallet`; the lookup DTO does not. Use `common.Marshal`
and `common.Unmarshal`, copy the HTTP client before setting timeout/redirect
policy, cap responses at `2 << 20`, and normalize only the three allowed crypto
identifiers.

- [ ] **Step 4: Write failing reconciliation and controller tests**

Cover:

- fixed partial `9/10` credits zero and remains recoverable;
- full, late-full, and overpaid credit once;
- callback/reconciliation replay and duplicate transaction rows;
- address/network/amount/external-ID mismatch;
- addressless pending lookup never creates a provider invoice;
- failed attempts update `last_reconciled_at` and rotate a bounded batch;
- package tampering and user quota capacity reject before provider create;
- ambiguous create response makes exactly one provider create call;
- manual hash rescan never changes balance directly;
- signed callback queries the provider and returns 202 only after safe
  reconciliation;
- dedicated/general settings responses expose neither secret and return `[]`
  for empty networks/packages;
- exact connection test uses the smallest package and `amount_matches`;
- public top-up info emits `shkeeper_packages`, networks, expiry, and recovery
  flag without rate/min/max/secrets.

Use `httptest.Server`, real Gin handlers, the main test database, and
`testify/require`/`assert`.

- [ ] **Step 5: Run focused service/controller tests and observe RED**

Run:

```powershell
go test ./service ./controller -run SHKeeper -count=1
```

Expected: missing reconciliation/controller symbols and missing routes/contracts.

- [ ] **Step 6: Implement reconciliation with lookup-only recovery**

`ReconcileSHKeeperOrder(ctx, tradeNo)` must:

1. load the local order;
2. record a monotonic attempt timestamp before remote work;
3. build a client from current write-only settings;
4. query by external ID and never call create;
5. validate external ID, `USD`, and lookup `amount_fiat` against requested USDT;
6. require confirmed transaction crypto/address to match the stored order;
7. trim/lowercase transaction IDs, deduplicate equivalent decimal values, and
   reject conflicts;
8. call `SettleSHKeeperTopUp`;
9. write a top-up audit log only for a positive committed quota delta.

Do not infer invoice wallet/crypto amount from lookup. If the local order lacks
an address, return a recoverable error after recording the attempt.

- [ ] **Step 7: Implement user, webhook, and admin contracts**

The create request and response are:

```go
type shkeeperPayRequest struct {
	USDTAmount int64  `json:"usdt_amount"`
	Crypto     string `json:"crypto"`
}

type SHKeeperTopUpResponse struct {
	TradeNo         string   `json:"trade_no"`
	Network         string   `json:"network"`
	Crypto          string   `json:"crypto"`
	USDTAmount      string   `json:"usdt_amount"`
	BalanceAmount   string   `json:"balance_amount"`
	Address         string   `json:"address"`
	QRPayload       string   `json:"qr_payload"`
	Status          string   `json:"status"`
	ExpiresAt       int64    `json:"expires_at"`
	ReceivedUSDT    string   `json:"received_usdt"`
	CreditedBalance string   `json:"credited_balance"`
	TransactionIDs  []string `json:"transaction_ids,omitempty"`
}
```

Creation resolves the package server-side, checks the user's signed-INT
capacity, inserts the local generic/provider order atomically, and calls
provider create once. On an ambiguous failure it may query but never call
create again. Validate the create response before storing/showing the address.

The webhook reads at most 1 MiB of raw body, verifies timestamp/signature before
decoding, verifies local callback identity, and treats the callback only as a
wake-up for authenticated lookup.

Admin settings preserve blank submitted secrets, normalize before saving with
`config.ConfigToMap`, prefix keys with `shkeeper_payment.`, and return only
`api_key_configured`/`backend_key_configured`. Connection readiness requires all
enabled cryptos plus an exact smallest-package quote.

- [ ] **Step 8: Register routes and scheduled reconciliation**

Add:

```text
POST /api/shkeeper/webhook
POST /api/user/shkeeper/pay
GET  /api/user/shkeeper/order/:trade_no
POST /api/user/shkeeper/order/:trade_no/transaction
GET  /api/option/shkeeper/status
POST /api/option/shkeeper/save
POST /api/option/shkeeper/test
```

Use existing anonymous body limits, `UserAuth`, `CriticalRateLimit`, and
`RootAuth` patterns. Register a scheduled task handler whose interval reads the
normalized setting and whose bounded run reports processed/credited/failed.
Existing invoices continue reconciling when new invoice creation is disabled.

- [ ] **Step 9: Run focused and full backend verification**

Run:

```powershell
gofmt -w service/shkeeper.go service/shkeeper_test.go service/shkeeper_reconcile.go service/shkeeper_reconcile_test.go controller/topup_shkeeper.go controller/topup_shkeeper_test.go controller/topup_shkeeper_admin.go controller/topup_shkeeper_admin_test.go controller/shkeeper_system_task_test.go controller/topup.go controller/system_task_handlers.go controller/payment_webhook_availability_test.go router/api-router.go model/system_task.go
go test ./setting/operation_setting ./model ./service ./controller -run SHKeeper -count=1
go test ./... -count=1
git diff --check
git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles
```

Expected: PASS and no immutable UI diff.

- [ ] **Step 10: Commit the provider and API port**

```powershell
git add service/shkeeper.go service/shkeeper_test.go service/shkeeper_reconcile.go service/shkeeper_reconcile_test.go controller/topup_shkeeper.go controller/topup_shkeeper_test.go controller/topup_shkeeper_admin.go controller/topup_shkeeper_admin_test.go controller/shkeeper_system_task_test.go controller/topup.go controller/system_task_handlers.go controller/payment_webhook_availability_test.go router/api-router.go model/system_task.go
git commit -m "feat: integrate SHKeeper with production API"
```

---

### Task 3: Native Production Payment Settings UI

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/api.ts`
- Create: `web/default/src/features/system-settings/integrations/shkeeper-settings-model.ts`
- Create: `web/default/src/features/system-settings/integrations/shkeeper-settings-model.test.ts`
- Create: `web/default/src/features/system-settings/integrations/shkeeper-package-editor.tsx`
- Create: `web/default/src/features/system-settings/integrations/shkeeper-settings-section.tsx`
- Modify: `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`

**Interfaces:**
- Consumes: Task 2 admin status/save/test JSON.
- Produces: production-native package editor and write-only connection settings without touching page shell or motion code.

- [ ] **Step 1: Load required frontend skills and inspect target production patterns**

Read `web/default/AGENTS.md`, `shadcn-ui`, `vercel-react-best-practices`, and
`i18n-translate`. Run:

```powershell
bunx shadcn@latest info --json
```

Read the target branch's existing payment settings fields, switch rows, secret
inputs, alerts, buttons, spacing, and section boundaries. Do not use the old
branch's CSS or top-level composition as visual authority.

- [ ] **Step 2: Write failing settings schema tests**

Test null-safe first use and package rules:

```ts
test('normalizes first-install omitted and null arrays', () => {
  const defaults = buildSHKeeperFormDefaults({
    enabled: false,
    base_url: '',
    api_key_configured: false,
    backend_key_configured: false,
    packages: null,
    enabled_networks: null,
    invoice_expiry_minutes: 30,
    reconcile_interval_seconds: 60,
    allow_private_url: false,
  })
  assert.deepEqual(defaults.packages, [])
  assert.deepEqual(defaults.enabled_networks, [])
})

test('rejects duplicate fixed package USDT amounts', () => {
  const result = createSHKeeperSettingsSchema(translate).safeParse(
    fixedSettings([{ usdt: 10, balance: '66', label: '' }, { usdt: 10, balance: '70', label: '' }])
  )
  assert.equal(result.success, false)
})
```

Also test enabled network/package requirements, positive integer USDT, positive
balance, omitted label normalization, deep-copy defaults, and array root error
selection.

- [ ] **Step 3: Run the schema test and observe RED**

Run from `web/default`:

```powershell
bun test src/features/system-settings/integrations/shkeeper-settings-model.test.ts
```

Expected: missing module/type failures.

- [ ] **Step 4: Implement TypeScript contracts, API calls, and schema**

Use:

```ts
export type SHKeeperTopUpPackage = {
  usdt: number
  balance: string
  label?: string
}

export type SHKeeperSettingsStatus = {
  enabled: boolean
  base_url: string
  api_key_configured: boolean
  backend_key_configured: boolean
  packages: SHKeeperTopUpPackage[] | null
  enabled_networks: SHKeeperNetwork[] | null
  invoice_expiry_minutes: number
  reconcile_interval_seconds: number
  allow_private_url: boolean
}
```

Keep nullable API inputs at the boundary; `buildSHKeeperFormDefaults` returns
non-null arrays and normalizes `label ?? ''`. The schema coerces whole USDT,
trims strings, rejects duplicates, and requires networks/packages only when
enabled.

Dedicated API functions call `/api/option/shkeeper/status`, `/save`, and `/test`
through the project `api` instance.

- [ ] **Step 5: Build a production-native package editor**

Use `useFieldArray`, existing `FormField`, `Field`, `FieldGroup`, `Input`,
`Button`, `Tooltip`, `Badge`, and the configured Hugeicons library. Requirements:

- no raw JSON;
- direct row inputs for USDT, credited balance, optional label;
- icon-only remove with tooltip;
- add package command;
- direct/root form errors visible and associated with inputs;
- compact inline preview that wraps long labels;
- responsive rows without fixed/sticky panels;
- semantic tokens only, no gradients/raw colors/manual dark overrides;
- no card inside a card and no separate branded preview surface.

Derive preview from watched form values; do not synchronize duplicate state in
an effect.

- [ ] **Step 6: Compose the section into existing payment settings**

Keep the current payment settings page shell and every existing payment method.
Add SHKeeper as one existing-style integration section with URL, write-only API
key/backend key, networks, package editor, expiry, reconciliation interval,
private URL opt-in, test, and save. Connection badges use
`available && quote_ok && amount_matches`.

Every visible string uses `t('literal English key')`; defer locale writes to
Task 5.

- [ ] **Step 7: Run focused frontend checks**

Run from `web/default`:

```powershell
bun test src/features/system-settings/integrations/shkeeper-settings-model.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/system-settings/types.ts src/features/system-settings/api.ts src/features/system-settings/integrations/shkeeper-settings-model.ts src/features/system-settings/integrations/shkeeper-settings-model.test.ts src/features/system-settings/integrations/shkeeper-package-editor.tsx src/features/system-settings/integrations/shkeeper-settings-section.tsx src/features/system-settings/integrations/payment-settings-section.tsx
bunx oxfmt --check src/features/system-settings/types.ts src/features/system-settings/api.ts src/features/system-settings/integrations/shkeeper-settings-model.ts src/features/system-settings/integrations/shkeeper-settings-model.test.ts src/features/system-settings/integrations/shkeeper-package-editor.tsx src/features/system-settings/integrations/shkeeper-settings-section.tsx src/features/system-settings/integrations/payment-settings-section.tsx
```

Expected: PASS.

- [ ] **Step 8: Verify production visuals remain immutable and commit**

Run:

```powershell
git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles
git diff --check
git add web/default/src/features/system-settings/types.ts web/default/src/features/system-settings/api.ts web/default/src/features/system-settings/integrations/shkeeper-settings-model.ts web/default/src/features/system-settings/integrations/shkeeper-settings-model.test.ts web/default/src/features/system-settings/integrations/shkeeper-package-editor.tsx web/default/src/features/system-settings/integrations/shkeeper-settings-section.tsx web/default/src/features/system-settings/integrations/payment-settings-section.tsx
git commit -m "feat: configure SHKeeper in production settings"
```

---

### Task 4: Native Production Wallet Flow

**Files:**
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/api.ts`
- Create: `web/default/src/features/wallet/hooks/use-shkeeper-payment.ts`
- Modify: `web/default/src/features/wallet/hooks/index.ts`
- Create: `web/default/src/features/wallet/lib/shkeeper-payment-model.ts`
- Create: `web/default/src/features/wallet/lib/shkeeper-payment-model.test.ts`
- Modify: `web/default/src/features/wallet/lib/payment.ts`
- Create or modify: `web/default/src/features/wallet/lib/payment.test.ts`
- Create: `web/default/src/features/wallet/components/dialogs/shkeeper-payment-dialog.tsx`
- Create: `web/default/src/features/wallet/components/dialogs/shkeeper-package-picker.tsx`
- Create: `web/default/src/features/wallet/components/dialogs/shkeeper-invoice-view.tsx`
- Create: `web/default/src/features/wallet/components/dialogs/shkeeper-transaction-recovery.tsx`
- Modify: `web/default/src/features/wallet/components/recharge-form-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`
- Modify: `web/default/src/features/wallet/hooks/use-payment.ts`
- Modify: `web/default/src/features/wallet/hooks/use-waffo-payment.ts`
- Modify: `web/default/src/features/wallet/hooks/use-waffo-pancake-payment.ts`

**Interfaces:**
- Consumes: Task 2 public package/invoice APIs and existing production wallet dialog primitives.
- Produces: fixed-package selection and same-dialog invoice lifecycle while preserving ordinary payment behavior.

- [ ] **Step 1: Write failing payment model tests**

Protect immutable sorting, status rules, business-error retention, and ordinary
integer input:

```ts
test('sorts fixed packages without mutating API data', () => {
  const source = [
    { usdt: 50, balance: '330', label: 'Common' },
    { usdt: 10, balance: '66' },
  ]
  assert.deepEqual(sortSHKeeperPackages(source).map((item) => item.usdt), [10, 50])
  assert.deepEqual(source.map((item) => item.usdt), [50, 10])
})

test('ordinary recharge rejects fractional or unsafe amounts', () => {
  for (const value of ['10.5', '-1', '1e3', '0', '9007199254740992']) {
    assert.equal(parseOrdinaryTopUpAmount(value), null)
  }
  assert.equal(parseOrdinaryTopUpAmount('10'), 10)
})
```

Use a deterministic TanStack QueryClient test to prove an HTTP-200
`success:false` order response rejects, retains the last valid invoice, and a
same-order refetch can recover without another create request. Keep tests free
of sleeps/timing assertions.

- [ ] **Step 2: Run wallet tests and observe RED**

Run from `web/default`:

```powershell
bun test src/features/wallet/lib/shkeeper-payment-model.test.ts src/features/wallet/lib/payment.test.ts
```

Expected: missing helper/module failures.

- [ ] **Step 3: Add public wallet contracts and hook**

Use:

```ts
export type SHKeeperNetwork = 'BNB-USDT' | 'USDT' | 'POLYGON-USDT'

export type SHKeeperInvoice = {
  trade_no: string
  network: string
  crypto: SHKeeperNetwork
  usdt_amount: string
  balance_amount: string
  address: string
  qr_payload: string
  status: SHKeeperOrderStatus
  expires_at: number
  received_usdt: string
  credited_balance: string
  transaction_ids?: string[]
}

export type SHKeeperPaymentRequest = {
  usdt_amount: number
  crypto: SHKeeperNetwork
}
```

Extend `TopupInfo` with enabled flag, packages, networks, expiry, and recovery
flag. API methods call create/status/rescan endpoints and use the existing API
instance's business-error behavior consistently.

`useSHKeeperPayment` keeps one trade number, polls every three seconds only for
nonterminal orders, limits each fetch retry, retains the last valid invoice on
failure, exposes `pollingError` and `retryOrder`, and never calls create from a
retry. Positive credited balance changes refresh the user's wallet once.

- [ ] **Step 4: Build the package picker in the existing production dialog language**

The picker must look like the current online USDT/recharge dialog, not the old
reference branch's redesign:

- existing `DialogContent`, header title/icon, description, body spacing, and
  footer action hierarchy;
- compact package choices showing `N USDT` and `credited balance`;
- existing-style network safety `Alert`;
- three network choices using the project's current option-set component;
- selected state through existing semantic variants, border, and check/radio;
- no left black marketing panel, hero copy, new background, or global effects;
- desktop four-package row and mobile two-column packages/one-column networks;
- normal body scrolling with footer and recovery reachable;
- stable border widths and dimensions across selection.

- [ ] **Step 5: Build invoice and transaction recovery states**

The same dialog transitions after create. Render exact whole USDT first, then
fixed credited balance, network warning, QR, address, expiry, received value,
status badge, and detected transaction IDs. Recoverable states show a hash input
and submit action. Paid/overpaid/late copy must state that excess USDT does not
increase the fixed package credit.

Polling errors render an inline localized alert with Retry; retain visible
amount/address and retry only the current order. Closing unmounts the session,
stops polling, and clears package/network/hash selections.

- [ ] **Step 6: Keep ordinary recharge integer-only and independent**

`RechargeFormCard` opens SHKeeper independently of the ordinary amount and does
not apply ordinary minimum validation. Restore the ordinary input to
`inputMode='numeric'`, `step={1}`, and digit-only positive safe integers.
`use-payment`, Waffo, and Waffo Pancake reject invalid values instead of
flooring. Existing Stripe/Epay/Creem behavior and amount display remain intact.

- [ ] **Step 7: Run focused frontend verification**

Run from `web/default`:

```powershell
bun test src/features/wallet/lib/shkeeper-payment-model.test.ts src/features/wallet/lib/payment.test.ts
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/wallet/types.ts src/features/wallet/api.ts src/features/wallet/hooks/use-shkeeper-payment.ts src/features/wallet/hooks/index.ts src/features/wallet/lib/shkeeper-payment-model.ts src/features/wallet/lib/shkeeper-payment-model.test.ts src/features/wallet/lib/payment.ts src/features/wallet/lib/payment.test.ts src/features/wallet/components/dialogs/shkeeper-payment-dialog.tsx src/features/wallet/components/dialogs/shkeeper-package-picker.tsx src/features/wallet/components/dialogs/shkeeper-invoice-view.tsx src/features/wallet/components/dialogs/shkeeper-transaction-recovery.tsx src/features/wallet/components/recharge-form-card.tsx src/features/wallet/index.tsx src/features/wallet/hooks/use-payment.ts src/features/wallet/hooks/use-waffo-payment.ts src/features/wallet/hooks/use-waffo-pancake-payment.ts
bunx oxfmt --check src/features/wallet/types.ts src/features/wallet/api.ts src/features/wallet/hooks/use-shkeeper-payment.ts src/features/wallet/hooks/index.ts src/features/wallet/lib/shkeeper-payment-model.ts src/features/wallet/lib/shkeeper-payment-model.test.ts src/features/wallet/lib/payment.ts src/features/wallet/lib/payment.test.ts src/features/wallet/components/dialogs/shkeeper-payment-dialog.tsx src/features/wallet/components/dialogs/shkeeper-package-picker.tsx src/features/wallet/components/dialogs/shkeeper-invoice-view.tsx src/features/wallet/components/dialogs/shkeeper-transaction-recovery.tsx src/features/wallet/components/recharge-form-card.tsx src/features/wallet/index.tsx src/features/wallet/hooks/use-payment.ts src/features/wallet/hooks/use-waffo-payment.ts src/features/wallet/hooks/use-waffo-pancake-payment.ts
```

Expected: PASS.

- [ ] **Step 8: Verify immutable UI paths and commit**

Run:

```powershell
git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles
git diff --check
git add web/default/src/features/wallet
git commit -m "feat: add native fixed-package wallet flow"
```

---

### Task 5: Localization, Full Verification, and Dual-Candidate Visual Audit

**Files:**
- Modify via script only: `web/default/src/i18n/locales/en.json`
- Modify via script only: `web/default/src/i18n/locales/zh.json`
- Modify via script only: `web/default/src/i18n/locales/fr.json`
- Modify via script only: `web/default/src/i18n/locales/ja.json`
- Modify via script only: `web/default/src/i18n/locales/ru.json`
- Modify via script only: `web/default/src/i18n/locales/vi.json`
- Modify only for runtime-generated keys: `web/default/src/i18n/static-keys.ts`
- Create: `docs/superpowers/handoffs/2026-09-26-shkeeper-production-line-local-verification.md`
- Create ignored artifacts under: `.local-tests/shkeeper-production-line/`

**Interfaces:**
- Consumes: all production-line backend/frontend work.
- Produces: six-locale completeness, full regression evidence, an untouched baseline process, a ported candidate process, motion/pixel comparisons, and a user-review URL.

- [ ] **Step 1: Apply all actual new translations through the mandated script**

Run `bun run i18n:sync`, read the report, scan Task 3/4 literal `t(...)`
calls, and populate `scripts/add-missing-keys.mjs` for all six locales. Include
only actual callsite keys; preserve placeholders and use compact translations
for buttons, badges, errors, and mobile labels.

Run:

```powershell
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

Delete temporary discovery/apply scripts after the locale files are updated.
Never edit locale JSON directly.

- [ ] **Step 2: Run fresh complete automated verification**

From repository root:

```powershell
go test ./... -count=1
git diff --check
git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles
```

From `web/default`:

```powershell
bun test
bun run i18n:sync
bun run typecheck
bun run build
```

Run scoped oxlint/oxfmt on every changed TS/TSX/JSON file. Run full lint/format
checks and record unrelated baseline failures without mass-changing unrelated
files. Any finding in a changed file must be fixed.

- [ ] **Step 3: Create an untouched production-baseline worktree and build both binaries**

From the main repository root, verify `.worktrees/` is ignored, then create:

```powershell
git worktree add 'D:\newapi-710-yuapi\.worktrees\shkeeper-production-baseline' --detach 27023295d
```

Build the baseline frontend/binary there and the candidate in the feature
worktree. Use separate ignored directories and isolated SQLite databases.
Neither process may read the production database or shared local preview data.

Start baseline on `127.0.0.1:13036` and candidate on
`127.0.0.1:13037` using `Start-Process -WindowStyle Hidden`. If either port is
occupied by a process not owned by this task, select another free port and
record it instead of stopping that process.

- [ ] **Step 4: Seed equivalent visual state and a local v2.5.32 provider fixture**

Create synthetic administrator accounts, identical theme/system settings, and
the same wallet values in both isolated databases. Candidate additionally gets
packages `10 -> 66`, `50 -> 330`, `100 -> 660`, `200 -> 1320`, all three
networks, and local-only write-only test credentials.

The provider fixture binds loopback only and returns exact v2.5.32 quote,
create, lookup, and signed callback shapes. It never contacts a chain or moves
funds.

- [ ] **Step 5: Compare global UI and motion before opening payment UI**

Use the bundled Playwright module with installed Edge. At `1440x900`,
`1024x768`, `390x844`, and `360x740`, in light and dark themes, capture baseline
and candidate for sign-in, docs, wallet shell, home, and system settings.

For each pair record:

- top-level layout dimensions and overflow;
- renderer/fallback selection exposed by DOM state;
- Canvas/WebGL element count and bounding boxes;
- nontransparent/nonblank canvas pixel samples;
- two frame samples separated by animation frames, proving motion on hardware
  or equivalent static fallback on software;
- console errors, page errors, failed requests, and HTTP >= 400;
- screenshot contact sheets for human comparison.

The candidate must not add/remove/change global renderer elements or animation
behavior. A mismatch blocks completion and must be traced to feature-owned code;
immutable production files remain untouched.

- [ ] **Step 6: Verify payment and settings workflows on the candidate**

Exercise:

- fresh empty/null settings;
- add/remove/duplicate/last-package validation;
- write-only secret reload;
- exact all-network connection readiness;
- package/network keyboard selection;
- create body contains only `usdt_amount` and `crypto`;
- exact invoice, QR, address copy, partial zero credit, expired recovery, signed
  full callback, callback replay, overpayment cap;
- HTTP-200 business polling error retains invoice and retries same order;
- ordinary fractional amount cannot replace the valid integer value;
- close/reopen reset;
- mobile scrolling with all actions reachable;
- no marketing-side panel or non-production payment design.

Capture desktop/mobile light/dark screenshots and inspect them visually.

- [ ] **Step 7: Update verification documentation and commit**

Record source/base commit, candidate commits, commands/results, immutable diff
guard, process IDs/ports, screenshots, fixture boundary, remaining MySQL/
PostgreSQL and real-chain requirements, and cleanup prerequisites.

Run:

```powershell
git add web/default/src/i18n/static-keys.ts web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json docs/superpowers/handoffs/2026-09-26-shkeeper-production-line-local-verification.md
git commit -m "docs: verify SHKeeper on production UI line"
```

- [ ] **Step 8: Leave only the correct candidate running for user review**

Stop the temporary baseline process owned by this task. Keep the correct
production-line candidate running and report its complete URL and synthetic
credentials. Do not touch `13035` yet; Task 6 performs authorized cleanup only
after every Task 5 gate passes.

---

### Task 6: Remove the Wrong Baseline and Preserve the Correct Candidate

**Files/Resources:**
- Remove after verified success: `D:\newapi-710-yuapi\.worktrees\shkeeper-usdt`
- Delete after worktree removal: local branch `codex/shkeeper-usdt`
- Stop after exact-path verification: old preview PID listening on `13035`
- Remove after comparison: `D:\newapi-710-yuapi\.worktrees\shkeeper-production-baseline`
- Preserve: `D:\newapi-710-yuapi\.worktrees\shkeeper-production-line`
- Preserve: branch `codex/shkeeper-usdt-production-line`
- Preserve: the new candidate process and isolated candidate database

**Interfaces:**
- Consumes: Task 5 green verification, correct candidate URL, exact process
  ownership, and explicit user authorization in the conversation.
- Produces: one correct local candidate, no wrong-baseline worktree/branch, no
  stale comparison process, and no effect on unrelated worktrees/services.

- [ ] **Step 1: Verify cleanup preconditions**

Before deleting anything, confirm:

```powershell
git -C 'D:\newapi-710-yuapi\.worktrees\shkeeper-production-line' status --short --branch
git -C 'D:\newapi-710-yuapi\.worktrees\shkeeper-production-line' log -1 --oneline
git -C 'D:\newapi-710-yuapi\.worktrees\shkeeper-production-line' diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles
Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:13037/api/status'
git worktree list --porcelain
```

Require: tracked candidate tree clean; automated/visual evidence committed;
immutable diff empty; candidate healthy; old and temporary paths resolve under
the exact repository `.worktrees` directory.

- [ ] **Step 2: Resolve exact process ownership**

List listeners for old preview, temporary baseline, and correct candidate.
Resolve every PID through `Win32_Process`. The old preview may be stopped only
when its executable path is inside:

```text
D:\newapi-710-yuapi\.worktrees\shkeeper-usdt\
```

The temporary baseline may be stopped only when its executable path is inside:

```text
D:\newapi-710-yuapi\.worktrees\shkeeper-production-baseline\
```

Never stop processes by port alone. Never stop the correct candidate or any
Docker/Sub2API/production process.

- [ ] **Step 3: Stop only the wrong preview and temporary baseline**

Use `Stop-Process -Id $verifiedPid` for each exact owned process after assigning
that variable from the verified `Win32_Process` result. Verify the
old and temporary ports are no longer listening and the correct candidate still
returns healthy status.

- [ ] **Step 4: Remove the wrong worktree and branch**

The user explicitly authorized deletion after successful completion. From
`D:\newapi-710-yuapi`, first print the wrong worktree's full status and commit
list into the new verification artifact, then run:

```powershell
git worktree remove --force 'D:\newapi-710-yuapi\.worktrees\shkeeper-usdt'
git worktree prune
git branch -D codex/shkeeper-usdt
```

The force removal is limited to the explicitly authorized wrong worktree and
its ignored test/SDD artifacts. Do not delete any sibling worktree or branch.

- [ ] **Step 5: Remove the detached comparison worktree**

After its process is stopped and artifacts needed for the committed report have
been copied into the candidate's ignored verification directory:

```powershell
git worktree remove --force 'D:\newapi-710-yuapi\.worktrees\shkeeper-production-baseline'
git worktree prune
```

- [ ] **Step 6: Verify cleanup and handoff**

Run:

```powershell
git worktree list --porcelain
git branch --list codex/shkeeper-usdt codex/shkeeper-usdt-production-line
Get-NetTCPConnection -State Listen | Where-Object { $_.LocalPort -in @(13035, 13036, 13037) }
Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:13037/api/status'
```

Expected:

- wrong worktree absent;
- wrong branch absent;
- temporary baseline absent;
- only `codex/shkeeper-usdt-production-line` and its worktree remain for this
  feature;
- only the correct candidate port listens;
- candidate health succeeds;
- production, Docker, Sub2API, Caddy, databases, and unrelated worktrees remain
  untouched.

Report the correct URL, credentials, final commits, verification results,
deleted paths/branch, and remaining real-chain/MySQL/PostgreSQL production gate.

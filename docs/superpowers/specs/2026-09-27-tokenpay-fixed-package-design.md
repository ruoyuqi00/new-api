# TokenPay Fixed-Package Recharge Integration

Date: 2026-09-27 (Asia/Shanghai)

Status: Product rules approved in chat; written design awaiting review.

YuAPI baseline: `f4cf272e0f3c083d9f0161fa962b6db59e13fd59` on
`codex/image-api-resolution-routing-20260907-local`.

Provider reference: `LightCountry/TokenPay`, release `v1.2.0`, commit
`d27d7637306599a315b53a50a122c0e071a8586d`.
Do not build from a moving branch or deploy an unverified latest image.

## Goal and Boundaries

Integrate a separately deployed, self-hosted TokenPay service with YuAPI. A
user chooses a fixed recharge package and a network, receives a new address
for that order, pays the fixed USDT amount, and obtains the package balance
after verified payment. Two orders can have the same amount on the same chain
without modifying either amount.

The service is not a full blockchain node and does not require a chain-sized
disk. Software is open source; public API quotas, hosting, and on-chain
transaction fees are not guaranteed to be free.

No production deployment, hot cutover, live wallet transfer, or modification
of another project is authorized by this implementation stage. Preserve
ordinary EPay, Stripe, other payment providers, and the existing SHKeeper
settings and orders. The new provider is disabled until explicitly configured.

## Confirmed Product Rules

- Supported networks are BSC/BEP20, TRON/TRC20, and Polygon, for USDT only.
- Users choose a network; the chain is not randomly changed between orders.
- Each order receives its own address on its selected network.
- Administrators configure positive whole-number USDT packages and their
  positive decimal site-balance entitlements, for example 10 USDT to 66 balance.
  There is no global exchange-rate change to existing YuAPI payment methods.
- Server-side settings determine the package entitlement. Client-supplied
  balance, rate, wallet address, or provider status is never authoritative.
- Freeze requested USDT, balance, quota entitlement, and network at creation.
  Later changes to packages or quota conversion cannot rewrite existing orders.
- Never add a payment tail or silently accept a provider quote with a different
  payable amount. Refuse to expose the payment page on a quote mismatch.
- Short payments grant no entitlement. A verified successful exact-amount
  payment grants the frozen entitlement once. Unexpected or excess payments
  remain available for operator review, not automatic extra balance.
- Keep a transaction-hash recovery field. A user claim cannot directly mark an
  order paid. It is queued for authorized operator review and must pass the
  same settlement and transaction-reuse checks.

## Provider Selection Evidence

TokenPay's `HomeController.CreateAddress` stores a generated wallet against
`OrderUserKey`. `UseDynamicAddress=true` and a unique order-derived key provide
a new address for every order. Use an order-and-network-specific key, never a
stable user ID or email. Dynamic-address mode calculates the requested amount
without the static-address collision increment.

The current `OrderCheckEVMERC20Service` uses an Etherscan-compatible
`account/tokentx` API, not JSON-RPC. Etherscan's supported-chain list does not
offer BSC transfer queries on the free tier. Merely replacing `ApiHost` with a
JSON-RPC URL will not work.

Read-only checks on 2026-09-27 found that the public BSC and Polygon RPCs could
return USDT `Transfer` events through `eth_getLogs`. This demonstrates a
possible free starting path, not uptime, quota, backfill, or production-payment
acceptance. A TokenPay EVM monitoring adaptation is required before claiming
that all three networks are usable without a paid indexer.

GMPay Edge is not the chosen provider for this design: its amount allocation
adds collision offsets, and its amount reuse quarantine conflicts with a
single-address fixed-amount package flow. Do not deploy the previously
discussed GMPay Edge or SHKeeper services as part of this implementation.

## Ownership and Funds

TokenPay generates random hot-wallet keys; it does not derive child addresses
from the user's existing main wallet. Its SQLite wallet database contains the
generated private keys. Losing the database or its backups may make funds
unrecoverable; exposure may allow theft.

Store this service in its own repository, runtime directory, persistent volume,
and container. Do not vendor its wallet or scanner implementation into YuAPI.
YuAPI stores no wallet private keys or mnemonic. Keep the TokenPay database
private to the service, with restrictive filesystem permissions and encrypted
backups tested for recovery. No keys belong in Git, HTTP responses, screenshots,
frontend code, or normal logs.

Funds initially remain in the generated addresses. Moving USDT to a main wallet
needs BNB gas on BSC, POL gas on Polygon, and TRX/energy resources on TRON.
TokenPay v1.2.0 includes TRON collection but not automatic EVM collection.
Keep collection disabled initially. EVM collection is manual or a separately
approved future feature; do not automatically fund or sweep these addresses.
Do not enable the upstream collection path that transmits a fee-wallet private
key over Telegram without a separate security review.

## Configuration and User Flow

Add an independent TokenPay section under YuAPI payment settings containing an
enable toggle, service base URL, write-only API signing secret, network toggles,
and a fixed-package editor. Do not reuse or change the global EPay price.
Leaving a secret field blank preserves its saved value; expose only a
configured-status flag. Explicit clearing is a distinct action.

Configure TokenPay with dynamic addresses, `BaseCurrency=USD`, fixed USDT rate
1, zero rate adjustment, and no approximate/custom-amount matching. Use
confirmed TRON transfers and appropriate EVM confirmation counts. Package
balance conversion occurs only in YuAPI. These provider-side settings are
documented separately from YuAPI's settings.

YuAPI creates a local pending order before contacting the provider. It calls
TokenPay's `POST /CreateOrder` using the persisted trade number as `OutOrderId`
and an order/network-specific `OrderUserKey`. Map the selected chain to the
configured TokenPay currency identifier. Authenticate with its HMAC-SHA256
mode; both provider and client must use the same configured algorithm.

Validate successful response fields: provider order ID, external order ID,
network/currency, original amount, chain payable amount, receive address, and
payment URL. Only expose a HTTPS payment URL on the configured provider origin.
Freeze validated provider identifiers and address before returning the link.
Users open the hosted payment page, not the provider administrator interface.
Use current YuCore UI components without changing global styles or effects.

A creation timeout has unknown outcome. Retain the same local order identity
and reconcile it; do not immediately create another external order. TokenPay
can recreate an expired external order, so retries must not blindly replace a
persisted invoice/address. Early callbacks before quote validation cannot
settle the order.

## Settlement and Recovery

Use dedicated TokenPay routes and provider identifiers; do not reinterpret an
EPay or SHKeeper order as a TokenPay order. Add an additive provider-order
table with exact decimal snapshots and a transaction ownership ledger. All
YuAPI schemas and transactions support SQLite, MySQL, and PostgreSQL.

Verify the complete callback's configured HMAC signature in constant time,
retaining monetary text exactly as received. Do not select the signature
algorithm solely from untrusted callback metadata. After authentication,
verify provider identity, order/user ownership, currency/network, amount,
receive address, transaction hash, and successful paid status against the
frozen invoice. Provider-side signed query is the recovery path where supported;
never substitute a browser redirect, screenshot, or claimed hash for payment.

Pending, expired, malformed, mismatched, short, or unverified events credit zero.
Treat unexpected successful amounts as operator-review cases. Late payment
uses provider-verified operator recovery and the same frozen entitlement.

The upstream provider does not offer a user transaction-hash rescan API. YuAPI
exposes an authenticated owner-scoped hash-submission route with bounded input
and attempts. Submission only creates a support/review state and returns no
payment success. The operator independently checks the correct chain, USDT
contract, recipient address, amount, confirmation, and global transaction
uniqueness in a block explorer. For an expired provider order, the operator
uses TokenPay v1.2.0's existing administrative supplement flow to trigger its
normal callback. YuAPI still validates the resulting signed callback and frozen
invoice before credit. The provider's admin supplement does not itself verify
the chain transaction, so this human check is mandatory; it is not described
as automatic recovery.

Order completion, ledger transaction ownership, user quota increment, and
applicable affiliate reward are one database transaction. Duplicate callbacks
and callback/recovery races cannot add balance twice. A transaction already
owned by another local order cannot be reused. Generic administrator completion
must not bypass the TokenPay settlement ledger.

Only acknowledge a payment callback with plain text `ok` after durable local
processing succeeds. Return failure on database or verification errors so the
provider can retry. Observe callback and provider errors without logging
secrets. Log normal top-ups through the established recharge audit flow, not
as AI consumption.

## Free-RPC Provider Adaptation

Implement the EVM monitoring change in the independent TokenPay checkout, using
its existing Nethereum dependencies and the stable release as the baseline.
Limit initial scope to USDT on BSC and Polygon; preserve existing indexer support
as an explicit alternative rather than pretending it is interchangeable.

Validate chain ID, configured contract, event sender/destination, token decimals,
amount, block time, successful receipt, and confirmations. Persist a bounded
scan cursor and recover from delayed indexing and restarts; failed/incomplete
RPC requests cannot advance the cursor or mark an order paid. Discard removed
or unconfirmed logs. Bound ranges, requests, response sizes, and retries to
avoid unbounded backfill and memory pressure. Do not scan or save entire chains.

Local tests and fake provider fixtures precede any real-money acceptance. Public
RPC failure remains a pending payment, not a guessed successful recharge.

## Verification and Handoff

- Test exact request signatures and callback canonicalization against native
  TokenPay fixtures, including monetary strings with trailing zeros.
- Test two simultaneous same-amount/same-chain orders obtaining distinct
  addresses with identical fixed payable amounts.
- Test invalid quotes, provider timeouts, early callbacks, changed package
  settings, invalid signatures, short payments, and unsupported networks.
- Test duplicate callbacks, recovery races, transaction reuse across orders,
  rollback on ledger/quota errors, decimal package history, and quota ceilings.
- Protect existing providers with regression tests; no global price changes.
- Test RPC contracts, wrong chain/contract/address, decimals, confirmation
  boundaries, removed logs, delayed results, bounded paging, and restart recovery.
- Verify the existing branded configuration/package flow on desktop and mobile,
  dark and light themes; keep unrelated rendering effects unchanged.
- Document separate service setup, backup recovery, three-chain configuration,
  write-only credentials, package setup, manual recovery, and collection limits.
- Do not declare live payments working from mocks. Actual three-chain small-value
  payment and callback acceptance require separately configured provider keys
  and explicit approval for the transfers before public enablement.

## Implementation Stage Limit

First deliver and verify the independent provider adaptations and YuAPI
compatibility locally, with safe test configuration and no real wallet keys.
Expose the local branded UI for user inspection. Deployment, production
configuration, wallet funding, collection, and hot cutover require separate
approval. No other containers or services are started for this stage.

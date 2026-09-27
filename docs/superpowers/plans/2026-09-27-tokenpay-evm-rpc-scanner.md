# TokenPay EVM RPC USDT Scanner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the independent TokenPay v1.2.0 service confirm fixed-amount USDT payments on BSC and Polygon through configurable JSON-RPC without a paid Etherscan transfer API.

**Architecture:** Preserve TokenPay's existing create-order, generated-address, callback, and SQLite behavior. Add a narrowly scoped EVM `eth_getLogs` reader for the selected USDT contracts and a durable per-chain/contract cursor; pass verified transfers into the existing paid-order and notification pipeline. Never treat a failed scan or provider timeout as payment.

**Tech Stack:** TokenPay v1.2.0 at `d27d7637306599a315b53a50a122c0e071a8586d`, .NET 8, C#, FreeSql/SQLite, existing Nethereum and Flurl dependencies, JSON-RPC.

**Spec:** `D:/newapi-710-yuapi/tmp/worktrees/image-api-resolution-routing-20260907/docs/superpowers/specs/2026-09-27-tokenpay-fixed-package-design.md`

## Global Constraints

- Work in an independent checkout at `D:/tokenpay-yuapi`; do not place TokenPay source or generated wallet databases inside YuAPI.
- Pin v1.2.0 commit `d27d7637306599a315b53a50a122c0e071a8586d`. Preserve upstream GPLv3 files and notices.
- Add JSON-RPC only for USDT on BSC (chain ID 56) and Polygon (chain ID 137). TRON remains on the upstream path.
- BSC USDT contract is `0x55d398326f99059ff775485246999027b3197955` (18 decimals); Polygon USDT contract is `0xc2132d05d31c914a87c6611c10748aeb04b58e8f` (6 decimals). Recheck these against trusted sources before live funds.
- Dynamic address is per order via a unique `OrderUserKey`; `BaseCurrency=USD`, fixed USDT rate 1, exact matching, and collection disabled until separately approved.
- No production deployment, hot cutover, real wallet keys, real transfers, or other-container startup in this stage.
- Public RPC availability is only a development hypothesis. Do not call the network production-ready without a sustained bounded scan and a small confirmed payment on each chain.

## File Map

| Unit | Target files in the independent repository | Responsibility |
| --- | --- | --- |
| RPC settings | `src/TokenPay/Models/EthModel/EVMChain.cs`, `src/TokenPay/EVMChains.Example.json`, docs | Opt-in JSON-RPC URL and scan limits per chain |
| Scanner | `src/TokenPay/BgServices/OrderCheckEVMERC20Service.cs`, new `src/TokenPay/Helper/EVMTransferLogReader.cs` | Bounded block/log reads and confirmation validation |
| Cursor | new `src/TokenPay/Domains/EVMScanCursor.cs`, `src/TokenPay/Program.cs` | Durable per-chain/contract scan progress |
| Tests | new `tests/TokenPay.Tests/*` or upstream test project's existing layout | Deterministic JSON-RPC fixtures and restart/duplicate regressions |

## Task 1: Isolated Provider Checkout and Baseline

**Files:** Separate checkout at `D:/tokenpay-yuapi`; no YuAPI files.

- [ ] **Step 1:** Confirm the target directory does not exist and clone only `LightCountry/TokenPay` into it. Check out release commit `d27d7637306599a315b53a50a122c0e071a8586d` on a `codex/evm-rpc-usdt` branch. Do not use an unverified release archive or `latest` image.
- [ ] **Step 2:** Run `dotnet restore` and `dotnet test` (or the repository's actual test project command). Record baseline failures before changing code.
- [ ] **Step 3:** Verify from the pinned source that `HomeController.CreateAddress` creates a distinct EVM address for each unique order key, and that TRON and EVM addresses are generated separately. Add a deterministic repository-level test if the upstream test layout supports controller fixtures.

Expected address invariant:

```csharp
Assert.NotEqual(firstOrder.ToAddress, secondOrder.ToAddress);
Assert.Equal(firstOrder.Amount, secondOrder.Amount);
```

## Task 2: Bounded EVM Transfer Reader

**Files:** Create `src/TokenPay/Helper/EVMTransferLogReader.cs`; add its focused tests. Modify `EVMChain.cs` with an opt-in `RpcUrl` and maximum scan range.

**Interfaces:** `ReadConfirmedTransfersAsync(EVMChain chain, EVMErc20 token, long fromBlock, long toBlock, CancellationToken cancellationToken)` returns decoded transaction hash, log index, from/to, exact token units, block number, block timestamp, and receipt success; errors are explicit and never an empty-success result.

- [ ] **Step 1:** Add mock-HTTP tests for `eth_chainId`, `eth_blockNumber`, `eth_getLogs`, `eth_getTransactionReceipt`, and `eth_getBlockByNumber`; assert wrong chain, contract, malformed topic, failed receipt, removed log, wrong destination, and unconfirmed block are rejected.
- [ ] **Step 2:** Run focused .NET tests and confirm they fail before implementation.
- [ ] **Step 3:** Implement topic decoding with the ERC-20 `Transfer` topic, exact integer units (`BigInteger` or library equivalent), canonical 20-byte addresses, and token decimals. Bound HTTP timeout, logs per response, block range, and concurrent requests. Split rejected large ranges; do not advance a cursor on partial failure.
- [ ] **Step 4:** Run tests including high-volume bounded-range fixtures; commit as `feat: read confirmed EVM USDT transfers from RPC`.

Example JSON-RPC fixture:

```json
{"jsonrpc":"2.0","id":1,"method":"eth_getLogs","params":[{"fromBlock":"0x10","toBlock":"0x10","address":"0x55d398326f99059ff775485246999027b3197955","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"]}]}
```

## Task 3: Durable Cursor and Existing Order Pipeline

**Files:** Create `src/TokenPay/Domains/EVMScanCursor.cs`; modify `src/TokenPay/BgServices/OrderCheckEVMERC20Service.cs` and `src/TokenPay/Program.cs`; add focused tests.

**Interfaces:** One cursor keyed by network chain ID and contract address. Only verified, confirmed transfers enter the existing order status update and callback queue. Existing Etherscan-compatible mode stays available when `RpcUrl` is absent.

- [ ] **Step 1:** Add SQLite fixture tests: two equal-amount different-address orders credit their matching transfers; duplicate log and duplicate transaction hash do not trigger two callbacks; restart resumes before the first unprocessed block; delayed RPC or malformed response does not move the cursor; a reorg/removed log does not mark paid.
- [ ] **Step 2:** Run focused tests red. Add GORM-equivalent FreeSql entity/migration registration without dropping legacy tables.
- [ ] **Step 3:** At each scheduled run, read a bounded range from the durable cursor to the confirmation-safe head, decode logs, match against pending orders by normalized receiving address/currency/exact amount and order time, persist result, then advance cursor after the whole range succeeds. Preserve the upstream queue for notifications. Avoid scanning the entire historical chain: bootstrap a new cursor from the earliest pending order's bounded lookback and refuse an unsafe gap instead of silently skipping.
- [ ] **Step 4:** Add restart and long-gap tests. Run the complete provider test suite and `dotnet publish -c Release`; commit as `feat: settle BSC and Polygon USDT through bounded RPC scan`.

## Task 4: Local Contract and Security Acceptance

**Files:** Update independent repository deployment docs and sample config only. No secret material.

- [ ] **Step 1:** Configure fake BSC and Polygon RPC endpoints and fake order callback. Create two equal-amount orders with separate unique keys, emit distinct verified logs, and assert two correct callbacks with no cross-credit.
- [ ] **Step 2:** Test public RPC `eth_chainId`, current-height `eth_getLogs`, and bounded historical queries as read-only diagnostics; record observed rate/latency without claiming uptime guarantees.
- [ ] **Step 3:** Verify no private key/seed appears in logs, API responses, or Git-tracked config. Confirm the upstream database backup guidance and collection-disabled default.
- [ ] **Step 4:** Provide the exact pinned commit, a separate Docker/systemd configuration, required public RPC or operator-supplied RPC URLs, and recovery instructions. Keep BSC/Polygon disabled in YuAPI until a separate approved small real transfer proves each chain end to end.

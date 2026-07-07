# Transaction Verification Matrix

Live sign-off sheet for typical Launchpad Orchestrator routes × transaction variants.  
**Only rows that quote successfully and produce a signable transaction.** Fill in **Signature** and **Solscan** after each on-chain run.

**中文：** [tx-verification-matrix.zh-CN.md](tx-verification-matrix.zh-CN.md)

---

## Variant keys (`quote.builds`)

| UI toggles | Build key | Fee payer | Notes |
|------------|-----------|-----------|-------|
| default | `selfFunded` | user | User pays gas + priority fee |
| MEV only | `selfFundedMev` | user | + Jito tip (needs `[jito] enabled`) |
| Sponsored only | `sponsoredSwap` | sponsor | User repays from SOL/WSOL proceeds |
| Both | `sponsoredSwapMev` | sponsor | Sponsored + Jito tip |

**SOL/WSOL full unwrap** appends settlement suffix: `_close` or `_unwrapAll`  
(e.g. `selfFunded_close`, `sponsoredSwap_unwrapAll`).

---

## Column legend

| Column | Meaning |
|--------|---------|
| **#** | Row ID — reference in notes |
| **Scenario** | What you are testing |
| **Pay / Receive** | Mint + settlement chip (`native_sol` / `wsol_spl` / `spl`) |
| **Route** | Token flow path (e.g. `SOL -> USDC -> X`) |
| **Variant** | Which build key to sign |
| **Expect** | What should appear in the tx inspector |
| **Status** | `—` / `ok` / `fail` |
| **Signature** | On-chain tx id |
| **Solscan** | `https://solscan.io/tx/<signature>` |

---

## A — SOL/WSOL settlement

No compute-budget / priority-fee instructions. No MEV variant rows (this route does not emit signable MEV builds).

| # | Scenario | Pay | Receive | Route | Variant (build key) | Expect | Status | Signature | Solscan | Notes |
|---|----------|-----|---------|-------|---------------------|--------|--------|-----------|---------|-------|
| A1 | Partial unwrap | WSOL `wsol_spl` | Native SOL `native_sol` | `WSOL -> SOL` | `selfFunded` | SyncNative + UnwrapLamports(partial); no Compute Budget | ok | `5ijSe2qN1jFhVsmnKeRfvdE7KfnNhHW7f92qKTtgnZRJdKm7S9t23LA5U63xXgsyBWKD9H1SKx22n85LrBo1Ruvu` | [Solscan](https://solscan.io/tx/5ijSe2qN1jFhVsmnKeRfvdE7KfnNhHW7f92qKTtgnZRJdKm7S9t23LA5U63xXgsyBWKD9H1SKx22n85LrBo1Ruvu) | amount &lt; full WSOL ATA balance |
| A2 | Partial unwrap + sponsored | WSOL `wsol_spl` | Native SOL `native_sol` | `WSOL -> SOL` | `sponsoredSwap` | above + SystemTransfer repay → sponsor; sponsor co-sign | ok | `5Pygc3wZwLZCQ4RUH3TzBFmRe59LSSG7UWfxYbUB2mJr3hyDapMZUXyCL24d3FAoyYLhVs9Py9jEEXkJrMwUVmaL` | [Solscan](https://solscan.io/tx/5Pygc3wZwLZCQ4RUH3TzBFmRe59LSSG7UWfxYbUB2mJr3hyDapMZUXyCL24d3FAoyYLhVs9Py9jEEXkJrMwUVmaL) | needs `[sponsor] enabled` |
| A3 | Full unwrap — close ATA | WSOL `wsol_spl` | Native SOL `native_sol` | `WSOL -> SOL` | `selfFunded_close` | SyncNative + CloseAccount | ok | `5h7F9NrvWffJ6zLZGrxNGxiAswSH11HPKEqUc73vL4XWTifKNoAtybYA4prVtij7GC12y6YQoZztvPCaWEPZtpgX` | [Solscan](https://solscan.io/tx/5h7F9NrvWffJ6zLZGrxNGxiAswSH11HPKEqUc73vL4XWTifKNoAtybYA4prVtij7GC12y6YQoZztvPCaWEPZtpgX) | input amount = full WSOL balance |
| A4 | Full unwrap — UnwrapLamports all | WSOL `wsol_spl` | Native SOL `native_sol` | `WSOL -> SOL` | `selfFunded_unwrapAll` | SyncNative + UnwrapLamports(all); ATA kept | ok | `4RmnoFib51nX4F1vJRRUSaiXKKtWNk3T7u5oBpV8onRcWZH2SHr9ixjx1TExQfRY8M2fnH8PxfS7y9boZFfUyZpy` | [Solscan](https://solscan.io/tx/4RmnoFib51nX4F1vJRRUSaiXKKtWNk3T7u5oBpV8onRcWZH2SHr9ixjx1TExQfRY8M2fnH8PxfS7y9boZFfUyZpy) | input amount = full WSOL balance |
| A5 | Full unwrap + sponsored (close) | WSOL `wsol_spl` | Native SOL `native_sol` | `WSOL -> SOL` | `sponsoredSwap_close` | close + offline repay transfer | ok | `zHbGU3zH2fDig7z2roCXrbd4XjxLgkQLKstaV6jSZewhoBf2ThsAsaWkknG6qd7Vjmb2biVmN8XzktHyjEBcePW` | [Solscan](https://solscan.io/tx/zHbGU3zH2fDig7z2roCXrbd4XjxLgkQLKstaV6jSZewhoBf2ThsAsaWkknG6qd7Vjmb2biVmN8XzktHyjEBcePW) | |
| A6 | Full unwrap + sponsored (unwrapAll) | WSOL `wsol_spl` | Native SOL `native_sol` | `WSOL -> SOL` | `sponsoredSwap_unwrapAll` | unwrapAll + offline repay transfer | ok | `2oQrCSB3DNZCphJCTEEvP2KCXogD2yY66eoYafNbQsK64nmTaa7dPLjad8bH2TH3v81vbnQUyRYjpyhG8VEY8MjA` | [Solscan](https://solscan.io/tx/2oQrCSB3DNZCphJCTEEvP2KCXogD2yY66eoYafNbQsK64nmTaa7dPLjad8bH2TH3v81vbnQUyRYjpyhG8VEY8MjA) | |
| A7 | Wrap SOL → WSOL | Native SOL `native_sol` | WSOL `wsol_spl` | `SOL -> WSOL` | `selfFunded` | Create ATA + Transfer + SyncNative | ok | `DkoJCbqq5ca7Z3knkwrKxwxTec5gDn989V9LX5iNJPeBARBeF1GARbf8r66CUam5LPJyEid3xAyHkYvzigfp16M` | [Solscan](https://solscan.io/tx/DkoJCbqq5ca7Z3knkwrKxwxTec5gDn989V9LX5iNJPeBARBeF1GARbf8r66CUam5LPJyEid3xAyHkYvzigfp16M) | `selfFunded` only |

---

## B — Quote swap

Single-hop Raydium AMM v4 / CPMM via Jupiter discovery. Compute-budget ixs present (except when testing disabled tier).

| # | Scenario | Pay | Receive | Route | Variant | Expect | Status | Signature | Solscan | Notes |
|---|----------|-----|---------|-------|---------|--------|--------|-----------|---------|-------|
| B1 | SOL → USDC | Native SOL `native_sol` | USDC `spl` | `SOL -> USDC` | `selfFunded` | bridge swap + CU limit/price | ok | `2Nt6rmh6bRFH6Bnt192rbbqtLkSkGaoiiV2bnJBn9ExpjEPiDKyc23a9etvVsZrNzvzX2y5AnmTjxAWC54ALdx1X` | [Solscan](https://solscan.io/tx/2Nt6rmh6bRFH6Bnt192rbbqtLkSkGaoiiV2bnJBn9ExpjEPiDKyc23a9etvVsZrNzvzX2y5AnmTjxAWC54ALdx1X) | |
| B2 | SOL → USDC + MEV | Native SOL `native_sol` | USDC `spl` | `SOL -> USDC` | `selfFundedMev` | + Jito tip transfer | ok | `3a4hbPg6m9LPQtfBze1Un68VaMf3yzqEDrqnykQr6FQxpkYs2DQsSQMcdiRhXiqwiJkhdY2nAALgV1vYxytWiggQ` | [Solscan](https://solscan.io/tx/3a4hbPg6m9LPQtfBze1Un68VaMf3yzqEDrqnykQr6FQxpkYs2DQsSQMcdiRhXiqwiJkhdY2nAALgV1vYxytWiggQ) | |
| B3 | USDC → Native SOL | USDC `spl` | Native SOL `native_sol` | `USDC -> SOL` | `selfFunded` | swap + CloseAccount unwrap to lamports | ok | `M9neGAJiuwZnfmPDGccLZL7zbsX9cjhJCt7brwjPSFsKHKtKRtdttLu1FxTNfARqz51ZxL9GivFhgaAsyh1JQUE` | [Solscan](https://solscan.io/tx/M9neGAJiuwZnfmPDGccLZL7zbsX9cjhJCt7brwjPSFsKHKtKRtdttLu1FxTNfARqz51ZxL9GivFhgaAsyh1JQUE) | |
| B4 | USDC → WSOL SPL | USDC `spl` | WSOL `wsol_spl` | `USDC -> WSOL` | `selfFunded` | swap only; WSOL stays in ATA | ok | `2viHoRFxphwod4emYNuoQ2FrnSPTfUEGEMaMNHVCGBaa5PmcirMw35tymeqfzXnUZ37tyJnCVFqWG1V36FyQuPwe` | [Solscan](https://solscan.io/tx/2viHoRFxphwod4emYNuoQ2FrnSPTfUEGEMaMNHVCGBaa5PmcirMw35tymeqfzXnUZ37tyJnCVFqWG1V36FyQuPwe) | |
| B7 | USDC → WSOL SPL + sponsored | USDC `spl` | WSOL `wsol_spl` | `USDC -> WSOL` | `sponsoredSwap` | swap + SyncNative + UnwrapLamports(repay→gas treasury); WSOL ATA kept | ok | `SLB2XszyunfWzfvkXjpynKeBQYoU9rwh2LbJ1g63Gqa38sfqT1akiHzDYNzExGLoC2SM57wN6ARFLLYwFui5qAM` | [Solscan](https://solscan.io/tx/SLB2XszyunfWzfvkXjpynKeBQYoU9rwh2LbJ1g63Gqa38sfqT1akiHzDYNzExGLoC2SM57wN6ARFLLYwFui5qAM) | needs `[sponsor] enabled` |
| B5 | USDC → USDT | USDC `spl` | USDT `spl` | `USDC -> USDT` | `selfFunded` | stable ↔ stable | ok | `5U1FUK8e1Cp8qqBA88bYn3GCz6hHtdqtD3ebhymMaR1DJDCYwmUx2pY3AWhDDSiPytv71MDFUiVFZphs6T51tiyc` | [Solscan](https://solscan.io/tx/5U1FUK8e1Cp8qqBA88bYn3GCz6hHtdqtD3ebhymMaR1DJDCYwmUx2pY3AWhDDSiPytv71MDFUiVFZphs6T51tiyc) | |
| B6 | USDC → Native SOL + sponsored | USDC `spl` | Native SOL `native_sol` | `USDC -> SOL` | `sponsoredSwap` | swap + unwrap + sponsor fee payer | ok | `5XbbZGUtVu7jAo6QuSa46qirjiNVhYq9K5rXWCyywH8GKes5SEKwKaMfALobWzEc3SW79GjCxizodZzu538BnuAd` | [Solscan](https://solscan.io/tx/5XbbZGUtVu7jAo6QuSa46qirjiNVhYq9K5rXWCyywH8GKes5SEKwKaMfALobWzEc3SW79GjCxizodZzu538BnuAd) | receive side is native SOL |

---

## C — Launchpad buy

Pump.fun bonding curve (v1). Pool quote = **native SOL** unless token is USDC-pool (rare in demo).  
**Native SOL 1-hop buy** supports only `selfFunded` / `selfFundedMev`; user already holds SOL for gas — **no** `sponsoredSwap` row.

| # | Scenario | Pay | Receive | Route | Variant | Expect | Status | Signature | Solscan | Notes |
|---|----------|-----|---------|-------|---------|--------|--------|-----------|---------|-------|
| C1 | SOL buy (1-hop) | Native SOL `native_sol` | `<pump-mint>` | `SOL -> X` | `selfFunded` | Pump buy + platform fee + CU | — | | | SOL-pool token |
| C2 | SOL buy + MEV | Native SOL `native_sol` | `<pump-mint>` | `SOL -> X` | `selfFundedMev` | + Jito tip | — | | | |
| C4 | USDC buy SOL-pool token | USDC `spl` | `<pump-mint>` | `USDC -> SOL -> X` | `selfFunded` | bridge + Ifx patch → Pump buy | — | | | |
| C5 | USDC buy + sponsored | USDC `spl` | `<pump-mint>` | `USDC -> SOL -> X` | `sponsoredSwap` | repay from bridge SOL/WSOL output | — | | | |
| C6 | USDT buy SOL-pool token | USDT `spl` | `<pump-mint>` | `USDT -> SOL -> X` | `selfFunded` | same pattern as C4 | — | | | |

---

## D — Launchpad sell

| # | Scenario | Pay | Receive | Route | Variant | Expect | Status | Signature | Solscan | Notes |
|---|----------|-----|---------|-------|---------|--------|--------|-----------|---------|-------|
| D1 | Sell → Native SOL | `<pump-mint>` | Native SOL `native_sol` | `X -> SOL` | `selfFunded` | Pump sell + SOL platform fee (Ifx/SystemTransfer) + CU | — | | | SOL-pool token |
| D2 | Sell → Native SOL + MEV | `<pump-mint>` | Native SOL `native_sol` | `X -> SOL` | `selfFundedMev` | + Jito tip | — | | | |
| D3 | Sell → Native SOL + sponsored | `<pump-mint>` | Native SOL `native_sol` | `X -> SOL` | `sponsoredSwap` | sponsor fee payer; repay from SOL proceeds | — | | | |
| D4 | Sell → USDC | `<pump-mint>` | USDC `spl` | `X -> SOL -> USDC` | `selfFunded` | sell + bridge | — | | | pick token with SOL pool |
| D5 | Sell → USDC + sponsored | `<pump-mint>` | USDC `spl` | `X -> SOL -> USDC` | `sponsoredSwap` | repay from SOL stream after sell | — | | | |
| D6 | Sell → WSOL SPL | `<pump-mint>` | WSOL `wsol_spl` | `X -> WSOL` | `selfFunded` | output stays WSOL ATA (no close) | — | | | |

---

## E — Launchpad swap A→B

Same `Q_native` on both tokens (v1: two Pump legs via SOL).

| # | Scenario | Pay | Receive | Route | Variant | Expect | Status | Signature | Solscan | Notes |
|---|----------|-----|---------|-------|---------|--------|--------|-----------|---------|-------|
| E1 | A → B (same quote) | `<mint-A>` | `<mint-B>` | `A -> SOL -> B` | `selfFunded` | sell A + buy B + inter-hop fee + CU | — | | | both SOL-pool |
| E2 | A → B + MEV | `<mint-A>` | `<mint-B>` | `A -> SOL -> B` | `selfFundedMev` | + Jito tip | — | | | |
| E3 | A → B + sponsored | `<mint-A>` | `<mint-B>` | `A -> SOL -> B` | `sponsoredSwap` | repay from SOL after sell A | — | | | both SOL-pool |

---

## Recommended test order

1. **A1 → A7** — settlement
2. **C1 → C2** — native SOL 1-hop buy
3. **D1 → D3** — core sell variants
4. **B1, B3, B4, B6, B7** — quote swap (incl. WSOL receive-side sponsored)
5. **C4, D4** — non-native quote 2-hop
6. **E1 → E3** — A→B swap
7. **A3–A6** — full-balance unwrap modes

---

## Mint placeholders (fill before testing)

| Symbol | Mainnet mint | Your test value |
|--------|--------------|-----------------|
| WSOL | `So11111111111111111111111111111111111111112` | |
| USDC | `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v` | |
| USDT | `Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB` | |
| Pump token A | SOL-pool, ungraduated | |
| Pump token B | SOL-pool, ungraduated | |

---

## Changelog

| Date | Tester | Summary |
|------|--------|---------|
| | | |

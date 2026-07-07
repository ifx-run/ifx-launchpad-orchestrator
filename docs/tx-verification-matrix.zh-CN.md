# 链上交易验证矩阵

Launchpad Orchestrator **典型场景 × 交易变体** 的链上验证结果（Solscan 签收）。

**English:** [tx-verification-matrix.md](tx-verification-matrix.md)

---

## 变体键名（`quote.builds`）

| 页面开关 | Build key | 费用支付方 | 说明 |
|----------|-----------|------------|------|
| 默认 | `selfFunded` | 用户 | 用户自付 gas + 优先费 |
| 仅 MEV | `selfFundedMev` | 用户 | 额外 Jito tip（需 `[jito] enabled`） |
| 仅 Sponsored | `sponsoredSwap` | sponsor | 从 SOL/WSOL 产出偿还 gas |
| 两者都开 | `sponsoredSwapMev` | sponsor | Sponsored + Jito tip |

**WSOL 全额解包** 会在 key 后追加：`_close` 或 `_unwrapAll`  
例如 `selfFunded_close`、`sponsoredSwap_unwrapAll`。

---

## 列说明

| 列 | 含义 |
|----|------|
| **#** | 行编号，便于备注引用 |
| **场景** | 测试意图 |
| **支付 / 收到** | Mint + 结算芯片（`native_sol` / `wsol_spl` / `spl`） |
| **路由** | 代币流经路径（如 `SOL -> USDC -> X`） |
| **变体** | 应签名的 build key |
| **预期** | Inspector 中应看到的指令特征 |
| **状态** | `—` / `ok` / `fail` |
| **Signature** | 链上交易签名 |
| **Solscan** | `https://solscan.io/tx/<signature>` |

---

## A — SOL/WSOL 互转

**无** compute-budget / 优先费指令。本组无 MEV 变体行（该路由不产出可签名的 MEV build）。

| # | 场景 | 支付 | 收到 | 路由 | 变体 (build key) | 预期 | 状态 | Signature | Solscan | 备注 |
|---|------|------|------|------|------------------|------|------|-----------|---------|------|
| A1 | 部分解包 | WSOL `wsol_spl` | 原生 SOL `native_sol` | `WSOL -> SOL` | `selfFunded` | SyncNative + UnwrapLamports(部分)；无 Compute Budget | ok | `5ijSe2qN1jFhVsmnKeRfvdE7KfnNhHW7f92qKTtgnZRJdKm7S9t23LA5U63xXgsyBWKD9H1SKx22n85LrBo1Ruvu` | [Solscan](https://solscan.io/tx/5ijSe2qN1jFhVsmnKeRfvdE7KfnNhHW7f92qKTtgnZRJdKm7S9t23LA5U63xXgsyBWKD9H1SKx22n85LrBo1Ruvu) | 数量 &lt; WSOL ATA 全额 |
| A2 | 部分解包 + sponsored | WSOL `wsol_spl` | 原生 SOL `native_sol` | `WSOL -> SOL` | `sponsoredSwap` | 同上 + SystemTransfer 偿还；sponsor 联签 | ok | `5Pygc3wZwLZCQ4RUH3TzBFmRe59LSSG7UWfxYbUB2mJr3hyDapMZUXyCL24d3FAoyYLhVs9Py9jEEXkJrMwUVmaL` | [Solscan](https://solscan.io/tx/5Pygc3wZwLZCQ4RUH3TzBFmRe59LSSG7UWfxYbUB2mJr3hyDapMZUXyCL24d3FAoyYLhVs9Py9jEEXkJrMwUVmaL) | 需 `[sponsor] enabled` |
| A3 | 全额解包 — 关闭 ATA | WSOL `wsol_spl` | 原生 SOL `native_sol` | `WSOL -> SOL` | `selfFunded_close` | SyncNative + CloseAccount | ok | `5h7F9NrvWffJ6zLZGrxNGxiAswSH11HPKEqUc73vL4XWTifKNoAtybYA4prVtij7GC12y6YQoZztvPCaWEPZtpgX` | [Solscan](https://solscan.io/tx/5h7F9NrvWffJ6zLZGrxNGxiAswSH11HPKEqUc73vL4XWTifKNoAtybYA4prVtij7GC12y6YQoZztvPCaWEPZtpgX) | 输入量 = WSOL 全额 |
| A4 | 全额解包 — UnwrapLamports | WSOL `wsol_spl` | 原生 SOL `native_sol` | `WSOL -> SOL` | `selfFunded_unwrapAll` | SyncNative + UnwrapLamports(全部)；保留 ATA | ok | `4RmnoFib51nX4F1vJRRUSaiXKKtWNk3T7u5oBpV8onRcWZH2SHr9ixjx1TExQfRY8M2fnH8PxfS7y9boZFfUyZpy` | [Solscan](https://solscan.io/tx/4RmnoFib51nX4F1vJRRUSaiXKKtWNk3T7u5oBpV8onRcWZH2SHr9ixjx1TExQfRY8M2fnH8PxfS7y9boZFfUyZpy) | 输入量 = WSOL 全额 |
| A5 | 全额 + sponsored (close) | WSOL `wsol_spl` | 原生 SOL `native_sol` | `WSOL -> SOL` | `sponsoredSwap_close` | close + 离线偿还转账 | ok | `zHbGU3zH2fDig7z2roCXrbd4XjxLgkQLKstaV6jSZewhoBf2ThsAsaWkknG6qd7Vjmb2biVmN8XzktHyjEBcePW` | [Solscan](https://solscan.io/tx/zHbGU3zH2fDig7z2roCXrbd4XjxLgkQLKstaV6jSZewhoBf2ThsAsaWkknG6qd7Vjmb2biVmN8XzktHyjEBcePW) | |
| A6 | 全额 + sponsored (unwrapAll) | WSOL `wsol_spl` | 原生 SOL `native_sol` | `WSOL -> SOL` | `sponsoredSwap_unwrapAll` | unwrapAll + 离线偿还转账 | ok | `2oQrCSB3DNZCphJCTEEvP2KCXogD2yY66eoYafNbQsK64nmTaa7dPLjad8bH2TH3v81vbnQUyRYjpyhG8VEY8MjA` | [Solscan](https://solscan.io/tx/2oQrCSB3DNZCphJCTEEvP2KCXogD2yY66eoYafNbQsK64nmTaa7dPLjad8bH2TH3v81vbnQUyRYjpyhG8VEY8MjA) | |
| A7 | 打包 WSOL | 原生 SOL `native_sol` | WSOL `wsol_spl` | `SOL -> WSOL` | `selfFunded` | Create ATA + Transfer + SyncNative | ok | `DkoJCbqq5ca7Z3knkwrKxwxTec5gDn989V9LX5iNJPeBARBeF1GARbf8r66CUam5LPJyEid3xAyHkYvzigfp16M` | [Solscan](https://solscan.io/tx/DkoJCbqq5ca7Z3knkwrKxwxTec5gDn989V9LX5iNJPeBARBeF1GARbf8r66CUam5LPJyEid3xAyHkYvzigfp16M) | 仅 `selfFunded` 可签名 |

---

## B — Quote 互换

Jupiter 发现单跳池 + 本地 Raydium AMM v4 / CPMM 指令。通常含 compute-budget。

| # | 场景 | 支付 | 收到 | 路由 | 变体 | 预期 | 状态 | Signature | Solscan | 备注 |
|---|------|------|------|------|------|------|------|-----------|---------|------|
| B1 | SOL → USDC | 原生 SOL `native_sol` | USDC `spl` | `SOL -> USDC` | `selfFunded` | bridge swap + CU | ok | `2Nt6rmh6bRFH6Bnt192rbbqtLkSkGaoiiV2bnJBn9ExpjEPiDKyc23a9etvVsZrNzvzX2y5AnmTjxAWC54ALdx1X` | [Solscan](https://solscan.io/tx/2Nt6rmh6bRFH6Bnt192rbbqtLkSkGaoiiV2bnJBn9ExpjEPiDKyc23a9etvVsZrNzvzX2y5AnmTjxAWC54ALdx1X) | |
| B2 | SOL → USDC + MEV | 原生 SOL `native_sol` | USDC `spl` | `SOL -> USDC` | `selfFundedMev` | + Jito tip | ok | `3a4hbPg6m9LPQtfBze1Un68VaMf3yzqEDrqnykQr6FQxpkYs2DQsSQMcdiRhXiqwiJkhdY2nAALgV1vYxytWiggQ` | [Solscan](https://solscan.io/tx/3a4hbPg6m9LPQtfBze1Un68VaMf3yzqEDrqnykQr6FQxpkYs2DQsSQMcdiRhXiqwiJkhdY2nAALgV1vYxytWiggQ) | |
| B3 | USDC → 原生 SOL | USDC `spl` | 原生 SOL `native_sol` | `USDC -> SOL` | `selfFunded` | swap + CloseAccount 解包 | ok | `M9neGAJiuwZnfmPDGccLZL7zbsX9cjhJCt7brwjPSFsKHKtKRtdttLu1FxTNfARqz51ZxL9GivFhgaAsyh1JQUE` | [Solscan](https://solscan.io/tx/M9neGAJiuwZnfmPDGccLZL7zbsX9cjhJCt7brwjPSFsKHKtKRtdttLu1FxTNfARqz51ZxL9GivFhgaAsyh1JQUE) | |
| B4 | USDC → WSOL | USDC `spl` | WSOL `wsol_spl` | `USDC -> WSOL` | `selfFunded` | 仅 swap；保留 WSOL ATA | ok | `2viHoRFxphwod4emYNuoQ2FrnSPTfUEGEMaMNHVCGBaa5PmcirMw35tymeqfzXnUZ37tyJnCVFqWG1V36FyQuPwe` | [Solscan](https://solscan.io/tx/2viHoRFxphwod4emYNuoQ2FrnSPTfUEGEMaMNHVCGBaa5PmcirMw35tymeqfzXnUZ37tyJnCVFqWG1V36FyQuPwe) | |
| B7 | USDC → WSOL + sponsored | USDC `spl` | WSOL `wsol_spl` | `USDC -> WSOL` | `sponsoredSwap` | swap + SyncNative + UnwrapLamports(偿还→gas 账户)；保留 WSOL ATA | ok | `SLB2XszyunfWzfvkXjpynKeBQYoU9rwh2LbJ1g63Gqa38sfqT1akiHzDYNzExGLoC2SM57wN6ARFLLYwFui5qAM` | [Solscan](https://solscan.io/tx/SLB2XszyunfWzfvkXjpynKeBQYoU9rwh2LbJ1g63Gqa38sfqT1akiHzDYNzExGLoC2SM57wN6ARFLLYwFui5qAM) | 需 `[sponsor] enabled` |
| B5 | USDC → USDT | USDC `spl` | USDT `spl` | `USDC -> USDT` | `selfFunded` | 稳定币互换 | ok | `5U1FUK8e1Cp8qqBA88bYn3GCz6hHtdqtD3ebhymMaR1DJDCYwmUx2pY3AWhDDSiPytv71MDFUiVFZphs6T51tiyc` | [Solscan](https://solscan.io/tx/5U1FUK8e1Cp8qqBA88bYn3GCz6hHtdqtD3ebhymMaR1DJDCYwmUx2pY3AWhDDSiPytv71MDFUiVFZphs6T51tiyc) | |
| B6 | USDC → 原生 SOL + sponsored | USDC `spl` | 原生 SOL `native_sol` | `USDC -> SOL` | `sponsoredSwap` | swap + unwrap + sponsor 代付 | ok | `5XbbZGUtVu7jAo6QuSa46qirjiNVhYq9K5rXWCyywH8GKes5SEKwKaMfALobWzEc3SW79GjCxizodZzu538BnuAd` | [Solscan](https://solscan.io/tx/5XbbZGUtVu7jAo6QuSa46qirjiNVhYq9K5rXWCyywH8GKes5SEKwKaMfALobWzEc3SW79GjCxizodZzu538BnuAd) | 收到侧为原生 SOL |

---

## C — 内盘买入

Pump.fun bonding curve（v1）。演示环境多为 **SOL 池**。  
**原生 SOL 买单跳** 仅 `selfFunded` / `selfFundedMev`；用户已持有 SOL 支付 gas，**无** `sponsoredSwap` 行。

| # | 场景 | 支付 | 收到 | 路由 | 变体 | 预期 | 状态 | Signature | Solscan | 备注 |
|---|------|------|------|------|------|------|------|-----------|---------|------|
| C1 | SOL 买单跳 | 原生 SOL `native_sol` | `<pump-mint>` | `SOL -> X` | `selfFunded` | Pump buy + 平台费 + CU | ok | `2moymuNZwSCuqfZWVFhU2b3Jd7CAwuYgcpiDeaCDqmVDpVcyoTZoEJemmpKrCoZmdXKaMaB34tmvRWW7qJzZ7bVK` | [Solscan](https://solscan.io/tx/2moymuNZwSCuqfZWVFhU2b3Jd7CAwuYgcpiDeaCDqmVDpVcyoTZoEJemmpKrCoZmdXKaMaB34tmvRWW7qJzZ7bVK) | SOL 池代币 |
| C2 | SOL 买 + MEV | 原生 SOL `native_sol` | `<pump-mint>` | `SOL -> X` | `selfFundedMev` | + Jito tip | ok | `3UHjxe5LNQf7Ew2ZobVCpVA2foJeRqeFDE1SeTRvWtRrJBeVQctpEo6Y82XnFtNYDvkoYmpFueungddSv9BEi8h9` | [Solscan](https://solscan.io/tx/3UHjxe5LNQf7Ew2ZobVCpVA2foJeRqeFDE1SeTRvWtRrJBeVQctpEo6Y82XnFtNYDvkoYmpFueungddSv9BEi8h9) | |
| C4 | USDC 买 SOL 池币 | USDC `spl` | `<pump-mint>` | `USDC -> SOL -> X` | `selfFunded` | bridge + Ifx patch → Pump buy | ok | `55pyoWvh45PCWWNt6YtjDoNxrdJanS8pdhWFA62PhGcuhBc5iz9B4agq57JKuGVaNdwm6SkjPW6us9JHyFyZk4V5` | [Solscan](https://solscan.io/tx/55pyoWvh45PCWWNt6YtjDoNxrdJanS8pdhWFA62PhGcuhBc5iz9B4agq57JKuGVaNdwm6SkjPW6us9JHyFyZk4V5) | USDT 输入同此路径 |
| C5 | USDC 买 + sponsored | USDC `spl` | `<pump-mint>` | `USDC -> SOL -> X` | `sponsoredSwap` | 从 bridge 产出 SOL/WSOL 偿还 | ok | `2kffhdsCv5sZUBtqpj58nWtYcnnwcNYFuXwx3JbB1AeqQeSaa47gAPQ6UBU8Xf1ixTCy8LVEG7rNt6AEukhrUb6S` | [Solscan](https://solscan.io/tx/2kffhdsCv5sZUBtqpj58nWtYcnnwcNYFuXwx3JbB1AeqQeSaa47gAPQ6UBU8Xf1ixTCy8LVEG7rNt6AEukhrUb6S) | |

---

## D — 内盘卖出

SOL 池代币卖出换稳定币（`X -> SOL -> USDT`）。USDC 等同路径，不再单独列行。

| # | 场景 | 支付 | 收到 | 路由 | 变体 | 预期 | 状态 | Signature | Solscan | 备注 |
|---|------|------|------|------|------|------|------|-----------|---------|------|
| D1 | 卖出换 USDT | `<pump-mint>` | USDT `spl` | `X -> SOL -> USDT` | `selfFunded` | sell + bridge | ok | `4y4QxRNCi1W47b8Cz9S3Zk5ju5EcsCUiBH2FQE2PDng1S6W4mxrUCPY7TLwVxSYcR1ahWtBVDnEk1ozZRnbcepJV` | [Solscan](https://solscan.io/tx/4y4QxRNCi1W47b8Cz9S3Zk5ju5EcsCUiBH2FQE2PDng1S6W4mxrUCPY7TLwVxSYcR1ahWtBVDnEk1ozZRnbcepJV) | SOL 池 |
| D2 | 卖出换 USDT + MEV | `<pump-mint>` | USDT `spl` | `X -> SOL -> USDT` | `selfFundedMev` | + Jito tip | ok | `CQ84VMaN4m9uko7PfKE9pstkKi3b99Fz3AT5tt35RbEGKunpberh11Qn4waGy6mvfyBjyMBG14nAnxL8sm7sqxT` | [Solscan](https://solscan.io/tx/CQ84VMaN4m9uko7PfKE9pstkKi3b99Fz3AT5tt35RbEGKunpberh11Qn4waGy6mvfyBjyMBG14nAnxL8sm7sqxT) | |
| D3 | 卖出换 USDT + sponsored + MEV | `<pump-mint>` | USDT `spl` | `X -> SOL -> USDT` | `sponsoredSwapMev` | sponsor 代付 + Jito tip；SOL 产出偿还 | ok | `w44veLgotXZiyKFNNZpvHkb1U1dnwAgyoaQfad9JRr3muL46Wwpz8aeG5YXCCjZtq35AQPcZhGYURtGBk7EeaFS` | [Solscan](https://solscan.io/tx/w44veLgotXZiyKFNNZpvHkb1U1dnwAgyoaQfad9JRr3muL46Wwpz8aeG5YXCCjZtq35AQPcZhGYURtGBk7EeaFS) | 需 `[sponsor]` + `[jito]` |

---

## E — 内盘 A→B 互换

两代币同 `Q_native`（v1：双 Pump 腿，经 SOL 中转）。

| # | 场景 | 支付 | 收到 | 路由 | 变体 | 预期 | 状态 | Signature | Solscan | 备注 |
|---|------|------|------|------|------|------|------|-----------|---------|------|
| E1 | A → B | `<mint-A>` | `<mint-B>` | `A -> SOL -> B` | `selfFunded` | sell A + buy B + 跳间费 + CU | ok | `2aG87vD7LdYbU1g9tJ47tmW1h84Rj1btsDkqcZ6VxZeoLUCzX11biNSUtJFg9sJnVZQoxEXEAs3cQ6d39vqr354b` | [Solscan](https://solscan.io/tx/2aG87vD7LdYbU1g9tJ47tmW1h84Rj1btsDkqcZ6VxZeoLUCzX11biNSUtJFg9sJnVZQoxEXEAs3cQ6d39vqr354b) | 均为 SOL 池 |
| E2 | A → B + MEV | `<mint-A>` | `<mint-B>` | `A -> SOL -> B` | `selfFundedMev` | + Jito tip | ok | `2AKoV3GpBgeW2FNhMbo8SVGejVL9LjFraEJm1ZEqaRLmboBPeGtunV5TWKtAqvZ1R4gc1Etd1SZWT6vvAZGXHJBc` | [Solscan](https://solscan.io/tx/2AKoV3GpBgeW2FNhMbo8SVGejVL9LjFraEJm1ZEqaRLmboBPeGtunV5TWKtAqvZ1R4gc1Etd1SZWT6vvAZGXHJBc) | |
| E3 | A → B + sponsored | `<mint-A>` | `<mint-B>` | `A -> SOL -> B` | `sponsoredSwap` | sell A 产出 SOL 偿还 | ok | `26rrvEKgK8KJ4thhY5EPeSFtR1WcXz8yhaaTTptw9HAaYCjt5NBSLpYmaW9xFtjFtmZzMxTen27Q8h4ykAs2A2Zn` | [Solscan](https://solscan.io/tx/26rrvEKgK8KJ4thhY5EPeSFtR1WcXz8yhaaTTptw9HAaYCjt5NBSLpYmaW9xFtjFtmZzMxTen27Q8h4ykAs2A2Zn) | 均为 SOL 池 |

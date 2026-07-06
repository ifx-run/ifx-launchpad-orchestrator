# Pump.fun bonding curve（v2）

Program: `6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P`

## PDA

| 账户 | Seeds |
|------|-------|
| `global` | `["global"]` → `4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf` |
| `bonding_curve` | `["bonding-curve", base_mint]` |
| `creator_vault` | `["creator-vault", bonding_curve.creator]` |
| `user_volume_accumulator` | `["user-volume-accumulator", user]` |

## BondingCurve 布局（Anchor account，跳过 8B discriminator）

| 偏移 | 字段 | 类型 |
|------|------|------|
| 8 | `virtual_token_reserves` | u64 |
| 16 | `virtual_sol_reserves` | u64 |
| 24 | `real_token_reserves` | u64 |
| 32 | `real_sol_reserves` | u64 |
| 40 | `token_total_supply` | u64 |
| 48 | `complete` | bool |
| 49 | `creator` | pubkey |
| 81 | `is_mayhem_mode` | bool |
| 82 | `is_cashback_coin` | bool |
| 83 | `quote_mint` | pubkey（`Pubkey::default()` = legacy SOL 池） |

扩展字段（`is_mayhem_mode` 等）见 pump-sdk IDL；v1 quote 仅读至 `quote_mint`。

## 主指令（v1 目标）

| 方向 | 指令 | 说明 |
|------|------|------|
| buy | `buy_exact_quote_in_v2` | exact-in quote（SOL 为 native transfer + WSOL mint） |
| sell | `sell_v2` | exact-in base |

账户表见 [pump-public-docs BUY.md](https://github.com/pump-fun/pump-public-docs/blob/main/docs/instructions/BUY.md)。v1 orchestrator 自构 ix，不用 Jupiter / 包装合约。

## 链下 quote

复用 `@pump-fun/pump-sdk` 公式（Go 移植）：

- buy：`getBuyTokenAmountFromSolAmount` — 协议费 + creator 费后 constant-product
- sell：`getSellSolAmountFromTokenAmount`

平台 `service_fee` 在 Pump 协议费之外另行扣除（见 [docs/plan.zh-CN.md](../plan.zh-CN.md) §4.1）。

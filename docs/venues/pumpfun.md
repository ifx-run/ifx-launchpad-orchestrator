# Pump.fun bonding curve (v2)

Program: `6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P`

**中文:** [pumpfun.zh-CN.md](pumpfun.zh-CN.md)

## PDAs

| Account | Seeds |
|---------|-------|
| `global` | `["global"]` → `4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf` |
| `bonding_curve` | `["bonding-curve", base_mint]` |
| `creator_vault` | `["creator-vault", bonding_curve.creator]` |
| `user_volume_accumulator` | `["user-volume-accumulator", user]` |

## BondingCurve layout (Anchor account, skip 8-byte discriminator)

| Offset | Field | Type |
|--------|-------|------|
| 8 | `virtual_token_reserves` | u64 |
| 16 | `virtual_sol_reserves` | u64 |
| 24 | `real_token_reserves` | u64 |
| 32 | `real_sol_reserves` | u64 |
| 40 | `token_total_supply` | u64 |
| 48 | `complete` | bool |
| 49 | `creator` | pubkey |
| 81 | `is_mayhem_mode` | bool |
| 82 | `is_cashback_coin` | bool |
| 83 | `quote_mint` | pubkey (`Pubkey::default()` = legacy SOL pool) |

Extended fields (`is_mayhem_mode`, etc.) follow the pump-sdk IDL; v1 quote reads only through `quote_mint`.

## Primary instructions (v1 target)

| Direction | Instruction | Notes |
|-----------|-------------|-------|
| buy | `buy_exact_quote_in_v2` | exact-in quote (SOL uses native transfer + WSOL mint) |
| sell | `sell_v2` | exact-in base |

Account list: [pump-public-docs BUY.md](https://github.com/pump-fun/pump-public-docs/blob/main/docs/instructions/BUY.md). The v1 orchestrator builds instructions locally — no Jupiter or wrapper contracts.

## Off-chain quote

Ported from `@pump-fun/pump-sdk` formulas:

- buy: `getBuyTokenAmountFromSolAmount` — constant-product after protocol + creator fees
- sell: `getSellSolAmountFromTokenAmount`

Platform `service_fee` is charged on top of Pump protocol fees (see [docs/plan.md](../plan.md) §4.1).

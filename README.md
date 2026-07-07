# Launchpad Orchestrator

Go service + static frontend: orchestrate **Pump.fun**, **Raydium Launchpad**, and **Meteora DBC** pre-graduation launchpad trades with [Ifx](https://github.com/ifx-run/ifx). A single quote returns pricing, routing, and **four signable transaction variants** (selfFunded / MEV / sponsored / sponsored+MEV).

**On-chain verification:** [docs/tx-verification-matrix.md](docs/tx-verification-matrix.md) ([中文](docs/tx-verification-matrix.zh-CN.md))

**Quote bridging (SOL / USDC / USDT):** Jupiter is used only for **single-hop pool discovery**; swap instructions are built locally. v1 accepts only low-account flat pools (Raydium AMM v4 / CPMM).

> Demo software — not an audited production exchange.

**中文:** [README.zh-CN.md](README.zh-CN.md)

## Feature overview

| Capability | Status |
|------------|--------|
| Pump.fun launchpad buy / sell / A↔B swap | ✅ |
| Two-hop routes (e.g. USDT→WSOL→Pump) with Ifx on-chain patch | ✅ |
| Platform fee (SOL lamports / USDC / USDT SPL) | ✅ |
| v0 transactions + ALT, 1232B size gate | ✅ |
| Four build variants (`builds` + `capabilities`) | ✅ |
| Sponsored swap (sponsor pays gas + rent; user repays from SOL proceeds) | ✅ |
| Jito tip (`*Mev` variants) | ✅ |
| Web UI: Phantom connect, quote, simulate, send | ✅ |
| Raydium Launchpad / Meteora DBC full build | 🔜 planned |

## Quick start

```bash
cp config.example.toml config.toml
# Set solana.rpc_url (or export SOLANA_RPC_URL=https://...)

# For sponsored swap, also configure:
#   [sponsor] enabled = true
#   keypair_path to the sponsor key (matching pubkey), and fund the sponsor wallet with SOL

go run ./cmd/server
# Open http://127.0.0.1:8789 in your browser
```

Requires the [Phantom](https://phantom.app/) wallet. Supports:

- **SOL-pool** tokens: buy with a native SOL amount
- **USDC / USDT** buys into SOL-pool tokens: automatic Raydium bridge + Pump two-hop
- Toggle **MEV** / **Sponsored** on the page (hover disabled toggles for the reason)

### Tests

```bash
go test ./...
```

## Architecture (overview)

```text
POST /api/quote
  → route planning (pair → 1–3 legs)
  → snapshot (getMultipleAccounts ×1, processed)
  → off-chain quote + four-variant instruction assembly (zero extra RPC)
  → returns quote + builds[selfFunded|selfFundedMev|sponsoredSwap|sponsoredSwapMev]
```

**Ifx orchestration highlights:**

- Each tx Ifx segment starts with `IxReset()`
- Inter-hop amounts use `ifx_let` + `rawCpiPatch` (bridge output → pump buy, etc.)
- **Sponsored:** sponsor is fee payer; Ifx creates ATAs and measures rent via lamport deltas; repayment from SOL proceeds after WSOL unwrap (basic + priority + rent + tip + buffer)

## Configuration

`config.toml` is gitignored; copy from `config.example.toml`. You can also set `IFX_LAUNCHPAD_CONFIG=/path/to/config.toml`.

| Section | Description |
|---------|-------------|
| `[server]` | Listen address, `debug` logging |
| `[solana]` | RPC, ALT, `commitment` |
| `[snapshot]` | Launchpad account fetch uses **`processed`** |
| `[ifx]` | Mainnet program id, shared Frame |
| `[quotes]` | WSOL / USDC / USDT mints |
| `[venues.*]` | Three launchpad program ids |
| `[bridge.*]` | Flat pool allowlist, `max_swap_accounts`, Jupiter `low_account_dexes` |
| `[jupiter]` | Single-hop pool discovery (**does not** use Jupiter swap tx) |
| `[jito]` | `*Mev` variant tip account and amount |
| `[priority_fee.*]` | low / medium / high CU and microLamports |
| `[service_fee]` | Platform fee bps + SOL direct-receive pubkey |
| `[sponsor]` | Sponsored toggle, `pubkey` (fee payer), `repay_pubkey`, `keypair_path`, `repay_buffer_percent` |
| `[tx]` | `max_bytes` (default 1232) |

### Sponsored swap

```toml
[sponsor]
enabled = true
pubkey = "<fee-payer-pubkey>"      # must match keypair; tx fee payer + ATA rent
repay_pubkey = "<repay-treasury>"  # where user SOL repayment lands; may equal pubkey
keypair_path = "./keys/sponsor.json"
repay_buffer_percent = 10
```

Requirements:

1. Route must involve SOL / WSOL (`sponsoredSwap` repays from trade proceeds; pure stablecoin paths are unsupported)
2. Sponsor wallet holds enough SOL (covers rent + tx fee until user repayment)
3. Service loads the keypair at startup and **partially co-signs** sponsored variants

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | RPC connectivity |
| `GET` | `/api/config/public` | Public config (rpcUrl, default slippage, jito/sponsor flags, etc.) |
| `POST` | `/api/quote` | Quote + build (see below) |
| `POST` | `/api/tx/inspect` | Parse base64 transaction structure |
| `POST` | `/api/tx/simulate` | RPC simulation |
| `GET` | `/` | Static trading page |

### `POST /api/quote`

Request body (main fields):

```json
{
  "inputMint": "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
  "outputMint": "<pump-mint>",
  "inputAmount": "1",
  "slippageBps": 100,
  "userPubkey": "<user>",
  "priorityTier": "medium",
  "inputSettlement": "spl",
  "outputSettlement": "native_sol"
}
```

`inputSettlement` / `outputSettlement`: `native_sol` | `wsol_spl` | `spl` (affects WSOL wrap/unwrap and SOL repayment paths).

Response (excerpt):

```json
{
  "quote": { "outputAmount": "...", "minOutputAmount": "..." },
  "route": [ { "kind": "quote_bridge", "..." }, { "kind": "launchpad", "..." } ],
  "pairClass": "buy_launchpad",
  "build": { "variant": "selfFunded", "transaction": "<base64 v0>", "feePayer": "..." },
  "builds": {
    "selfFunded": { "transaction": "...", "repayEstimateLamports": 0 },
    "sponsoredSwap": { "transaction": "...", "repayEstimateLamports": 6746124 }
  },
  "capabilities": {
    "sponsoredSwap": { "supported": true },
    "sponsoredSwapMev": { "supported": false, "reason": "jito_disabled" }
  }
}
```

The frontend can switch variants **without re-quoting** — sign and send `builds[variant].transaction` directly.

Common `capabilities.reason` values: `no_sol_in_route`, `sponsor_disabled`, `sponsor_keypair`, `tx_too_large`, `sponsored_not_wired`, etc.

## Example routes

| User action | Path |
|-------------|------|
| Buy SOL-pool token with SOL | Pump buy (1 leg) |
| Buy SOL-pool token with USDT | USDT → Raydium → WSOL → unwrap → repay → Pump buy (2 legs + Ifx) |
| Sell SOL-pool token for USDT | Pump sell → WSOL → bridge → USDT (2 legs + Ifx) |
| Swap A→B (same quote) | Pump sell A → Pump buy B (2 legs + Ifx) |

## Documentation

- Development plan: [docs/plan.md](docs/plan.md)
- 开发计划（中文）：[docs/plan.zh-CN.md](docs/plan.zh-CN.md)
- Pump.fun spec: [docs/venues/pumpfun.md](docs/venues/pumpfun.md)
- Pump.fun 规格（中文）：[docs/venues/pumpfun.zh-CN.md](docs/venues/pumpfun.zh-CN.md)

## License & disclaimer

For demonstration only. You assume all on-chain trading risk; audit configuration, keys, and custody before mainnet use.

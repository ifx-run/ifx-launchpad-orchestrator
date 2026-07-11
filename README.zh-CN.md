# ifx-launchpad-orchestrator

Go 服务 + 静态前端：用 [Ifx](https://github.com/ifx-run/ifx) 编排 **Pump.fun**、**Raydium Launchpad**、**Meteora DBC** 内盘未毕业代币交易。一次询价返回报价、路由与 **四种可签名交易变体**（selfFunded / MEV / sponsored / sponsored+MEV）。

**链上验证结果：** [docs/tx-verification-matrix.zh-CN.md](docs/tx-verification-matrix.zh-CN.md)（[English](docs/tx-verification-matrix.md)）

**Quote 桥接（SOL / USDC / USDT）：** Jupiter 仅做**单跳池发现**；swap 指令本地构造。v1 接低账户 flat 池（Meteora DAMM v2、Raydium AMM v4 / CPMM）。

> 演示软件，非经审计的生产交易所。

See [README.md](README.md).

## 功能概览

| 能力 | 状态 |
|------|------|
| Pump.fun 内盘 buy / sell / A↔B swap | ✅ |
| 两跳路由（USDT→WSOL→Pump 等）Ifx 链上 patch | ✅ |
| 平台费（SOL lamports / USDC / USDT SPL） | ✅ |
| v0 交易 + ALT，1232B 体积门控 | ✅ |
| 四变体 build（`builds` + `capabilities`） | ✅ |
| Sponsored swap（sponsor 代付 gas + rent，用户从 SOL 产出偿还） | ✅ |
| Jito tip（`*Mev` 变体） | ✅ |
| 网页：Phantom 连接、询价、模拟、发送 | ✅ |
| Raydium Launchpad / Meteora DBC 完整 build | 🔜 规划中 |

## 快速开始

```bash
cp config.example.toml config.toml
# 填写 solana.rpc_url（或 export SOLANA_RPC_URL=https://...）

# 启用 sponsored 时另需：
#   [sponsor] enabled = true
#   keypair_path 指向 sponsor 密钥（与 pubkey 匹配），并给 sponsor 钱包充 SOL

go run ./cmd/server
# 浏览器打开 http://127.0.0.1:8789
```

需要 [Phantom](https://phantom.app/) 钱包。支持：

- **SOL 池**代币：直接填 SOL 数量买入
- **USDC / USDT** 买入 SOL 池代币：自动走 Raydium bridge + Pump 两跳
- 页面开关切换 **MEV** / **Sponsored**（灰掉时悬停显示不可用原因）

### 测试

```bash
go test ./...
```

## 架构（简图）

```text
POST /api/quote
  → 路由规划（pair → 1–3 leg）
  → snapshot（getMultipleAccounts ×1, processed）
  → 链下报价 + 四变体指令组装（零额外 RPC）
  → 返回 quote + builds[selfFunded|selfFundedMev|sponsoredSwap|sponsoredSwapMev]
```

**Ifx 编排要点：**

- 每笔 tx 内 Ifx 段以 `IxReset()` 开头
- 跳间金额用 `ifx_let` + `rawCpiPatch`（bridge 产出 → pump buy 等）
- **Sponsored：** sponsor 作 fee payer；Ifx 内 sponsor 创建 ATA 并按 lamports 差值实测 rent；从 WSOL unwrap 后的 SOL 产出偿还（basic + priority + rent + tip + buffer）

## 配置

`config.toml` 已 gitignore，从 `config.example.toml` 复制。也可用环境变量 `IFX_LAUNCHPAD_CONFIG=/path/to/config.toml`。

| 段 | 说明 |
|----|------|
| `[server]` | 监听地址、`debug` 日志 |
| `[solana]` | RPC、ALT、`commitment` |
| `[snapshot]` | 内盘账户拉取 **`processed`** |
| `[ifx]` | 主网 program id、公共 Frame |
| `[quotes]` | WSOL / USDC / USDT mint |
| `[venues.*]` | 三内盘 program id |
| `[bridge.*]` | flat 池白名单（`supported_types` 同时决定 Jupiter dex 过滤）、`max_swap_accounts` |
| `[jupiter]` | 单跳池发现（**不**使用 Jupiter swap tx） |
| `[jito]` | `*Mev` 变体 tip 账户与金额 |
| `[priority_fee.*]` | low / medium / high CU 与 microLamports |
| `[service_fee]` | 平台费 bps + SOL 直收 pubkey |
| `[sponsor]` | 代付开关、`pubkey`（fee payer）、`repay_pubkey`（gas 偿还收款）、`keypair_path`、`repay_buffer_percent` |
| `[tx]` | `max_bytes`（默认 1232） |

### Sponsor 代付

```toml
[sponsor]
enabled = true
pubkey = "<fee-payer-pubkey>"      # 与 keypair 一致；tx fee payer + ATA rent 出资
repay_pubkey = "<repay-treasury>"  # 用户 SOL 偿还目标；可与 pubkey 相同
keypair_path = "./keys/sponsor.json"
repay_buffer_percent = 10
```

要求：

1. 路由须经 SOL / WSOL（`sponsoredSwap` 从成交资金偿还，纯 U 路径不可用）
2. Sponsor 钱包有足够 SOL（代付 rent + tx fee，用户偿还后回笼）
3. 服务启动时加载 keypair，对 sponsored 变体 **partial co-sign**

## HTTP API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/health` | RPC 连通性 |
| `GET` | `/api/config/public` | 公开配置（rpcUrl、滑点默认、jito/sponsor 开关等） |
| `POST` | `/api/quote` | 询价 + 构建（见下） |
| `POST` | `/api/tx/inspect` | 解析 base64 交易结构 |
| `POST` | `/api/tx/simulate` | RPC 模拟 |
| `GET` | `/` | 静态交易页 |

### `POST /api/quote`

请求体（主要字段）：

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

`inputSettlement` / `outputSettlement`：`native_sol` | `wsol_spl` | `spl`（影响 WSOL wrap/unwrap 与 SOL 偿还路径）。

响应（节选）：

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

前端切换变体时**无需重新询价**，直接选用 `builds[variant].transaction` 签名发送。

`capabilities.reason` 常见值：`no_sol_in_route`、`sponsor_disabled`、`sponsor_keypair`、`tx_too_large`、`sponsored_not_wired` 等。

## 支持的路由示例

| 用户操作 | 路径 |
|----------|------|
| SOL 买 SOL 池代币 | Pump buy（1 腿） |
| USDT 买 SOL 池代币 | USDT → Raydium → WSOL → unwrap → repay → Pump buy（2 腿 + Ifx） |
| SOL 池代币卖出换 USDT | Pump sell → WSOL → bridge → USDT（2 腿 + Ifx） |
| 同 quote 双内盘 A→B | Pump sell A → Pump buy B（2 腿 + Ifx） |

## 文档

- 开发计划（中文）：[docs/plan.zh-CN.md](docs/plan.zh-CN.md)
- Development plan: [docs/plan.md](docs/plan.md)
- Pump.fun 规格（中文）：[docs/venues/pumpfun.zh-CN.md](docs/venues/pumpfun.zh-CN.md)
- Pump.fun spec: [docs/venues/pumpfun.md](docs/venues/pumpfun.md)

## 许可与免责

演示用途。自行承担链上交易风险；主网使用前请审计配置、密钥与资金托管流程。

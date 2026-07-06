import { Connection, PublicKey, VersionedTransaction } from "@solana/web3.js";

const SOL_RESERVE = 0.01;

const els = {
  connectBtn: document.getElementById("connectBtn"),
  disconnectBtn: document.getElementById("disconnectBtn"),
  fromMint: document.getElementById("fromMint"),
  toMint: document.getElementById("toMint"),
  fromBalance: document.getElementById("fromBalance"),
  toBalance: document.getElementById("toBalance"),
  maxBtn: document.getElementById("maxBtn"),
  inputAmount: document.getElementById("inputAmount"),
  outputAmount: document.getElementById("outputAmount"),
  flipBtn: document.getElementById("flipBtn"),
  slippage: document.getElementById("slippage"),
  quoteBtn: document.getElementById("quoteBtn"),
  simulateBtn: document.getElementById("simulateBtn"),
  sendBtn: document.getElementById("sendBtn"),
  statusMsg: document.getElementById("statusMsg"),
  inspectorStatus: document.getElementById("inspectorStatus"),
  simBanner: document.getElementById("simBanner"),
  inspectorTx: document.getElementById("inspectorTx"),
  inspectorFee: document.getElementById("inspectorFee"),
  inspectorRoute: document.getElementById("inspectorRoute"),
  inspectorMeta: document.getElementById("inspectorMeta"),
  inspectorInstructions: document.getElementById("inspectorInstructions"),
  inspectorRaw: document.getElementById("inspectorRaw"),
  inspectorRawPre: document.getElementById("inspectorRawPre"),
  fromQuoteChips: document.getElementById("fromQuoteChips"),
  toQuoteChips: document.getElementById("toQuoteChips"),
  mevToggle: document.getElementById("mevToggle"),
  sponsoredToggle: document.getElementById("sponsoredToggle"),
  mevToggleHint: document.getElementById("mevToggleHint"),
  sponsoredToggleHint: document.getElementById("sponsoredToggleHint"),
  settlementModeWrap: document.getElementById("settlementModeWrap"),
};

let wallet = null;
let lastQuote = null;
let lastSimulation = null;
let lastSignature = null;
let lastTxStatus = null; // pending | confirmed | failed
let publicConfig = null;
let balanceTimer = null;
let fromSettlement = "native_sol";
let toSettlement = "native_sol";
let settlementMode = "close";
const balanceCache = { from: null, to: null };

function settlementAwareKey(baseKey) {
  const modes = lastQuote?.settlementModes;
  if (modes?.length) {
    const mode = modes.includes(settlementMode) ? settlementMode : modes[0];
    return `${baseKey}_${mode}`;
  }
  return baseKey;
}

function activeVariantKey() {
  const sponsored = els.sponsoredToggle?.checked;
  const mev = els.mevToggle?.checked;
  let base = "selfFunded";
  if (sponsored && mev) base = "sponsoredSwapMev";
  else if (sponsored) base = "sponsoredSwap";
  else if (mev) base = "selfFundedMev";
  return settlementAwareKey(base);
}

function activeBuild() {
  if (!lastQuote) return null;
  const key = activeVariantKey();
  const fromMap = lastQuote.builds?.[key];
  if (fromMap) return fromMap;
  if (key === "selfFunded" && lastQuote.build) return lastQuote.build;
  return null;
}

function variantStatusLabel(key, build) {
  const mode = build?.settlementMode;
  const modeLabel = mode === "close" ? " · close ATA" : mode === "unwrapAll" ? " · UnwrapLamports" : "";
  const size = build?.transactionSizeBytes;
  return size ? `Variant ${key}${modeLabel} · ${size} B` : `Variant ${key}${modeLabel}`;
}

function capabilityReasonText(reason) {
  const maxB = publicConfig?.maxTxBytes;
  const map = {
    jito_disabled: "Jito is disabled on the server",
    no_sol_in_route: "Route has no SOL/WSOL to recover gas",
    sponsor_disabled: "Sponsor is disabled on the server",
    sponsor_keypair: "Sponsor keypair not configured or invalid",
    build_error: "Failed to build this variant",
    sponsored_not_wired: "Sponsored swap not wired for this route",
    jito_tip: "Failed to build Jito tip instruction",
    compile_error: "Transaction compile failed",
    tx_too_large: maxB ? `Transaction exceeds size limit (${maxB} B)` : "Transaction exceeds size limit",
    sponsor_sign: "Sponsor signing failed",
    marshal_error: "Transaction serialization failed",
    wsol_unwrap_no_mev: "WSOL unwrap does not support MEV",
    sol_settlement_no_mev: "SOL/WSOL conversion does not support MEV",
    wrap_user_pays: "Wrapping SOL to WSOL requires user-paid gas",
  };
  return map[reason] ?? reason ?? "unavailable";
}

function variantToggleState(baseKey) {
  const variantKey = settlementAwareKey(baseKey);
  const caps = lastQuote?.capabilities ?? {};
  const cap = caps[variantKey];

  if (baseKey === "selfFundedMev") {
    if (lastQuote?.pairClass === "sol_settlement") {
      return { disabled: true, reason: "SOL/WSOL conversion does not support MEV" };
    }
    if (publicConfig?.jitoEnabled === false) {
      return { disabled: true, reason: "Jito is disabled on the server" };
    }
    if (cap && !cap.supported) {
      return { disabled: true, reason: capabilityReasonText(cap.reason) };
    }
    return { disabled: false, reason: "" };
  }

  if (baseKey === "sponsoredSwap" || baseKey === "sponsoredSwapMev") {
    if (baseKey === "sponsoredSwapMev" && lastQuote?.pairClass === "sol_settlement") {
      return { disabled: true, reason: "SOL/WSOL conversion does not support MEV" };
    }
    if (publicConfig?.sponsorEnabled === false) {
      return { disabled: true, reason: "Sponsor is disabled on the server" };
    }
    if (cap && !cap.supported) {
      return { disabled: true, reason: capabilityReasonText(cap.reason) };
    }
    return { disabled: false, reason: "" };
  }

  return { disabled: false, reason: "" };
}

function applyToggleState(toggleEl, hintEl, labelEl, state) {
  if (!toggleEl) return;
  toggleEl.disabled = state.disabled;
  toggleEl.title = state.disabled ? state.reason : "";
  labelEl?.classList.toggle("toggle-label--disabled", state.disabled);
  if (hintEl) {
    if (state.disabled && state.reason) {
      hintEl.textContent = state.reason;
      hintEl.classList.remove("hidden");
    } else {
      hintEl.textContent = "";
      hintEl.classList.add("hidden");
    }
  }
  if (state.disabled && toggleEl.checked) toggleEl.checked = false;
}

function syncSettlementModeToggles() {
  const wrap = els.settlementModeWrap;
  if (!wrap) return;
  const modes = lastQuote?.settlementModes;
  const show = Array.isArray(modes) && modes.length > 0;
  wrap.classList.toggle("hidden", !show);
  if (!show) return;
  if (!modes.includes(settlementMode)) {
    settlementMode = modes[0];
  }
  wrap.querySelectorAll(".settlement-mode-btn").forEach((btn) => {
    const mode = btn.getAttribute("data-settle-mode");
    btn.classList.toggle("active", mode === settlementMode);
  });
}

function syncVariantToggles() {
  const mevLabel = els.mevToggle?.closest(".toggle-label");
  const spLabel = els.sponsoredToggle?.closest(".toggle-label");
  const spBase = els.mevToggle?.checked ? "sponsoredSwapMev" : "sponsoredSwap";

  syncSettlementModeToggles();
  applyToggleState(els.mevToggle, els.mevToggleHint, mevLabel, variantToggleState("selfFundedMev"));
  applyToggleState(els.sponsoredToggle, els.sponsoredToggleHint, spLabel, variantToggleState(spBase));
}

async function onVariantToggle() {
  syncVariantToggles();
  const key = activeVariantKey();
  const build = activeBuild();
  if (!lastQuote) return;
  if (!build) {
    const state = variantToggleState(key);
    const msg = state.reason || `Variant ${key} unavailable`;
    els.inspectorStatus.textContent = msg;
    els.simulateBtn.disabled = true;
    els.sendBtn.disabled = true;
    lastSimulation = null;
    renderTxInspector(lastQuote, null, msg);
    return;
  }
  lastQuote.build = build;
  lastSimulation = null;
  els.simulateBtn.disabled = false;
  els.sendBtn.disabled = !wallet;
  renderQuote(lastQuote);
  renderTxInspector(lastQuote, build, `${variantStatusLabel(key, build)} · simulating…`);
  if (build.transaction) {
    await runSimulate({ quiet: true });
  }
}

function getProvider() {
  const p = window.solana;
  if (p?.isPhantom) return p;
  return null;
}

async function loadConfig() {
  const res = await fetch("/api/config/public");
  publicConfig = await res.json();
  renderQuoteChips();
  syncVariantToggles();
}

function renderQuoteChips() {
  const q = publicConfig?.quoteMints;
  if (!q) return;

  const fromQuotes = [
    { label: "Native SOL", mint: q.wsol, settlement: "native_sol" },
    { label: "WSOL", mint: q.wsol, settlement: "wsol_spl" },
    { label: "USDC", mint: q.usdc, settlement: "spl" },
    { label: "USDT", mint: q.usdt, settlement: "spl" },
  ].filter((x) => x.mint);

  const toQuotes = [
    { label: "Native SOL", mint: q.wsol, settlement: "native_sol" },
    { label: "WSOL", mint: q.wsol, settlement: "wsol_spl" },
    { label: "USDC", mint: q.usdc, settlement: "spl" },
    { label: "USDT", mint: q.usdt, settlement: "spl" },
  ].filter((x) => x.mint);

  const makeChips = (items, target) =>
    items
      .map(
        (x) =>
          `<button type="button" class="chip-btn" data-mint="${x.mint}" data-settlement="${x.settlement}" data-target="${target}">${x.label}</button>`
      )
      .join("");

  els.fromQuoteChips.innerHTML = makeChips(fromQuotes, "from");
  els.toQuoteChips.innerHTML = makeChips(toQuotes, "to");

  document.querySelectorAll(".chip-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      const mint = btn.getAttribute("data-mint");
      const settlement = btn.getAttribute("data-settlement");
      const target = btn.getAttribute("data-target");
      if (target === "from") {
        els.fromMint.value = mint;
        fromSettlement = settlement;
        updateFromChipActive();
      } else {
        els.toMint.value = mint;
        toSettlement = settlement;
        updateToChipActive();
      }
      scheduleBalanceRefresh();
    });
  });
  updateFromChipActive();
  updateToChipActive();
}

function updateToChipActive() {
  const wsol = publicConfig?.quoteMints?.wsol;
  document.querySelectorAll("#toQuoteChips .chip-btn").forEach((btn) => {
    const sameMint = btn.getAttribute("data-mint") === els.toMint.value.trim();
    const sameSettle = btn.getAttribute("data-settlement") === toSettlement;
    const active = wsol && els.toMint.value.trim() === wsol ? sameMint && sameSettle : sameMint;
    btn.classList.toggle("active", active);
  });
}

function updateFromChipActive() {
  const wsol = publicConfig?.quoteMints?.wsol;
  document.querySelectorAll("#fromQuoteChips .chip-btn").forEach((btn) => {
    const sameMint = btn.getAttribute("data-mint") === els.fromMint.value.trim();
    const sameSettle = btn.getAttribute("data-settlement") === fromSettlement;
    const active = wsol && els.fromMint.value.trim() === wsol ? sameMint && sameSettle : sameMint;
    btn.classList.toggle("active", active);
  });
}

function resolveInputSettlement() {
  const q = publicConfig?.quoteMints;
  const mint = els.fromMint.value.trim();
  if (!q) return "";
  if (mint === q.wsol) return fromSettlement;
  if (mint === q.usdc || mint === q.usdt) return "spl";
  return "";
}

function resolveOutputSettlement() {
  const q = publicConfig?.quoteMints;
  const mint = els.toMint.value.trim();
  if (!q) return "";
  if (mint === q.wsol) return toSettlement;
  if (mint === q.usdc || mint === q.usdt) return "spl";
  return "";
}

function isSOLSettlementConvert(inputMint, outputMint) {
  const wsol = publicConfig?.quoteMints?.wsol;
  if (!wsol || inputMint !== wsol || outputMint !== wsol) return false;
  return (
    (fromSettlement === "wsol_spl" && toSettlement === "native_sol") ||
    (fromSettlement === "native_sol" && toSettlement === "wsol_spl")
  );
}

function balanceLabel(mint, settlement) {
  return mintLabel(mint, settlement);
}

function getConnection() {
  const rpc = publicConfig?.rpcUrl;
  if (!rpc) return null;
  return new Connection(rpc, "confirmed");
}

function shortMint(mint) {
  if (!mint) return "—";
  if (mint.length <= 12) return mint;
  return `${mint.slice(0, 4)}…${mint.slice(-4)}`;
}

function mintLabel(mint, settlement) {
  if (!mint) return "—";
  const q = publicConfig?.quoteMints;
  if (q && mint === q.wsol) {
    return settlement === "wsol_spl" ? "WSOL" : "SOL";
  }
  if (q) {
    if (mint === q.usdc) return "USDC";
    if (mint === q.usdt) return "USDT";
  }
  return shortMint(mint);
}

let lastInputAmountRaw = null;

function formatAmountFromRaw(raw, decimals) {
  if (raw == null || raw === 0n) return "0";
  const s = raw.toString();
  if (decimals === 0) return s;
  const padded = s.padStart(decimals + 1, "0");
  const whole = padded.slice(0, -decimals) || "0";
  const frac = padded.slice(-decimals).replace(/0+$/, "");
  return frac ? `${whole}.${frac}` : whole;
}

function formatUiAmount(ui) {
  if (!ui || ui === 0) return "0";
  if (ui < 0.000001) return ui.toExponential(2);
  if (ui < 1) return ui.toFixed(6).replace(/\.?0+$/, "");
  return ui.toFixed(4).replace(/\.?0+$/, "");
}

function formatBalance(bal, mint, settlement) {
  if (bal === null) return "Balance —";
  if (bal === undefined) return "Balance …";
  const label = balanceLabel(mint, settlement);
  if (bal.raw === 0n) return `Balance 0 ${label}`;
  return `Balance ${formatAmountFromRaw(bal.raw, bal.decimals)} ${label}`;
}

function setBalanceLoading() {
  els.fromBalance.textContent = "Balance …";
  els.toBalance.textContent = "Balance …";
}

function renderBalances() {
  const fromMint = els.fromMint.value.trim();
  const toMint = els.toMint.value.trim();
  els.fromBalance.textContent = formatBalance(balanceCache.from, fromMint, fromSettlement);
  els.toBalance.textContent = formatBalance(balanceCache.to, toMint, toSettlement);
  const canMax = wallet && balanceCache.from && balanceCache.from.raw > 0n;
  els.maxBtn.hidden = !canMax;
}

function clearBalances() {
  balanceCache.from = null;
  balanceCache.to = null;
  els.fromBalance.textContent = "Balance —";
  els.toBalance.textContent = "Balance —";
  els.maxBtn.hidden = true;
}

async function fetchMintBalance(connection, mintStr, settlement) {
  if (!wallet || !mintStr) return null;
  const q = publicConfig?.quoteMints;

  if (mintStr === q?.wsol && settlement === "native_sol") {
    const lamports = await connection.getBalance(wallet);
    return { raw: lamports, decimals: 9, ui: lamports / 1e9 };
  }

  try {
    const mint = new PublicKey(mintStr);
    const resp = await connection.getParsedTokenAccountsByOwner(wallet, { mint });
    if (resp.value.length === 0) {
      return { raw: 0n, decimals: 0, ui: 0 };
    }
    let total = 0n;
    let decimals = 0;
    for (const { account } of resp.value) {
      const amount = account.data?.parsed?.info?.tokenAmount;
      if (!amount) continue;
      decimals = amount.decimals;
      total += BigInt(amount.amount);
    }
    const ui = Number(total) / 10 ** decimals;
    return { raw: total, decimals, ui };
  } catch {
    return null;
  }
}

async function refreshBalances() {
  if (!wallet) {
    clearBalances();
    return;
  }
  const connection = getConnection();
  if (!connection) {
    els.fromBalance.textContent = "Balance (no RPC)";
    els.toBalance.textContent = "Balance (no RPC)";
    return;
  }

  const fromMint = els.fromMint.value.trim();
  const toMint = els.toMint.value.trim();
  setBalanceLoading();

  try {
    const [fromBal, toBal] = await Promise.all([
      fromMint ? fetchMintBalance(connection, fromMint, fromSettlement) : null,
      toMint ? fetchMintBalance(connection, toMint, toSettlement) : null,
    ]);
    balanceCache.from = fromBal;
    balanceCache.to = toBal;
    renderBalances();
  } catch (e) {
    console.error(e);
    els.fromBalance.textContent = "Balance fetch failed";
    els.toBalance.textContent = "Balance fetch failed";
  }
}

function scheduleBalanceRefresh() {
  clearTimeout(balanceTimer);
  balanceTimer = setTimeout(() => refreshBalances().catch(console.error), 350);
}

function fillMaxFrom() {
  const bal = balanceCache.from;
  if (!bal || bal.raw === 0n) return;
  const mint = els.fromMint.value.trim();
  const q = publicConfig?.quoteMints;
  if (mint === q?.wsol && fromSettlement === "native_sol") {
    const reserve = BigInt(Math.round(SOL_RESERVE * 1e9));
    const maxRaw = bal.raw > reserve ? bal.raw - reserve : 0n;
    if (maxRaw === 0n) return;
    els.inputAmount.value = formatAmountFromRaw(maxRaw, 9);
    lastInputAmountRaw = maxRaw.toString();
    return;
  }
  els.inputAmount.value = formatAmountFromRaw(bal.raw, bal.decimals);
  lastInputAmountRaw = bal.raw.toString();
}

function clearInputAmountRaw() {
  lastInputAmountRaw = null;
}

function readPair() {
  return {
    inputMint: els.fromMint.value.trim(),
    outputMint: els.toMint.value.trim(),
    inputAmount: els.inputAmount.value.trim(),
  };
}

function validatePair() {
  const { inputMint, outputMint, inputAmount } = readPair();
  if (!inputMint || !outputMint) {
    alert("Enter both mints");
    return null;
  }
  if (inputMint === outputMint && !isSOLSettlementConvert(inputMint, outputMint)) {
    const wsol = publicConfig?.quoteMints?.wsol;
    if (inputMint === wsol) {
      alert("Same WSOL mint requires different settlement (Native SOL ↔ WSOL)");
    } else {
      alert("Input and output mints must differ");
    }
    return null;
  }
  if (!inputAmount) {
    alert("Enter an input amount");
    return null;
  }
  return { inputMint, outputMint, inputAmount };
}

function flipMints() {
  const from = els.fromMint.value;
  const to = els.toMint.value;
  els.fromMint.value = to;
  els.toMint.value = from;
  const prevFromSettle = fromSettlement;
  fromSettlement = toSettlement;
  toSettlement = prevFromSettle;
  updateFromChipActive();
  updateToChipActive();
  els.outputAmount.value = "";
  lastQuote = null;
  lastSignature = null;
  lastTxStatus = null;
  syncVariantToggles();
  els.sendBtn.disabled = true;
  els.statusMsg.textContent = "Mints swapped";
  renderTxInspector(null, null, "Mints swapped");
  scheduleBalanceRefresh();
}

function updateWalletUI() {
  if (wallet) {
    const addr = wallet.toBase58();
    els.connectBtn.hidden = true;
    els.disconnectBtn.hidden = false;
    els.disconnectBtn.textContent = `${addr.slice(0, 4)}…${addr.slice(-4)}`;
  } else {
    els.connectBtn.hidden = false;
    els.disconnectBtn.hidden = true;
    els.connectBtn.textContent = "Connect wallet";
  }
}

async function connectWallet() {
  const provider = getProvider();
  if (!provider) {
    alert("Install Phantom wallet");
    return;
  }
  const resp = await provider.connect();
  wallet = resp.publicKey;
  updateWalletUI();
  await refreshBalances();
}

async function disconnectWallet() {
  const provider = getProvider();
  if (provider) await provider.disconnect();
  wallet = null;
  updateWalletUI();
  clearBalances();
}

function acctFlags(isSigner, isWritable) {
  const parts = [];
  if (isSigner) parts.push("S");
  if (isWritable) parts.push("W");
  return parts.length ? parts.join("") : "—";
}

function solscanTxUrl(signature) {
  return `https://solscan.io/tx/${signature}`;
}

function solscanAccountUrl(address) {
  return `https://solscan.io/account/${address}`;
}

function formatLamports(lamports) {
  const n = Number(lamports);
  if (!Number.isFinite(n)) return "—";
  return `${(n / 1e9).toFixed(9).replace(/\.?0+$/, "")} SOL`;
}

function formatRawAmount(raw, decimals = 9) {
  const n = Number(raw);
  if (!Number.isFinite(n)) return "—";
  return (n / 10 ** decimals).toFixed(Math.min(decimals, 6)).replace(/\.?0+$/, "");
}

function feeBasisLabel(basis) {
  if (basis === "input_lamports") return "Buy: deducted from input SOL";
  if (basis === "min_gross_lamports") return "Sell: based on min SOL output after slippage (conservative)";
  return basis || "—";
}

function txStatusLabel(status) {
  if (status === "confirmed") return { text: "Confirmed on-chain", cls: "status-ok" };
  if (status === "failed") return { text: "Failed on-chain", cls: "status-fail" };
  if (status === "pending") return { text: "Awaiting confirmation…", cls: "status-pending" };
  return null;
}

function acctLink(pubkey) {
  if (!pubkey) return "—";
  const short = pubkey.length > 12 ? `${pubkey.slice(0, 4)}…${pubkey.slice(-4)}` : pubkey;
  return `<a class="ext-link" href="${solscanAccountUrl(pubkey)}" target="_blank" rel="noopener noreferrer" title="${pubkey}">${short}</a>`;
}

function wireCopyButtons(root) {
  root.querySelectorAll("[data-copy]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const text = btn.getAttribute("data-copy") ?? "";
      try {
        await navigator.clipboard.writeText(text);
        const prev = btn.textContent;
        btn.textContent = "Copied";
        setTimeout(() => { btn.textContent = prev; }, 1200);
      } catch { /* ignore */ }
    });
  });
}

function makeCopyButton(text, label = "Copy") {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "copy-btn";
  btn.textContent = label;
  btn.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(text);
      const prev = btn.textContent;
      btn.textContent = "Copied";
      setTimeout(() => { btn.textContent = prev; }, 1200);
    } catch { /* ignore */ }
  });
  return btn;
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function renderSimBanner(simulation) {
  if (!simulation) {
    els.simBanner.classList.add("hidden");
    els.simBanner.innerHTML = "";
    return;
  }
  els.simBanner.classList.remove("hidden");
  const ok = simulation.ok;
  els.simBanner.className = `sim-banner ${ok ? "sim-ok" : "sim-fail"}`;
  const logs = (simulation.logs ?? []).slice(-20);
  const errText = simulation.error
    ? typeof simulation.error === "string"
      ? simulation.error
      : JSON.stringify(simulation.error, null, 2)
    : "";
  const simCopyText = [errText, ...(simulation.logs ?? [])].filter(Boolean).join("\n");
  els.simBanner.innerHTML = `
    <div class="sim-head">
      <span class="sim-badge ${ok ? "badge-ok" : "badge-fail"}">${ok ? "SIM OK" : "SIM FAIL"}</span>
      <span class="sim-title">${ok ? "RPC simulation passed" : "RPC simulation failed"}</span>
    </div>
    ${simulation.unitsConsumed != null ? `<div class="sim-meta">Compute · ${simulation.unitsConsumed.toLocaleString()} CU</div>` : ""}
    ${errText ? `<pre class="sim-error">${escapeHtml(errText)}</pre>` : ""}
    ${logs.length ? `<details class="sim-logs-wrap"><summary>Program logs (${logs.length})</summary><pre class="sim-logs">${escapeHtml(logs.join("\n"))}</pre></details>` : ""}
  `;
  const simHead = els.simBanner.querySelector(".sim-head");
  if (simHead && simCopyText) {
    simHead.appendChild(makeCopyButton(simCopyText, "Copy logs"));
  }
}

function renderInspectorRoute(quoteData) {
  if (!quoteData) {
    els.inspectorRoute.classList.add("hidden");
    els.inspectorRoute.innerHTML = "";
    return;
  }
  els.inspectorRoute.classList.remove("hidden");
  const venue = quoteData.route?.[0]?.venue ?? "—";
  els.inspectorRoute.innerHTML = `
    <div class="card-title">Route</div>
    <div class="info-grid">
      <div class="info-item"><span>Source</span><code>${quoteData.source ?? "—"}</code></div>
      <div class="info-item"><span>Pair</span><code>${quoteData.pairClass ?? "—"}</code></div>
      <div class="info-item"><span>Venue</span><code>${venue}</code></div>
      <div class="info-item"><span>Snapshot</span><code>${quoteData.snapshot?.accountCount ?? "—"} accts</code></div>
    </div>
  `;
}

function renderInspectorFee(quoteData, build) {
  const q = quoteData?.quote;
  const feeRaw = q?.serviceFeeRaw ?? (build?.serviceFeeLamports != null ? String(build.serviceFeeLamports) : "");
  if (!feeRaw) {
    els.inspectorFee.classList.add("hidden");
    els.inspectorFee.innerHTML = "";
    return;
  }
  els.inspectorFee.classList.remove("hidden");
  const bps = publicConfig?.serviceFeeBps ?? "—";
  const gross = q?.grossOutputAmount ? formatLamports(q.grossOutputAmount) : null;
  els.inspectorFee.innerHTML = `
    <div class="card-title">Platform fee</div>
    <div class="info-grid">
      <div class="info-item"><span>Rate</span><code>${bps} bps</code></div>
      <div class="info-item"><span>Charged</span><code>${formatLamports(feeRaw)}</code></div>
      <div class="info-item"><span>Basis</span><code>${feeBasisLabel(q?.serviceFeeBasis)}</code></div>
      ${gross ? `<div class="info-item"><span>Pump output</span><code>${gross}</code></div>` : ""}
      ${q?.minOutputAmount ? `<div class="info-item"><span>Min received</span><code>${formatLamports(q.minOutputAmount)}</code></div>` : ""}
    </div>
    <p class="fee-note">On sells, a <code>SystemProgram.transfer</code> is appended after the Pump instruction; the fee is estimated from the curve at quote time using a conservative basis.</p>
  `;
}

function renderInspectorTx(build, signature, txStatus) {
  if (!build?.transaction && !signature) {
    els.inspectorTx.classList.add("hidden");
    els.inspectorTx.innerHTML = "";
    return;
  }
  els.inspectorTx.classList.remove("hidden");
  const status = txStatusLabel(txStatus);
  const sigRow = signature
    ? `<div class="tx-sig-row">
        <code class="tx-sig">${signature}</code>
        <a class="ext-link" href="${solscanTxUrl(signature)}" target="_blank" rel="noopener noreferrer">Solscan ↗</a>
        <button type="button" class="copy-btn" data-copy="${signature}">Copy</button>
      </div>`
    : `<p class="muted">Solscan link appears after signing</p>`;
  els.inspectorTx.innerHTML = `
    <div class="card-title">Transaction</div>
    <div class="info-grid">
      <div class="info-item"><span>Variant</span><code>${build?.variant ?? activeVariantKey()}</code></div>
      ${build?.variant?.includes("Mev") ? `<div class="info-item"><span>Jito tip</span><code>${formatLamports(String(build.jitoTipLamports ?? 0))}</code>${!build.jitoTipLamports ? ' <span class="muted">(tip_lamports=0)</span>' : ""}</div>` : ""}
      ${build?.repayEstimateLamports ? `<div class="info-item"><span>Repay est.</span><code>${formatLamports(String(build.repayEstimateLamports))}</code></div>` : ""}
      <div class="info-item"><span>Size</span><code>${build?.transactionSizeBytes ?? "—"} B</code></div>
      <div class="info-item"><span>Fee payer</span>${acctLink(build?.feePayer)}</div>
      <div class="info-item"><span>Blockhash</span><code>${build?.recentBlockhash ?? "—"}</code></div>
      ${status ? `<div class="info-item"><span>On-chain</span><span class="status-pill ${status.cls}">${status.text}</span></div>` : ""}
    </div>
    ${sigRow}
    ${build?.transaction ? `<button type="button" class="copy-btn" data-copy="${build.transaction}">Copy tx base64</button>` : ""}
  `;
  wireCopyButtons(els.inspectorTx);
}

function renderTxInspector(quoteData, build, statusText, signature = lastSignature, txStatus = lastTxStatus) {
  els.inspectorStatus.textContent = statusText;
  renderInspectorRoute(quoteData);
  renderInspectorFee(quoteData, build);
  renderInspectorTx(build, signature, txStatus);

  const inspection = build?.inspection;
  const simulation = build?.simulation ?? lastSimulation;
  if (!inspection) {
    els.inspectorMeta.classList.add("hidden");
    els.inspectorRaw.classList.add("hidden");
    els.inspectorInstructions.innerHTML = "";
    renderSimBanner(simulation);
    return;
  }

  renderSimBanner(simulation);

  els.inspectorMeta.classList.remove("hidden");
  els.inspectorRaw.classList.remove("hidden");
  els.inspectorMeta.innerHTML = `
    <div class="meta-kv"><span>Format</span><code>${inspection.format}</code></div>
    <div class="meta-kv"><span>Instructions</span><code>${inspection.numInstructions}</code></div>
    <div class="meta-kv"><span>Tx size</span><code>${inspection.transactionSizeBytes ?? "—"} B</code></div>
    <div class="meta-kv"><span>Accounts</span><code>${inspection.totalAccountKeys}</code></div>
    <div class="meta-kv"><span>Fee payer</span>${acctLink(inspection.feePayer)}</div>
    <div class="meta-kv"><span>Blockhash</span><code>${inspection.recentBlockhash ?? "—"}</code></div>
  `;

  const instructions = inspection.instructions ?? [];
  els.inspectorInstructions.innerHTML = instructions
    .map(
      (ix) => `
      <article class="ix-card">
        <header class="ix-card-head">
          <span class="ix-index">#${ix.index}</span>
          <span class="ix-program">${ix.programLabel}</span>
          ${ix.hint ? `<span class="ix-hint">${escapeHtml(ix.hint)}</span>` : ""}
          <span class="ix-program-id">${acctLink(ix.programId)}</span>
        </header>
        <table class="ix-accounts">
          <thead><tr><th>#</th><th>Account</th><th>Flags</th><th></th></tr></thead>
          <tbody>
            ${(ix.accounts ?? [])
              .map(
                (a, i) => `
              <tr>
                <td>${i}</td>
                <td>${acctLink(a.pubkey)}</td>
                <td class="acct-flags">${acctFlags(a.isSigner, a.isWritable)}</td>
                <td><button type="button" class="copy-btn" data-copy="${a.pubkey}">Copy</button></td>
              </tr>`
              )
              .join("")}
          </tbody>
        </table>
        <div class="ix-data">
          <div class="ix-data-label">
            <span>Data · ${ix.dataLength} bytes</span>
            <button type="button" class="copy-btn" data-copy="${ix.dataHex}">Copy hex</button>
          </div>
          <pre>${ix.dataHex}</pre>
        </div>
      </article>`
    )
    .join("");

  const rawPayload = JSON.stringify(
    { quote: quoteData?.quote, build, route: quoteData?.route, pairClass: quoteData?.pairClass, simulation },
    null,
    2
  );
  els.inspectorRawPre.textContent = rawPayload;
  const rawSummary = els.inspectorRaw.querySelector("summary");
  if (rawSummary) {
    rawSummary.querySelector(".raw-copy-btn")?.remove();
    const copyBtn = makeCopyButton(rawPayload, "Copy");
    copyBtn.classList.add("raw-copy-btn");
    rawSummary.appendChild(copyBtn);
  }
  wireCopyButtons(els.inspectorInstructions);
}

async function pollTxConfirmation(signature) {
  const connection = getConnection();
  if (!connection || !signature) return;
  lastTxStatus = "pending";
  renderTxInspector(lastQuote, activeBuild(), els.inspectorStatus.textContent, signature, lastTxStatus);
  try {
    const { value } = await connection.confirmTransaction(signature, "confirmed");
    lastTxStatus = value?.err ? "failed" : "confirmed";
  } catch {
    lastTxStatus = "failed";
  }
  renderTxInspector(lastQuote, activeBuild(), els.inspectorStatus.textContent, signature, lastTxStatus);
  if (lastTxStatus === "confirmed") {
    els.inspectorStatus.textContent = `Transaction confirmed · ${signature.slice(0, 8)}…`;
  } else {
    els.inspectorStatus.textContent = `Transaction failed · ${signature.slice(0, 8)}…`;
  }
}

function renderQuote(data) {
  const { inputMint, outputMint, inputAmount } = readPair();
  const outRaw = data.quote?.outputAmount ?? "";
  els.outputAmount.value = outRaw;

  const min = data.quote?.minOutputAmount;
  const venue = data.route?.[0]?.venue ?? data.source ?? "";
  let status = `${inputAmount} ${mintLabel(inputMint, fromSettlement)} → ${outRaw || "?"} ${mintLabel(outputMint, toSettlement)}`;
  if (min) status += ` · min ${min}`;
  if (venue) status += ` · ${venue}`;
  const build = activeBuild();
  if (build?.transactionSizeBytes) status += ` · ${build.transactionSizeBytes}B`;
  const buildKeys = data.builds ? Object.keys(data.builds) : [];
  if (buildKeys.length > 1) status += ` · ${buildKeys.length} variants`;
  els.statusMsg.textContent = status;
}

async function simulateTransaction(b64) {
  const res = await fetch("/api/tx/simulate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ transaction: b64 }),
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || "Simulation failed");
  }
  return data;
}

async function runSimulate({ quiet = false } = {}) {
  const tx = activeBuild()?.transaction;
  if (!tx) {
    if (!quiet) alert("Quote first and ensure a transaction was built");
    return null;
  }
  if (!quiet) {
    els.inspectorStatus.textContent = "RPC simulating…";
    els.simulateBtn.disabled = true;
  }
  try {
    lastSimulation = await simulateTransaction(tx);
    const note = lastSimulation.ok ? " · simulation passed" : " · simulation failed";
    const b = activeBuild();
    const base = b?.transactionSizeBytes
      ? `${variantStatusLabel(activeVariantKey(), b)}${note}`
      : `Simulation complete${note}`;
    renderTxInspector(lastQuote, activeBuild(), base);
    if (!quiet) {
      els.inspectorStatus.textContent = lastSimulation.ok
        ? `Simulation passed${lastSimulation.unitsConsumed != null ? ` · ${lastSimulation.unitsConsumed.toLocaleString()} CU` : ""}`
        : "Simulation failed — see logs below";
    }
    return lastSimulation;
  } catch (e) {
    lastSimulation = null;
    renderSimBanner(null);
    if (!quiet) {
      els.inspectorStatus.textContent = e.message || "Simulation failed";
      alert(e.message || "Simulation failed");
    }
    return null;
  } finally {
    els.simulateBtn.disabled = !activeBuild()?.transaction;
  }
}

async function fetchQuote() {
  if (!wallet) {
    alert("Connect wallet first");
    return;
  }
  const pair = validatePair();
  if (!pair) return;

  const slippageBps = Math.round(parseFloat(els.slippage.value || "1") * 100);

  els.statusMsg.textContent = "Quoting…";
  els.sendBtn.disabled = true;
  els.simulateBtn.disabled = true;
  els.outputAmount.value = "";
  lastSimulation = null;
  renderTxInspector(null, null, "Quoting…");

  const body = {
      inputMint: pair.inputMint,
      outputMint: pair.outputMint,
      inputAmount: pair.inputAmount,
      slippageBps,
      userPubkey: wallet.toBase58(),
      priorityTier: publicConfig?.defaultPriorityTier || "medium",
      inputSettlement: resolveInputSettlement(),
      outputSettlement: resolveOutputSettlement(),
    };
  if (lastInputAmountRaw) {
    body.inputAmountRaw = lastInputAmountRaw;
  }
  const res = await fetch("/api/quote", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    els.statusMsg.textContent = data.error || "Quote failed";
    lastQuote = null;
    syncVariantToggles();
    renderTxInspector(null, null, data.error || "Quote failed");
    return;
  }

  lastQuote = data;
  syncVariantToggles();
  const build = activeBuild();
  if (build) lastQuote.build = build;
  renderQuote(data);

  lastSignature = null;
  lastTxStatus = null;
  if (build?.transaction) {
    renderTxInspector(data, build, `Ready · ${build.transactionSizeBytes} B · simulating…`);
    els.sendBtn.disabled = false;
    els.simulateBtn.disabled = false;
    await runSimulate({ quiet: true });
  } else {
    els.inspectorStatus.textContent = data.buildError || data.buildSkippedReason || "No transaction built";
    els.sendBtn.disabled = true;
    els.simulateBtn.disabled = true;
    renderTxInspector(data, data.build ?? null, data.buildError || data.buildSkippedReason || "No transaction built");
  }
}

async function sendTx() {
  const build = activeBuild();
  if (!build?.transaction) return;
  const provider = getProvider();
  if (!provider || !wallet) return;

  const raw = Uint8Array.from(atob(build.transaction), (c) => c.charCodeAt(0));
  const tx = VersionedTransaction.deserialize(raw);

  els.inspectorStatus.textContent = "Confirm in wallet…";
  const finishSend = async (signature) => {
    lastSignature = signature;
    lastTxStatus = "pending";
    renderTxInspector(lastQuote, build, `Sent · awaiting confirmation`, signature, lastTxStatus);
    await pollTxConfirmation(signature);
    await refreshBalances();
  };

  if (provider.signAndSendTransaction) {
    const { signature } = await provider.signAndSendTransaction(tx);
    await finishSend(signature);
    return;
  }

  const signed = await provider.signTransaction(tx);
  const connection = getConnection();
  if (!connection) {
    els.inspectorStatus.textContent = "Cannot send: RPC not configured";
    return;
  }
  const sig = await connection.sendRawTransaction(signed.serialize());
  await finishSend(sig);
}

els.connectBtn.addEventListener("click", () => connectWallet().catch((e) => alert(e.message)));
els.disconnectBtn.addEventListener("click", () => disconnectWallet().catch((e) => alert(e.message)));
els.quoteBtn.addEventListener("click", () => fetchQuote().catch((e) => alert(e.message)));
els.simulateBtn.addEventListener("click", () => runSimulate().catch((e) => alert(e.message)));
els.sendBtn.addEventListener("click", () => sendTx().catch((e) => alert(e.message)));
els.flipBtn.addEventListener("click", flipMints);
els.maxBtn.addEventListener("click", fillMaxFrom);
els.inputAmount.addEventListener("input", clearInputAmountRaw);
els.fromMint.addEventListener("input", () => {
  const q = publicConfig?.quoteMints;
  const mint = els.fromMint.value.trim();
  if (mint !== q?.wsol) fromSettlement = "spl";
  else if (fromSettlement !== "native_sol" && fromSettlement !== "wsol_spl") {
    fromSettlement = "native_sol";
  }
  updateFromChipActive();
  scheduleBalanceRefresh();
});
els.toMint.addEventListener("input", () => {
  const q = publicConfig?.quoteMints;
  const mint = els.toMint.value.trim();
  if (mint !== q?.wsol) toSettlement = "spl";
  else if (toSettlement !== "native_sol" && toSettlement !== "wsol_spl") {
    toSettlement = "native_sol";
  }
  updateToChipActive();
  scheduleBalanceRefresh();
});

els.mevToggle?.addEventListener("change", onVariantToggle);
els.sponsoredToggle?.addEventListener("change", onVariantToggle);
els.settlementModeWrap?.addEventListener("click", (e) => {
  const btn = e.target.closest(".settlement-mode-btn");
  if (!btn) return;
  const mode = btn.getAttribute("data-settle-mode");
  if (!mode || mode === settlementMode) return;
  settlementMode = mode;
  syncSettlementModeToggles();
  onVariantToggle();
});

loadConfig()
  .then(() => {
    if (getProvider()?.isConnected && getProvider().publicKey) {
      wallet = getProvider().publicKey;
      updateWalletUI();
      return refreshBalances();
    }
  })
  .catch(console.error);

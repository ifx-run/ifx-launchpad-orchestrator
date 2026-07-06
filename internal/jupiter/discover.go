package jupiter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/bridge"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/logx"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	cfg        *config.Config
}

func NewClient(cfg *config.Config) *Client {
	timeout := time.Duration(cfg.Jupiter.TimeoutSeconds) * time.Second
	transport := &http.Transport{}
	if cfg.Jupiter.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.Jupiter.ProxyURL)
		if err != nil {
			logx.Warn("jupiter", "invalid proxy_url, ignoring", "proxy", cfg.Jupiter.ProxyURL, "err", err)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
			logx.Info("jupiter", "http proxy enabled", "proxy", proxyURL.Host)
		}
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.Jupiter.APIURL, "/"),
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		cfg: cfg,
	}
}

type quoteResponse struct {
	InAmount       string `json:"inAmount"`
	OutAmount      string `json:"outAmount"`
	PriceImpactPct string `json:"priceImpactPct"`
	RoutePlan      []struct {
		Percent  int `json:"percent"`
		SwapInfo struct {
			AmmKey     string `json:"ammKey"`
			Label      string `json:"label"`
			InputMint  string `json:"inputMint"`
			OutputMint string `json:"outputMint"`
			InAmount   string `json:"inAmount"`
			OutAmount  string `json:"outAmount"`
		} `json:"swapInfo"`
	} `json:"routePlan"`
}

type DiscoverRequest struct {
	InputMint   string
	OutputMint  string
	Amount      string
	SlippageBPS int
}

// DiscoverSingleHop returns the best direct pool from Jupiter quote API.
func (c *Client) DiscoverSingleHop(ctx context.Context, req DiscoverRequest) (*bridge.DiscoveredPool, error) {
	start := time.Now()
	if !c.cfg.Jupiter.Enabled {
		return nil, fmt.Errorf("jupiter disabled")
	}

	u, err := url.Parse(c.baseURL + "/quote")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("inputMint", req.InputMint)
	q.Set("outputMint", req.OutputMint)
	q.Set("amount", req.Amount)
	q.Set("slippageBps", strconv.Itoa(req.SlippageBPS))
	q.Set("swapMode", "ExactIn")
	if c.cfg.Bridge.OnlyDirectRoutes {
		q.Set("onlyDirectRoutes", "true")
	}
	if len(c.cfg.Bridge.LowAccountDexes) > 0 {
		q.Set("dexes", strings.Join(c.cfg.Bridge.LowAccountDexes, ","))
	}
	u.RawQuery = q.Encode()

	logx.Info("jupiter", "discover request",
		"inputMint", req.InputMint,
		"outputMint", req.OutputMint,
		"amount", req.Amount,
		"slippageBps", req.SlippageBPS,
		"timeoutSec", c.cfg.Jupiter.TimeoutSeconds,
		"hasApiKey", c.cfg.Jupiter.APIKey != "",
		"proxy", c.cfg.Jupiter.ProxyURL,
		"url", u.String(),
	)
	logx.Debug("jupiter", "discover context",
		"ctxErr", ctx.Err(),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		logx.Error("jupiter", "build request failed", "err", err, "ms", logx.Since(start))
		return nil, err
	}
	if c.cfg.Jupiter.APIKey != "" {
		httpReq.Header.Set("x-api-key", c.cfg.Jupiter.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logx.Error("jupiter", "quote request failed",
			"err", err,
			"ms", logx.Since(start),
			"inputMint", req.InputMint,
			"outputMint", req.OutputMint,
			"amount", req.Amount,
		)
		return nil, fmt.Errorf("jupiter quote: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logx.Error("jupiter", "read body failed", "err", err, "status", resp.StatusCode, "ms", logx.Since(start))
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		bodyPreview := string(body)
		if len(bodyPreview) > 300 {
			bodyPreview = bodyPreview[:300] + "..."
		}
		logx.Error("jupiter", "quote non-200",
			"status", resp.StatusCode,
			"body", bodyPreview,
			"ms", logx.Since(start),
		)
		return nil, fmt.Errorf("jupiter quote %d: %s", resp.StatusCode, string(body))
	}

	var parsed quoteResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		logx.Error("jupiter", "decode response failed", "err", err, "ms", logx.Since(start))
		return nil, fmt.Errorf("decode jupiter quote: %w", err)
	}
	if len(parsed.RoutePlan) != 1 || parsed.RoutePlan[0].Percent != 100 {
		logx.Warn("jupiter", "unexpected route shape",
			"routePlanLen", len(parsed.RoutePlan),
			"ms", logx.Since(start),
		)
		return nil, fmt.Errorf("jupiter: expected single-hop route")
	}

	step := parsed.RoutePlan[0].SwapInfo
	poolType, ok := LabelToPoolType(step.Label)
	if !ok {
		logx.Warn("jupiter", "unsupported pool label", "label", step.Label, "ms", logx.Since(start))
		return nil, fmt.Errorf("jupiter: unsupported pool label %q", step.Label)
	}
	if !bridge.IsSupported(poolType, c.cfg.Bridge.SupportedTypes, c.cfg.Bridge.MaxSwapAccounts) {
		logx.Warn("jupiter", "pool not in whitelist",
			"poolType", poolType,
			"label", step.Label,
			"ms", logx.Since(start),
		)
		return nil, fmt.Errorf("jupiter: pool type %s not in whitelist or exceeds account budget", poolType)
	}

	pool := &bridge.DiscoveredPool{
		PoolID:      step.AmmKey,
		PoolType:    poolType,
		InputMint:   step.InputMint,
		OutputMint:  step.OutputMint,
		InAmount:    step.InAmount,
		OutAmount:   step.OutAmount,
		PriceImpact: parsed.PriceImpactPct,
		Label:       step.Label,
	}
	logx.Info("jupiter", "discover ok",
		"poolId", pool.PoolID,
		"poolType", pool.PoolType,
		"label", pool.Label,
		"inAmount", pool.InAmount,
		"outAmount", pool.OutAmount,
		"priceImpact", pool.PriceImpact,
		"ms", logx.Since(start),
	)
	return pool, nil
}

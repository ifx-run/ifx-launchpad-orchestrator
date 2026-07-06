package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/config"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/logx"
	"github.com/chopin65536/ifx-launchpad-orchestrator/internal/orchestrator"
)

type Server struct {
	cfg    *config.Config
	svc    *orchestrator.Service
	mux    *http.ServeMux
	static fs.FS
}

func NewServer(cfg *config.Config, static fs.FS) *Server {
	logx.Init(cfg.Server.Debug)
	s := &Server{
		cfg:    cfg,
		svc:    orchestrator.NewService(cfg),
		mux:    http.NewServeMux(),
		static: static,
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		withCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.mux.ServeHTTP(w, r)
			return
		}
		if s.static != nil {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			http.FileServer(http.FS(s.static)).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func withCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/config/public", s.handlePublicConfig)
	s.mux.HandleFunc("/api/quote", s.handleQuote)
	s.mux.HandleFunc("/api/tx/inspect", s.handleTxInspect)
	s.mux.HandleFunc("/api/tx/simulate", s.handleTxSimulate)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if err := s.svc.Health(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"debounceMs":         s.cfg.Quote.DebounceMS,
		"defaultSlippageBps": s.cfg.Quote.DefaultSlippageBPS,
		"serviceFeeBps":      s.cfg.ServiceFee.BPS,
		"quoteMints": map[string]string{
			"wsol": s.cfg.Quotes.WSOLMint,
			"usdc": s.cfg.Quotes.USDCMint,
			"usdt": s.cfg.Quotes.USDTMint,
		},
		"jitoEnabled":     s.cfg.Jito.Enabled,
		"jitoTipLamports": s.cfg.Jito.TipLamports,
		"sponsorEnabled":  s.cfg.Sponsor.Enabled,
		"maxTxBytes":     s.cfg.Tx.MaxBytes,
		"priorityTiers":  []string{"low", "medium", "high"},
		"defaultPriorityTier": s.cfg.PriorityFee.DefaultTier,
		"rpcUrl":         s.cfg.Solana.RPCURL,
	})
}

type quoteRequest struct {
	InputMint       string `json:"inputMint"`
	OutputMint      string `json:"outputMint"`
	InputAmount     string `json:"inputAmount"`
	InputAmountRaw  string `json:"inputAmountRaw,omitempty"`
	SlippageBps     int    `json:"slippageBps"`
	UserPubkey      string `json:"userPubkey"`
	RecipientPubkey string `json:"recipientPubkey"`
	PriorityTier    string `json:"priorityTier"`
	InputSettlement  string `json:"inputSettlement"`  // native_sol | wsol_spl | spl
	OutputSettlement string `json:"outputSettlement"` // native_sol | wsol_spl | spl
}

func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	start := time.Now()
	var req quoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	in := orchestrator.QuoteInput{
		InputMint:       strings.TrimSpace(req.InputMint),
		OutputMint:      strings.TrimSpace(req.OutputMint),
		InputAmount:     strings.TrimSpace(req.InputAmount),
		InputAmountRaw:  strings.TrimSpace(req.InputAmountRaw),
		SlippageBPS:     req.SlippageBps,
		UserPubkey:      strings.TrimSpace(req.UserPubkey),
		RecipientPubkey: strings.TrimSpace(req.RecipientPubkey),
		PriorityTier:    strings.TrimSpace(req.PriorityTier),
		InputSettlement:  strings.TrimSpace(req.InputSettlement),
		OutputSettlement: strings.TrimSpace(req.OutputSettlement),
	}
	logx.Info("api", "quote request",
		"inputMint", in.InputMint,
		"outputMint", in.OutputMint,
		"inputAmount", in.InputAmount,
		"inputAmountRaw", in.InputAmountRaw,
		"slippageBps", in.SlippageBPS,
		"user", in.UserPubkey,
		"inputSettlement", in.InputSettlement,
		"outputSettlement", in.OutputSettlement,
	)

	result, err := s.svc.Quote(r.Context(), in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusRequestTimeout
		}
		logx.Error("api", "quote failed",
			"err", err,
			"status", status,
			"ms", logx.Since(start),
			"inputMint", in.InputMint,
			"outputMint", in.OutputMint,
		)
		writeError(w, status, err.Error())
		return
	}
	logx.Info("api", "quote ok",
		"pairClass", result.PairClass,
		"source", result.Source,
		"hops", len(result.Route),
		"build", result.Build != nil,
		"buildSkipped", result.BuildSkippedReason,
		"ms", logx.Since(start),
	)
	if result.BuildError != "" {
		logx.Warn("api", "quote build skipped",
			"reason", result.BuildSkippedReason,
			"err", result.BuildError,
		)
	}
	writeJSON(w, http.StatusOK, result)
}

type txPayload struct {
	Transaction string `json:"transaction"`
}

func (s *Server) handleTxInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req txPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Transaction) == "" {
		writeError(w, http.StatusBadRequest, "transaction is required")
		return
	}
	result, err := s.svc.InspectTransaction(strings.TrimSpace(req.Transaction))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleTxSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req txPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Transaction) == "" {
		writeError(w, http.StatusBadRequest, "transaction is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := s.svc.SimulateTransaction(ctx, strings.TrimSpace(req.Transaction))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

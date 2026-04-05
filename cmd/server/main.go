package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/nakad/cardgames/config"
	"github.com/nakad/cardgames/internal/auth"
	"github.com/nakad/cardgames/internal/engine"
	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/games/teenpatti"
	"github.com/nakad/cardgames/internal/lobby"
	"github.com/nakad/cardgames/internal/store"
	"github.com/nakad/cardgames/internal/wallet"
	"github.com/nakad/cardgames/internal/ws"
)

type Server struct {
	hub       *ws.Hub
	lobby     *lobby.Lobby
	registry  *game.Registry
	auth      *auth.Store
	jwt       *auth.JWTService
	playerMap *lobby.PlayerTableMap
	wallet    *wallet.Service
	store     *store.MemStore
}

func main() {
	// --- Config ---
	cfg := config.Load()

	// --- Setup ---
	registry := game.NewRegistry()
	_ = registry.Register(teenpatti.New())

	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	userStore := auth.NewStore()
	hub := ws.NewHub()
	lob := lobby.NewLobby(registry, cfg.TurnTimeout)
	playerMap := lobby.NewPlayerTableMap()

	// In-memory store for development (swap with PgStore when DB is available)
	memStore := store.NewMemStore()
	walletService := wallet.NewService(memStore, memStore)

	srv := &Server{
		hub:       hub,
		lobby:     lob,
		registry:  registry,
		auth:      userStore,
		jwt:       jwtService,
		playerMap: playerMap,
		wallet:    walletService,
		store:     memStore,
	}

	// Wire WebSocket message handler
	hub.OnMessage = srv.handleClientMessage

	// Wire reconnection handler
	hub.OnReconnect = srv.handleReconnect

	// --- HTTP Routes ---
	mux := http.NewServeMux()

	// Auth endpoints
	mux.HandleFunc("POST /api/register", srv.handleRegister)
	mux.HandleFunc("POST /api/login", srv.handleLogin)

	// Lobby endpoints (authenticated)
	mux.HandleFunc("GET /api/tables", srv.requireAuth(srv.handleListTables))
	mux.HandleFunc("POST /api/tables", srv.requireAuth(srv.handleCreateTable))

	// Wallet endpoints (authenticated)
	mux.HandleFunc("GET /api/wallet/balance", srv.requireAuth(srv.handleGetBalance))
	mux.HandleFunc("GET /api/wallet/transactions", srv.requireAuth(srv.handleGetTransactions))

	// WebSocket endpoint (authenticated via query param token)
	mux.HandleFunc("GET /ws", srv.handleWS)

	addr := ":" + cfg.Port
	log.Printf("Server starting on %s", addr)
	log.Printf("Registered games: %v", registry.List())
	log.Fatal(http.ListenAndServe(addr, mux))
}

// --- Auth HTTP handlers ---

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	user, err := s.auth.Register(req.Username, req.Password)
	if err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}
	// Also create in store for wallet tracking
	_, _ = s.store.CreateUserWithID(r.Context(), user.ID, user.Username, user.Password, user.Balance)
	token, err := s.jwt.GenerateToken(user)
	if err != nil {
		jsonError(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]any{"token": token, "user_id": user.ID, "balance": user.Balance}, http.StatusCreated)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	user, err := s.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := s.jwt.GenerateToken(user)
	if err != nil {
		jsonError(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"token": token, "user_id": user.ID}, http.StatusOK)
}

// --- Lobby HTTP handlers ---

func (s *Server) handleListTables(w http.ResponseWriter, r *http.Request) {
	tables := s.lobby.ListTables()
	jsonResp(w, tables, http.StatusOK)
}

type createTableRequest struct {
	GameType   string `json:"game_type"`
	BootAmount int64  `json:"boot_amount"`
}

func (s *Server) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	var req createTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.BootAmount <= 0 {
		req.BootAmount = 10
	}
	actor, err := s.lobby.CreateTable(req.GameType, req.BootAmount)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Wire broadcast callback
	actor.OnBroadcast = s.broadcastToTable

	// Wire wallet hooks
	actor.Wallet = &engine.WalletHook{
		CollectBoot: func(ctx context.Context, userID string, amount int64, tableID string) error {
			return s.wallet.CollectBoot(ctx, userID, amount, tableID)
		},
		CreditWinnings: func(ctx context.Context, userID string, amount int64, tableID string) error {
			return s.wallet.CreditWinnings(ctx, userID, amount, tableID)
		},
	}

	jsonResp(w, map[string]string{"table_id": actor.Table.ID}, http.StatusCreated)
}

// --- WebSocket handler ---

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := s.jwt.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	s.hub.HandleWebSocket(claims.UserID, claims.Username, w, r)
}

// --- WebSocket message dispatcher ---

func (s *Server) handleClientMessage(clientID string, msg ws.ClientMessage) {
	switch msg.Type {
	case ws.EventJoinTable:
		s.handleJoinTable(clientID, msg.TableID)
	case ws.EventLeaveTable:
		s.handleLeaveTable(clientID, msg.TableID)
	case ws.EventStartGame:
		s.handleStartGame(clientID, msg.TableID)
	case ws.EventPlayerAction:
		s.handlePlayerAction(clientID, msg.TableID, msg.Action)
	default:
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: fmt.Sprintf("unknown event: %s", msg.Type),
		})
	}
}

func (s *Server) handleJoinTable(clientID, tableID string) {
	actor := s.lobby.GetTable(tableID)
	if actor == nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: "table not found",
		})
		return
	}

	reply := actor.Send(engine.TableEvent{
		Type:     "join",
		PlayerID: clientID,
	})
	if reply.Err != nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: reply.Err.Error(),
		})
		return
	}

	s.playerMap.Set(clientID, tableID)
	s.hub.SetClientTable(clientID, tableID)
}

func (s *Server) handleLeaveTable(clientID, tableID string) {
	actor := s.lobby.GetTable(tableID)
	if actor == nil {
		return
	}

	reply := actor.Send(engine.TableEvent{
		Type:     "leave",
		PlayerID: clientID,
	})
	if reply.Err != nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: reply.Err.Error(),
		})
		return
	}

	s.playerMap.Delete(clientID)
	s.hub.SetClientTable(clientID, "")
}

func (s *Server) handleStartGame(clientID, tableID string) {
	actor := s.lobby.GetTable(tableID)
	if actor == nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: "table not found",
		})
		return
	}

	reply := actor.Send(engine.TableEvent{
		Type:     "start",
		PlayerID: clientID,
	})
	if reply.Err != nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: reply.Err.Error(),
		})
	}
}

func (s *Server) handlePlayerAction(clientID, tableID string, action *ws.ClientAction) {
	if action == nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: "no action provided",
		})
		return
	}

	actor := s.lobby.GetTable(tableID)
	if actor == nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: "table not found",
		})
		return
	}

	gameAction := game.Action{
		Type:   action.Type,
		Amount: action.Amount,
	}

	reply := actor.Send(engine.TableEvent{
		Type:     "action",
		PlayerID: clientID,
		Action:   &gameAction,
	})
	if reply.Err != nil {
		s.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: reply.Err.Error(),
		})
	}
}

// broadcastToTable sends actor broadcast messages to the right WebSocket clients.
func (s *Server) broadcastToTable(tableID string, msgs []engine.BroadcastMsg) {
	actor := s.lobby.GetTable(tableID)
	if actor == nil {
		return
	}

	// Build list of player IDs at this table
	playerIDs := make([]string, len(actor.Table.Players))
	for i, p := range actor.Table.Players {
		playerIDs[i] = p.ID
	}

	for _, msg := range msgs {
		serverMsg := ws.ServerMessage{
			Type:    ws.EventType(msg.Type),
			Payload: msg.Payload,
		}

		if msg.TargetID != "" {
			// Private message to one player
			s.hub.SendToClient(msg.TargetID, serverMsg)
		} else {
			// Broadcast to all at table
			s.hub.SendToClients(playerIDs, serverMsg)
		}
	}
}

// --- Auth middleware ---

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			jsonError(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		_, err := s.jwt.ValidateToken(tokenStr)
		if err != nil {
			jsonError(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// --- Reconnection handler ---

func (s *Server) handleReconnect(clientID string) {
	tableID := s.playerMap.Get(clientID)
	if tableID == "" {
		return
	}
	actor := s.lobby.GetTable(tableID)
	if actor == nil {
		s.playerMap.Delete(clientID)
		return
	}
	// Restore client's table association
	s.hub.SetClientTable(clientID, tableID)
	// Send current game state
	reply := actor.Send(engine.TableEvent{Type: "reconnect", PlayerID: clientID})
	if reply.Err == nil && len(reply.Broadcast) > 0 {
		s.broadcastToTable(tableID, reply.Broadcast)
	}
}

// --- Wallet HTTP handler ---

func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	claims := s.claimsFromRequest(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	balance, err := s.wallet.GetBalance(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]any{"user_id": claims.UserID, "balance": balance}, http.StatusOK)
}

func (s *Server) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	claims := s.claimsFromRequest(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	txs, err := s.wallet.GetHistory(r.Context(), claims.UserID, 50)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, txs, http.StatusOK)
}

func (s *Server) claimsFromRequest(r *http.Request) *auth.Claims {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := s.jwt.ValidateToken(tokenStr)
	if err != nil {
		return nil
	}
	return claims
}

// --- JSON helpers ---

func jsonResp(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

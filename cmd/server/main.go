package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nakad/cardgames/internal/auth"
	"github.com/nakad/cardgames/internal/engine"
	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/games/teenpatti"
	"github.com/nakad/cardgames/internal/lobby"
	"github.com/nakad/cardgames/internal/ws"
)

type Server struct {
	hub       *ws.Hub
	lobby     *lobby.Lobby
	registry  *game.Registry
	auth      *auth.Store
	jwt       *auth.JWTService
	playerMap *lobby.PlayerTableMap
}

func main() {
	// --- Setup ---
	registry := game.NewRegistry()
	_ = registry.Register(teenpatti.New())

	jwtService := auth.NewJWTService("teen-patti-secret-change-me", 24*time.Hour)
	userStore := auth.NewStore()
	hub := ws.NewHub()
	lob := lobby.NewLobby(registry, 20*time.Second)
	playerMap := lobby.NewPlayerTableMap()

	srv := &Server{
		hub:       hub,
		lobby:     lob,
		registry:  registry,
		auth:      userStore,
		jwt:       jwtService,
		playerMap: playerMap,
	}

	// Wire WebSocket message handler
	hub.OnMessage = srv.handleClientMessage

	// --- HTTP Routes ---
	mux := http.NewServeMux()

	// Auth endpoints
	mux.HandleFunc("POST /api/register", srv.handleRegister)
	mux.HandleFunc("POST /api/login", srv.handleLogin)

	// Lobby endpoints (authenticated)
	mux.HandleFunc("GET /api/tables", srv.requireAuth(srv.handleListTables))
	mux.HandleFunc("POST /api/tables", srv.requireAuth(srv.handleCreateTable))

	// WebSocket endpoint (authenticated via query param token)
	mux.HandleFunc("GET /ws", srv.handleWS)

	addr := ":8080"
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
	token, err := s.jwt.GenerateToken(user)
	if err != nil {
		jsonError(w, "token generation failed", http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"token": token, "user_id": user.ID}, http.StatusCreated)
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

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nakad/cardgames/internal/auth"
	"github.com/nakad/cardgames/internal/engine"
	"github.com/nakad/cardgames/internal/game"
	"github.com/nakad/cardgames/internal/games/teenpatti"
	"github.com/nakad/cardgames/internal/lobby"
	"github.com/nakad/cardgames/internal/store"
	"github.com/nakad/cardgames/internal/wallet"
	"github.com/nakad/cardgames/internal/ws"
)

// testServer sets up the full server stack for testing.
type testServer struct {
	hub       *ws.Hub
	lobby     *lobby.Lobby
	registry  *game.Registry
	auth      *auth.Store
	jwt       *auth.JWTService
	playerMap *lobby.PlayerTableMap
	store     *store.MemStore
	wallet    *wallet.Service
	httpSrv   *httptest.Server
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	registry := game.NewRegistry()
	_ = registry.Register(teenpatti.New())

	jwtService := auth.NewJWTService("test-secret", 1*time.Hour)
	userStore := auth.NewStore()
	hub := ws.NewHub()
	lob := lobby.NewLobby(registry, 20*time.Second)
	playerMap := lobby.NewPlayerTableMap()
	memStore := store.NewMemStore()
	walletService := wallet.NewService(memStore, memStore)

	ts := &testServer{
		hub:       hub,
		lobby:     lob,
		registry:  registry,
		auth:      userStore,
		jwt:       jwtService,
		playerMap: playerMap,
		store:     memStore,
		wallet:    walletService,
	}

	hub.OnMessage = ts.handleClientMessage
	hub.OnReconnect = ts.handleReconnect

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", ts.handleRegister)
	mux.HandleFunc("POST /api/login", ts.handleLogin)
	mux.HandleFunc("GET /api/tables", ts.handleListTables)
	mux.HandleFunc("POST /api/tables", ts.handleCreateTable)
	mux.HandleFunc("GET /ws", ts.handleWS)

	ts.httpSrv = httptest.NewServer(mux)
	return ts
}

func (ts *testServer) close() {
	ts.httpSrv.Close()
}

// registerUser creates a user and returns their token.
func (ts *testServer) registerUser(t *testing.T, username, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	resp, err := http.Post(ts.httpSrv.URL+"/api/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result["token"]
}

// connectWS dials the WebSocket with the given token.
func (ts *testServer) connectWS(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.httpSrv.URL, "http") + "/ws?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	return conn
}

// createTable via HTTP and return table ID.
func (ts *testServer) createTable(t *testing.T, token, gameType string, boot int64) string {
	t.Helper()
	body := fmt.Sprintf(`{"game_type":%q,"boot_amount":%d}`, gameType, boot)
	req, _ := http.NewRequest("POST", ts.httpSrv.URL+"/api/tables", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create table: expected 201, got %d", resp.StatusCode)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result["table_id"]
}

// sendMsg sends a JSON message over websocket.
func sendMsg(t *testing.T, conn *websocket.Conn, msg ws.ClientMessage) {
	t.Helper()
	data, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatal(err)
	}
}

// collectMessages reads messages from a connection using a goroutine.
// It reads until the done channel is closed, then returns all collected messages.
type msgCollector struct {
	msgs []ws.ServerMessage
	mu   sync.Mutex
	done chan struct{}
}

func newCollector(conn *websocket.Conn) *msgCollector {
	mc := &msgCollector{done: make(chan struct{})}
	go func() {
		for {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg ws.ServerMessage
			json.Unmarshal(data, &msg)
			mc.mu.Lock()
			mc.msgs = append(mc.msgs, msg)
			mc.mu.Unlock()
		}
	}()
	return mc
}

func (mc *msgCollector) messages() []ws.ServerMessage {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	cp := make([]ws.ServerMessage, len(mc.msgs))
	copy(cp, mc.msgs)
	return cp
}

func (mc *msgCollector) hasType(t ws.EventType) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	for _, m := range mc.msgs {
		if m.Type == t {
			return true
		}
	}
	return false
}

// --- HTTP handler duplicates for test server (same logic as cmd/server) ---

func (ts *testServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	user, err := ts.auth.Register(req.Username, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	// Also create in store for wallet tracking
	ts.store.CreateUserWithID(r.Context(), user.ID, user.Username, user.Password, user.Balance)
	token, _ := ts.jwt.GenerateToken(user)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"token": token, "user_id": user.ID})
}

func (ts *testServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	user, err := ts.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	token, _ := ts.jwt.GenerateToken(user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token, "user_id": user.ID})
}

func (ts *testServer) handleListTables(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ts.lobby.ListTables())
}

func (ts *testServer) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GameType   string `json:"game_type"`
		BootAmount int64  `json:"boot_amount"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.BootAmount <= 0 {
		req.BootAmount = 10
	}
	actor, err := ts.lobby.CreateTable(req.GameType, req.BootAmount)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	actor.OnBroadcast = ts.broadcastToTable
	actor.Wallet = &engine.WalletHook{
		CollectBoot: func(ctx context.Context, userID string, amount int64, tableID string) error {
			return ts.wallet.CollectBoot(ctx, userID, amount, tableID)
		},
		CreditWinnings: func(ctx context.Context, userID string, amount int64, tableID string) error {
			return ts.wallet.CreditWinnings(ctx, userID, amount, tableID)
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"table_id": actor.Table.ID})
}

func (ts *testServer) handleWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	claims, err := ts.jwt.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	ts.hub.HandleWebSocket(claims.UserID, claims.Username, w, r)
}

func (ts *testServer) handleClientMessage(clientID string, msg ws.ClientMessage) {
	switch msg.Type {
	case ws.EventJoinTable:
		ts.handleJoin(clientID, msg.TableID)
	case ws.EventLeaveTable:
		ts.handleLeave(clientID, msg.TableID)
	case ws.EventStartGame:
		ts.handleStart(clientID, msg.TableID)
	case ws.EventPlayerAction:
		ts.handleAction(clientID, msg.TableID, msg.Action)
	default:
		ts.hub.SendToClient(clientID, ws.ServerMessage{
			Type:  ws.EventError,
			Error: fmt.Sprintf("unknown event: %s", msg.Type),
		})
	}
}

func (ts *testServer) handleJoin(clientID, tableID string) {
	actor := ts.lobby.GetTable(tableID)
	if actor == nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: "table not found"})
		return
	}
	reply := actor.Send(engine.TableEvent{Type: "join", PlayerID: clientID})
	if reply.Err != nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: reply.Err.Error()})
		return
	}
	ts.playerMap.Set(clientID, tableID)
	ts.hub.SetClientTable(clientID, tableID)
}

func (ts *testServer) handleLeave(clientID, tableID string) {
	actor := ts.lobby.GetTable(tableID)
	if actor == nil {
		return
	}
	reply := actor.Send(engine.TableEvent{Type: "leave", PlayerID: clientID})
	if reply.Err != nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: reply.Err.Error()})
		return
	}
	ts.playerMap.Delete(clientID)
	ts.hub.SetClientTable(clientID, "")
}

func (ts *testServer) handleStart(clientID, tableID string) {
	actor := ts.lobby.GetTable(tableID)
	if actor == nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: "table not found"})
		return
	}
	reply := actor.Send(engine.TableEvent{Type: "start", PlayerID: clientID})
	if reply.Err != nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: reply.Err.Error()})
	}
}

func (ts *testServer) handleAction(clientID, tableID string, action *ws.ClientAction) {
	if action == nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: "no action"})
		return
	}
	actor := ts.lobby.GetTable(tableID)
	if actor == nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: "table not found"})
		return
	}
	gameAction := game.Action{Type: action.Type, Amount: action.Amount}
	reply := actor.Send(engine.TableEvent{Type: "action", PlayerID: clientID, Action: &gameAction})
	if reply.Err != nil {
		ts.hub.SendToClient(clientID, ws.ServerMessage{Type: ws.EventError, Error: reply.Err.Error()})
	}
}

func (ts *testServer) broadcastToTable(tableID string, msgs []engine.BroadcastMsg) {
	actor := ts.lobby.GetTable(tableID)
	if actor == nil {
		return
	}
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
			ts.hub.SendToClient(msg.TargetID, serverMsg)
		} else {
			ts.hub.SendToClients(playerIDs, serverMsg)
		}
	}
}

func (ts *testServer) handleReconnect(clientID string) {
	tableID := ts.playerMap.Get(clientID)
	if tableID == "" {
		return
	}
	actor := ts.lobby.GetTable(tableID)
	if actor == nil {
		ts.playerMap.Delete(clientID)
		return
	}
	ts.hub.SetClientTable(clientID, tableID)
	reply := actor.Send(engine.TableEvent{Type: "reconnect", PlayerID: clientID})
	if reply.Err == nil && len(reply.Broadcast) > 0 {
		ts.broadcastToTable(tableID, reply.Broadcast)
	}
}

// === TESTS ===

func TestRegisterAndLogin(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	// Register
	token := ts.registerUser(t, "alice", "pass123")
	if token == "" {
		t.Fatal("expected token")
	}

	// Duplicate registration fails
	body := `{"username":"alice","password":"pass123"}`
	resp, _ := http.Post(ts.httpSrv.URL+"/api/register", "application/json", strings.NewReader(body))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Login
	resp, _ = http.Post(ts.httpSrv.URL+"/api/login", "application/json",
		strings.NewReader(`{"username":"alice","password":"pass123"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Bad credentials
	resp, _ = http.Post(ts.httpSrv.URL+"/api/login", "application/json",
		strings.NewReader(`{"username":"alice","password":"wrong"}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWebSocketConnect(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	token := ts.registerUser(t, "bob", "pass456")
	conn := ts.connectWS(t, token)
	defer conn.Close()

	// Bad token should fail
	wsURL := "ws" + strings.TrimPrefix(ts.httpSrv.URL, "http") + "/ws?token=invalid"
	_, resp, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token, got %d", resp.StatusCode)
	}
}

func TestCreateAndListTables(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	token := ts.registerUser(t, "charlie", "pass789")
	tableID := ts.createTable(t, token, "teen_patti", 10)
	if tableID == "" {
		t.Fatal("expected table ID")
	}

	// List tables
	req, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/tables", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var tables []lobby.TableInfo
	json.NewDecoder(resp.Body).Decode(&tables)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	if tables[0].ID != tableID {
		t.Fatalf("expected table %s, got %s", tableID, tables[0].ID)
	}
}

func TestFullGameFlow(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	// Register 3 players
	token1 := ts.registerUser(t, "p1", "pass")
	token2 := ts.registerUser(t, "p2", "pass")
	token3 := ts.registerUser(t, "p3", "pass")

	// Create table
	tableID := ts.createTable(t, token1, "teen_patti", 10)

	// Connect WebSockets and start collecting messages immediately
	conn1 := ts.connectWS(t, token1)
	defer conn1.Close()
	coll1 := newCollector(conn1)

	conn2 := ts.connectWS(t, token2)
	defer conn2.Close()
	coll2 := newCollector(conn2)

	conn3 := ts.connectWS(t, token3)
	defer conn3.Close()
	coll3 := newCollector(conn3)

	// All 3 join the table
	for _, conn := range []*websocket.Conn{conn1, conn2, conn3} {
		sendMsg(t, conn, ws.ClientMessage{
			Type:    ws.EventJoinTable,
			TableID: tableID,
		})
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	// Verify all 3 players joined
	actor := ts.lobby.GetTable(tableID)
	if len(actor.Table.Players) != 3 {
		t.Fatalf("expected 3 players at table, got %d", len(actor.Table.Players))
	}

	// Player 1 starts the game
	sendMsg(t, conn1, ws.ClientMessage{
		Type:    ws.EventStartGame,
		TableID: tableID,
	})
	time.Sleep(500 * time.Millisecond)

	if actor.Table.State.String() != "BETTING" {
		t.Fatalf("expected BETTING, got %s", actor.Table.State)
	}

	// All players should have received DEAL_CARDS and TURN_CHANGE
	if !coll1.hasType(ws.EventDealCards) {
		t.Fatalf("player 1 should have received DEAL_CARDS, got: %v", msgTypes(coll1.messages()))
	}
	if !coll1.hasType(ws.EventTurnChange) {
		t.Fatalf("player 1 should have received TURN_CHANGE, got: %v", msgTypes(coll1.messages()))
	}
	if !coll2.hasType(ws.EventDealCards) {
		t.Fatalf("player 2 should have received DEAL_CARDS")
	}
	if !coll3.hasType(ws.EventDealCards) {
		t.Fatalf("player 3 should have received DEAL_CARDS")
	}

	// Two players fold to end the game
	for i := 0; i < 2; i++ {
		if actor.Table.State != 3 { // StateBetting
			break
		}
		gs := actor.Table.GameState
		currentID := gs.ActivePlayers[gs.CurrentTurn]

		var currentConn *websocket.Conn
		for j, tok := range []string{token1, token2, token3} {
			claims, _ := ts.jwt.ValidateToken(tok)
			if claims.UserID == currentID {
				currentConn = []*websocket.Conn{conn1, conn2, conn3}[j]
				break
			}
		}
		if currentConn == nil {
			t.Fatal("couldn't find connection for current player")
		}

		sendMsg(t, currentConn, ws.ClientMessage{
			Type:    ws.EventPlayerAction,
			TableID: tableID,
			Action:  &ws.ClientAction{Type: "fold"},
		})
		time.Sleep(200 * time.Millisecond)
	}

	// The game should have ended
	if actor.Table.State != 5 { // StateFinished
		t.Fatalf("expected FINISHED state, got %s", actor.Table.State)
	}

	// At least one collector should have received GAME_RESULT
	time.Sleep(200 * time.Millisecond)
	if !coll1.hasType(ws.EventGameResult) && !coll2.hasType(ws.EventGameResult) && !coll3.hasType(ws.EventGameResult) {
		t.Fatal("no player received GAME_RESULT")
	}
}

func msgTypes(msgs []ws.ServerMessage) []string {
	types := make([]string, len(msgs))
	for i, m := range msgs {
		types[i] = string(m.Type)
	}
	return types
}

func TestWalletBootCollection(t *testing.T) {
	memStore := store.NewMemStore()
	walletService := wallet.NewService(memStore, memStore)
	ctx := context.Background()

	// Create users with known balance
	memStore.CreateUser(ctx, "u1", "hash", 1000)
	memStore.CreateUser(ctx, "u2", "hash", 1000)
	memStore.CreateUser(ctx, "u3", "hash", 50)

	u1, _ := memStore.GetByUsername(ctx, "u1")
	u2, _ := memStore.GetByUsername(ctx, "u2")
	u3, _ := memStore.GetByUsername(ctx, "u3")

	// Collect boot
	if err := walletService.CollectBoot(ctx, u1.ID, 100, "table-1"); err != nil {
		t.Fatalf("boot collect u1: %v", err)
	}
	if err := walletService.CollectBoot(ctx, u2.ID, 100, "table-1"); err != nil {
		t.Fatalf("boot collect u2: %v", err)
	}

	// u3 doesn't have enough — should fail
	if err := walletService.CollectBoot(ctx, u3.ID, 100, "table-1"); err == nil {
		t.Fatal("expected insufficient balance error for u3")
	}

	// Check balances
	b1, _ := walletService.GetBalance(ctx, u1.ID)
	b2, _ := walletService.GetBalance(ctx, u2.ID)
	b3, _ := walletService.GetBalance(ctx, u3.ID)

	if b1 != 900 {
		t.Fatalf("u1 balance: expected 900, got %d", b1)
	}
	if b2 != 900 {
		t.Fatalf("u2 balance: expected 900, got %d", b2)
	}
	if b3 != 50 {
		t.Fatalf("u3 balance should remain 50, got %d", b3)
	}

	// Credit winnings
	if err := walletService.CreditWinnings(ctx, u1.ID, 300, "table-1"); err != nil {
		t.Fatalf("credit u1: %v", err)
	}
	b1, _ = walletService.GetBalance(ctx, u1.ID)
	if b1 != 1200 {
		t.Fatalf("u1 after win: expected 1200, got %d", b1)
	}

	// Check transaction history
	txs, _ := walletService.GetHistory(ctx, u1.ID, 10)
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions for u1, got %d", len(txs))
	}
}

func TestMemStore(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemStore()

	// Create user
	u, err := s.CreateUser(ctx, "testuser", "hashedpw", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if u.Balance != 5000 {
		t.Fatalf("expected balance 5000, got %d", u.Balance)
	}

	// Duplicate should fail
	_, err = s.CreateUser(ctx, "testuser", "hash2", 1000)
	if err == nil {
		t.Fatal("expected duplicate error")
	}

	// Lookup by username
	found, err := s.GetByUsername(ctx, "testuser")
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != u.ID {
		t.Fatal("ID mismatch")
	}

	// Lookup by ID
	found2, err := s.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found2.Username != "testuser" {
		t.Fatal("username mismatch")
	}

	// Debit
	newBal, err := s.DebitWithCheck(ctx, u.ID, 1000, "bet", "table-x")
	if err != nil {
		t.Fatal(err)
	}
	if newBal != 4000 {
		t.Fatalf("expected 4000 after debit, got %d", newBal)
	}

	// Debit more than balance should fail
	_, err = s.DebitWithCheck(ctx, u.ID, 5000, "bet", "table-x")
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}

	// Credit
	newBal, err = s.Credit(ctx, u.ID, 2000, "win", "table-x")
	if err != nil {
		t.Fatal(err)
	}
	if newBal != 6000 {
		t.Fatalf("expected 6000 after credit, got %d", newBal)
	}

	// Transaction history
	txs, err := s.GetTransactions(ctx, u.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 txs, got %d", len(txs))
	}
}

func TestReconnection(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	// Register and connect
	token1 := ts.registerUser(t, "reconnP1", "pass")
	token2 := ts.registerUser(t, "reconnP2", "pass")
	token3 := ts.registerUser(t, "reconnP3", "pass")

	tableID := ts.createTable(t, token1, "teen_patti", 10)

	// Connect all 3
	conn1 := ts.connectWS(t, token1)
	coll1 := newCollector(conn1)
	conn2 := ts.connectWS(t, token2)
	newCollector(conn2)
	conn3 := ts.connectWS(t, token3)
	newCollector(conn3)

	// All join
	for _, conn := range []*websocket.Conn{conn1, conn2, conn3} {
		sendMsg(t, conn, ws.ClientMessage{Type: ws.EventJoinTable, TableID: tableID})
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	// Start game
	sendMsg(t, conn1, ws.ClientMessage{Type: ws.EventStartGame, TableID: tableID})
	time.Sleep(500 * time.Millisecond)

	actor := ts.lobby.GetTable(tableID)
	if actor.Table.State.String() != "BETTING" {
		t.Fatalf("expected BETTING, got %s", actor.Table.State)
	}

	// Verify p1 got DEAL_CARDS
	if !coll1.hasType(ws.EventDealCards) {
		t.Fatal("p1 should have received DEAL_CARDS before disconnect")
	}

	// Disconnect player 1 (simulate disconnect)
	conn1.Close()
	time.Sleep(200 * time.Millisecond)

	// Reconnect player 1
	reconnConn := ts.connectWS(t, token1)
	defer reconnConn.Close()
	reconnColl := newCollector(reconnConn)
	time.Sleep(500 * time.Millisecond)

	// Should receive TABLE_STATE with current game state
	if !reconnColl.hasType(ws.EventTableState) {
		t.Fatalf("reconnected player should have received TABLE_STATE, got: %v", msgTypes(reconnColl.messages()))
	}

	// Clean up other connections
	conn2.Close()
	conn3.Close()
}

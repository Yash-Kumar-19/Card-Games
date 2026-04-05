package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // tighten in production
	},
}

// Client represents a connected WebSocket client.
type Client struct {
	ID      string
	Name    string
	conn    *websocket.Conn
	hub     *Hub
	send    chan []byte
	mu      sync.Mutex
	tableID string // current table this client is at
}

// Hub manages all connected WebSocket clients.
type Hub struct {
	clients   map[string]*Client // playerID -> Client
	mu        sync.RWMutex
	OnMessage func(clientID string, msg ClientMessage)
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.ID] = c
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[clientID]; ok {
		close(c.send)
		delete(h.clients, clientID)
	}
}

// GetClient returns a client by ID.
func (h *Hub) GetClient(id string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[id]
}

// SendToClient sends a message to a specific client.
func (h *Hub) SendToClient(clientID string, msg ServerMessage) {
	h.mu.RLock()
	c, ok := h.clients[clientID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		log.Printf("client %s send buffer full, dropping message", clientID)
	}
}

// SendToClients sends a message to a list of client IDs.
func (h *Hub) SendToClients(clientIDs []string, msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, id := range clientIDs {
		if c, ok := h.clients[id]; ok {
			select {
			case c.send <- data:
			default:
				log.Printf("client %s send buffer full, dropping message", id)
			}
		}
	}
}

// SetClientTable sets which table a client is currently at.
func (h *Hub) SetClientTable(clientID, tableID string) {
	h.mu.RLock()
	c, ok := h.clients[clientID]
	h.mu.RUnlock()
	if ok {
		c.mu.Lock()
		c.tableID = tableID
		c.mu.Unlock()
	}
}

// GetClientTable returns the table ID a client is at.
func (h *Hub) GetClientTable(clientID string) string {
	h.mu.RLock()
	c, ok := h.clients[clientID]
	h.mu.RUnlock()
	if !ok {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tableID
}

// HandleWebSocket upgrades an HTTP connection to WebSocket.
func (h *Hub) HandleWebSocket(playerID, playerName string, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:   playerID,
		Name: playerName,
		conn: conn,
		hub:  h,
		send: make(chan []byte, 256),
	}

	h.Register(client)

	go client.writePump()
	go client.readPump()
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c.ID)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error from %s: %v", c.ID, err)
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("invalid message from %s: %v", c.ID, err)
			continue
		}

		if c.hub.OnMessage != nil {
			c.hub.OnMessage(c.ID, msg)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

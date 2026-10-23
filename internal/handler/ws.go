package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type GeoLocation struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Accuracy float64 `json:"accuracy,omitempty"`
}

type WSClient struct {
	ID       string      `json:"id"`
	UserID   string      `json:"user_id"`
	Name     string      `json:"name"`
	Room     string      `json:"room"`
	Location GeoLocation `json:"location"`
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *WSHub
	mu       sync.Mutex
}

type WSMessage struct {
	Type     string      `json:"type"`
	Room     string      `json:"room,omitempty"`
	UserID   string      `json:"user_id,omitempty"`
	Name     string      `json:"name,omitempty"`
	Location GeoLocation `json:"location,omitempty"`
	Payload  interface{} `json:"payload,omitempty"`
}

type WSHub struct {
	clients    map[*WSClient]bool
	rooms      map[string]map[*WSClient]bool
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan *wsEnvelope
	mu         sync.RWMutex
	upgrader   websocket.Upgrader
}

type wsEnvelope struct {
	room string
	data []byte
	skip *WSClient
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		rooms:      make(map[string]map[*WSClient]bool),
		register:   make(chan *WSClient, 256),
		unregister: make(chan *WSClient, 256),
		broadcast:  make(chan *wsEnvelope, 256),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			if h.rooms[c.Room] == nil {
				h.rooms[c.Room] = make(map[*WSClient]bool)
			}
			h.rooms[c.Room][c] = true
			h.mu.Unlock()
			h.broadcastPresence(c.Room)
			h.sendSnapshot(c)
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				if room, ok := h.rooms[c.Room]; ok {
					delete(room, c)
					if len(room) == 0 {
						delete(h.rooms, c.Room)
					}
				}
			}
			h.mu.Unlock()
			close(c.Send)
			h.broadcastPresence(c.Room)
		case env := <-h.broadcast:
			h.mu.RLock()
			room := h.rooms[env.room]
			h.mu.RUnlock()
			for c := range room {
				if c == env.skip {
					continue
				}
				select {
				case c.Send <- env.data:
				default:
					h.unregister <- c
				}
			}
		}
	}
}

func (h *WSHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		room = "global"
	}
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-User-Id")
	}
	if userID == "" {
		userID = uuid.New().String()
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "Guest"
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := &WSClient{
		ID:     uuid.New().String(),
		UserID: userID,
		Name:   name,
		Room:   room,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    h,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *WSClient) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(65536)
	c.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})
	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var m WSMessage
		if err := json.Unmarshal(msg, &m); err != nil {
			continue
		}
		if m.Type == "location" {
			c.mu.Lock()
			c.Location = m.Location
			c.mu.Unlock()
			out := WSMessage{
				Type:     "location",
				Room:     c.Room,
				UserID:   c.UserID,
				Name:     c.Name,
				Location: m.Location,
			}
			b, _ := json.Marshal(out)
			c.Hub.broadcast <- &wsEnvelope{room: c.Room, data: b}
		} else if m.Type == "join" {
			if m.Room != "" && m.Room != c.Room {
				c.Hub.unregister <- c
				c.Room = m.Room
				c.Hub.register <- c
			}
		} else {
			m.UserID = c.UserID
			m.Name = c.Name
			m.Room = c.Room
			b, _ := json.Marshal(m)
			c.Hub.broadcast <- &wsEnvelope{room: c.Room, data: b, skip: c}
		}
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WSHub) broadcastPresence(room string) {
	h.mu.RLock()
	clients := h.rooms[room]
	type user struct {
		ID       string      `json:"id"`
		UserID   string      `json:"user_id"`
		Name     string      `json:"name"`
		Location GeoLocation `json:"location"`
	}
	var users []user
	for c := range clients {
		c.mu.Lock()
		loc := c.Location
		c.mu.Unlock()
		users = append(users, user{ID: c.ID, UserID: c.UserID, Name: c.Name, Location: loc})
	}
	h.mu.RUnlock()
	msg := WSMessage{Type: "presence", Room: room, Payload: users}
	b, _ := json.Marshal(msg)
	h.broadcast <- &wsEnvelope{room: room, data: b}
}

func (h *WSHub) sendSnapshot(c *WSClient) {
	h.mu.RLock()
	clients := h.rooms[c.Room]
	type user struct {
		ID       string      `json:"id"`
		UserID   string      `json:"user_id"`
		Name     string      `json:"name"`
		Location GeoLocation `json:"location"`
	}
	var users []user
	for cl := range clients {
		cl.mu.Lock()
		loc := cl.Location
		cl.mu.Unlock()
		users = append(users, user{ID: cl.ID, UserID: cl.UserID, Name: cl.Name, Location: loc})
	}
	h.mu.RUnlock()
	msg := WSMessage{Type: "snapshot", Room: c.Room, Payload: users}
	b, _ := json.Marshal(msg)
	select {
	case c.Send <- b:
	default:
	}
}

func (h *WSHub) Broadcast(msg []byte) {
	// broadcast to all rooms
	h.mu.RLock()
	rooms := make([]string, 0, len(h.rooms))
	for r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.RUnlock()
	for _, r := range rooms {
		h.broadcast <- &wsEnvelope{room: r, data: msg}
	}
}

// Gin adapter
func (h *WSHub) HandleWSGin(c *gin.Context) {
	h.HandleWS(c.Writer, c.Request)
}

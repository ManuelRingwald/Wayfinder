package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/manuelringwald/wayfinder/pkg/broadcast"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingInterval is the interval at which ping messages are sent to the peer.
	pingInterval = (pongWait * 9) / 10

	// messageBufferSize is the size of the client's message send channel.
	messageBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// For now, accept all origins. In production, restrict to known hosts.
		return true
	},
}

// Handler is an HTTP handler for WebSocket connections.
type Handler struct {
	broadcaster *broadcast.Broadcaster
	logger      *slog.Logger
}

// New creates a new WebSocket handler.
func New(broadcaster *broadcast.Broadcaster, logger *slog.Logger) *Handler {
	return &Handler{
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// ServeHTTP upgrades HTTP connections to WebSocket and manages the client.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("upgrade websocket", slog.String("error", err.Error()))
		return
	}

	// Create a send channel for this client.
	sendChan := make(chan broadcast.Message, messageBufferSize)

	// Register the client with the broadcaster.
	client := h.broadcaster.RegisterClient(sendChan)

	h.logger.Debug("websocket client connected", slog.String("remote", r.RemoteAddr))

	// Start the client's read and write loops.
	go h.readLoop(conn, client)
	go h.writeLoop(conn, sendChan)
}

// readLoop reads messages from the client (for future use, e.g., ping/pong).
func (h *Handler) readLoop(conn *websocket.Conn, client *broadcast.Client) {
	defer func() {
		h.broadcaster.UnregisterClient(client)
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("websocket read error", slog.String("error", err.Error()))
			}
			break
		}
		// Currently, we don't expect messages from clients.
		// Future: support client commands, subscriptions, etc.
	}
}

// writeLoop sends messages to the client.
func (h *Handler) writeLoop(conn *websocket.Conn, sendChan <-chan broadcast.Message) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case msg, ok := <-sendChan:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Broadcaster has closed the channel.
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteJSON(msg); err != nil {
				h.logger.Error("websocket write", slog.String("error", err.Error()))
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

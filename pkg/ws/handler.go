package ws

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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

// ScopeResolver resolves a connecting request to the tenant scope that filters
// its track stream (WF2-21). It runs at the handshake, with the tenant Identity
// already in the request context (the tenant middleware gates /ws). Returning an
// error rejects the connection fail-closed. Production always wires a resolver
// (ADR 0014); a nil resolver leaves the client's scope nil, which the broadcaster
// treats fail-closed (the client receives no feed), never as a passthrough.
type ScopeResolver func(r *http.Request) (*broadcast.Scope, error)

// Handler is an HTTP handler for WebSocket connections.
type Handler struct {
	broadcaster    *broadcast.Broadcaster
	logger         *slog.Logger
	allowedOrigins []string
	scopeResolver  ScopeResolver
	upgrader       websocket.Upgrader
}

// New creates a new WebSocket handler. allowedOrigins is an additional
// allowlist of origins (scheme://host[:port]) permitted to open a WebSocket
// connection, beyond same-origin requests, which are always allowed (ADR
// 0003: fail-closed origin check on /ws). scopeResolver resolves the per-tenant
// track scope at connect (WF2-21); production always passes one (ADR 0014). A nil
// resolver leaves the scope nil, which the broadcaster treats fail-closed.
func New(broadcaster *broadcast.Broadcaster, logger *slog.Logger, allowedOrigins []string, scopeResolver ScopeResolver) *Handler {
	h := &Handler{
		broadcaster:    broadcaster,
		logger:         logger,
		allowedOrigins: allowedOrigins,
		scopeResolver:  scopeResolver,
	}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}
	return h
}

// checkOrigin rejects cross-site WebSocket connection attempts (CSWSH).
// Requests without an Origin header (non-browser clients) and same-origin
// requests are always allowed; cross-origin requests are allowed only if
// the Origin is present in allowedOrigins.
func (h *Handler) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		h.logger.Warn("websocket upgrade rejected: invalid Origin header", slog.String("origin", origin))
		return false
	}

	if strings.EqualFold(originURL.Host, r.Host) {
		return true
	}

	for _, allowed := range h.allowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}

	h.logger.Warn("websocket upgrade rejected: origin not allowed",
		slog.String("origin", origin), slog.String("host", r.Host))
	return false
}

// ServeHTTP upgrades HTTP connections to WebSocket and manages the client.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Resolve the tenant scope before upgrading, so a failure can still return an
	// HTTP status. Fail-closed: no resolved scope ⇒ no stream (WF2-21).
	var scope *broadcast.Scope
	if h.scopeResolver != nil {
		s, err := h.scopeResolver(r)
		if err != nil {
			h.logger.Warn("websocket scope resolution failed", slog.String("error", err.Error()))
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		scope = s
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("upgrade websocket", slog.String("error", err.Error()))
		return
	}

	// Create a send channel for this client.
	sendChan := make(chan broadcast.Message, messageBufferSize)

	// Register the client with the broadcaster, scoped to its tenant's feeds.
	client := h.broadcaster.RegisterClient(sendChan, scope)

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

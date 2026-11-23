package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type WSMessageType string

const (
	MessageJoin WSMessageType = "join"
)

type WSMessage struct {
	Type    WSMessageType   `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

const (
	writeWait            = 5 * time.Second
	pongWait             = 60 * time.Second
	pingPeriod           = pongWait * 9 / 10
	QueueHeartbeatMaxAge = pongWait + 15*time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err != nil && u.Hostname() == "localhost" // todo change later to env variable
	},
}

func closeWith(conn *websocket.Conn, code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(writeWait))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	playerId := playerID(r)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "player_id", playerId, "err", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	joined, err := s.JoinQueue(r.Context(), playerId)
	if err != nil {
		slog.Error("join queue failed", "player_id", playerId, "err", err)
		closeWith(conn, websocket.CloseInternalServerErr, "internal error")
		return
	}

	if !joined {
		closeWith(conn, websocket.ClosePolicyViolation, "already in queue")
		return
	}

	defer func() {
		leaveContext, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		s.LeaveQueue(leaveContext, playerId)
	}()

	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))

		heartbeatContext, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()

		err := s.Heartbeat(heartbeatContext, playerId)

		if err != nil {
			slog.Error("Heartbeat failed", "player_id", playerId)
			return err
		}

		return nil
	})

	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteJSON(WSMessage{Type: MessageJoin}); err != nil {
		return
	}
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Closing connection", "playerId", playerId)
			closeWith(conn, websocket.CloseNormalClosure, "")
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

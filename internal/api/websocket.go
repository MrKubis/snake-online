package api

import (
	"context"
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
	Type WSMessageType `json:"type"`
}

const (
	writeWait = 5 * time.Second
	pongWait  = 60 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// check if sent via browser
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
	}

	defer func() {
		leaveContext, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		s.LeaveQueue(leaveContext, playerId)
	}()

	setupHeartBeat(conn)

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteJSON(WSMessage{Type: MessageJoin}); err != nil {
		return
	}

	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func setupHeartBeat(conn *websocket.Conn) {
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
}

package terminal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/api"
)

func EstablishConnection(c *gin.Context) {
	session_id := c.Query("id")
	TerminalSessionsMutex.Lock()
	session, exists := TerminalSessions[session_id]
	TerminalSessionsMutex.Unlock()
	if !exists || session == nil || session.Browser == nil {
		c.JSON(404, gin.H{"status": "error", "error": "Session not found"})
		return
	}
	// Upgrade the connection to WebSocket
	if !api.IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	conn, err := api.UpgradeSafeConn(c)
	if err != nil {
		TerminalSessionsMutex.Lock()
		if TerminalSessions[session_id] == session {
			delete(TerminalSessions, session_id)
		}
		TerminalSessionsMutex.Unlock()
		if session.Browser != nil {
			session.Browser.Close()
		}
		return
	}
	TerminalSessionsMutex.Lock()
	if TerminalSessions[session_id] != session || session.Agent != nil {
		TerminalSessionsMutex.Unlock()
		conn.Close()
		return
	}
	session.Agent = conn
	TerminalSessionsMutex.Unlock()
	conn.SetCloseHandler(func(code int, text string) error {
		TerminalSessionsMutex.Lock()
		if TerminalSessions[session_id] == session {
			delete(TerminalSessions, session_id)
		}
		TerminalSessionsMutex.Unlock()
		// 通知 Browser 关闭终端连接
		if session.Browser != nil {
			session.Browser.Close()
		}
		return nil
	})
	go ForwardTerminal(session_id)
}

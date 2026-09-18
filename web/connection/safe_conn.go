package connection

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SafeConn serializes WebSocket writes so concurrent callers cannot interleave frames.
type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
	ID   int64
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{
		conn: conn,
		mu:   sync.Mutex{},
		ID:   time.Now().UnixNano(),
	}
}

func (sc *SafeConn) writeFrame(messageType int, data []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteMessage(messageType, data)
}

func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	return sc.writeFrame(messageType, data)
}

// WriteJSON encodes v and sends it as one serialized text frame.
func (sc *SafeConn) WriteJSON(v interface{}) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return err
	}
	return sc.writeFrame(websocket.TextMessage, buf.Bytes())
}

func (sc *SafeConn) Close() error {
	return sc.conn.Close()
}

func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	return sc.conn.ReadMessage()
}

func (sc *SafeConn) ReadJSON(v interface{}) error {
	_, data, err := sc.conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (sc *SafeConn) SetReadDeadline(t time.Time) error {
	return sc.conn.SetReadDeadline(t)
}

func (sc *SafeConn) GetConn() *websocket.Conn {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn
}

// SetCloseHandler forwards to the underlying connection (used by the
// terminal sessions to clean up on close).
func (sc *SafeConn) SetCloseHandler(h func(code int, text string) error) {
	sc.conn.SetCloseHandler(h)
}

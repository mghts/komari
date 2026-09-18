package connection

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// wsEchoServer upgrades every request and echoes each frame back.
func wsEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	var upgrader websocket.Upgrader
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}))
}

func dialEcho(t *testing.T, server *httptest.Server) *SafeConn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewSafeConn(conn)
}

func TestSafeConnPreservesFramesAndJSON(t *testing.T) {
	server := wsEchoServer(t)
	defer server.Close()
	sc := dialEcho(t, server)
	defer sc.Close()
	_ = sc.SetReadDeadline(time.Now().Add(5 * time.Second))
	for _, kind := range []int{websocket.TextMessage, websocket.BinaryMessage} {
		if err := sc.WriteMessage(kind, []byte("plain")); err != nil {
			t.Fatal(err)
		}
		gotKind, data, err := sc.ReadMessage()
		if err != nil || gotKind != kind || string(data) != "plain" {
			t.Fatalf("frame changed: %d %q %v", gotKind, data, err)
		}
	}
	if err := sc.WriteJSON(map[string]string{"message": "hello"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := sc.ReadJSON(&got); err != nil || got["message"] != "hello" {
		t.Fatalf("JSON changed: %v %v", got, err)
	}
	if err := sc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sc.ReadMessage(); err == nil {
		t.Fatal("closed connection remained readable")
	}
}

func TestSafeConnConcurrentWritesRemainComplete(t *testing.T) {
	server := wsEchoServer(t)
	defer server.Close()
	sc := dialEcho(t, server)
	defer sc.Close()
	_ = sc.SetReadDeadline(time.Now().Add(5 * time.Second))
	const count = 40
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var err error
			if id%2 == 0 {
				err = sc.WriteJSON(map[string]int{"id": id})
			} else {
				err = sc.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":%d}`, id)))
			}
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	seen := map[int]bool{}
	for i := 0; i < count; i++ {
		kind, data, err := sc.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var item struct {
			ID int `json:"id"`
		}
		if kind != websocket.TextMessage || json.Unmarshal(data, &item) != nil || seen[item.ID] {
			t.Fatalf("invalid or duplicate frame: %q", data)
		}
		seen[item.ID] = true
	}
	wg.Wait()
}

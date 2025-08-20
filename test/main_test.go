package test

import (
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocketConnection(t *testing.T) {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8888/ws", nil)
	if err != nil {
		t.Fatalf("failed to connect ws server: %v\n", err)
	}
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte("111"))
	if err != nil {
		t.Fatalf("failed to send message: %v\n", err)
	}

	_, resp, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v\n", err)
	}
	t.Log(string(resp))
}

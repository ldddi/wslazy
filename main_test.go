package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// 建立 WebSocket 测试连接
func connectWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + url[4:] // http://xxx 转换成 ws://xxx
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("连接 WebSocket 失败: %v", err)
	}
	return conn
}

// 启动带广播的 Hub
func startTestServer(hub *Hub) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r)
	}))
}

// 启动定时广播（用于测试）
func startTicker(hub *Hub, duration time.Duration) {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				message := fmt.Sprintf("服务器时间: %s", time.Now().Format("15:04:05"))
				hub.broadcast <- []byte(message)
			}
		}
	}()
}

// Test 1: 基本广播测试
func TestWebSocketBroadcast(t *testing.T) {
	hub := newHub()
	go hub.run()

	srv := startTestServer(hub)
	defer srv.Close()

	clientA := connectWS(t, srv.URL)
	defer clientA.Close()

	clientB := connectWS(t, srv.URL)
	defer clientB.Close()

	testMsg := "hello from A"
	err := clientA.WriteMessage(websocket.TextMessage, []byte(testMsg))
	assert.NoError(t, err)

	clientB.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := clientB.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, testMsg, string(msg))
}

// Test 2: 客户端断开连接后 Hub 中应移除
func TestClientDisconnect(t *testing.T) {
	hub := newHub()
	go hub.run()

	srv := startTestServer(hub)
	defer srv.Close()

	client := connectWS(t, srv.URL)
	time.Sleep(100 * time.Millisecond) // 等待注册
	assert.Equal(t, 1, len(hub.clients))

	client.Close()
	time.Sleep(100 * time.Millisecond) // 等待注销

	assert.Equal(t, 0, len(hub.clients))
}

// Test 3: 定时广播是否被客户端接收到
func TestPeriodicBroadcast(t *testing.T) {
	hub := newHub()
	go hub.run()
	startTicker(hub, 300*time.Millisecond)

	srv := startTestServer(hub)
	defer srv.Close()

	conn := connectWS(t, srv.URL)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	_, msg, err := conn.ReadMessage()
	assert.NoError(t, err)
	assert.Contains(t, string(msg), "服务器时间:")
}

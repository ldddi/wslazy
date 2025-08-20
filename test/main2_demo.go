package test

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket升级器配置
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源（生产环境需要更严格的检查）
		return true
	},
}

// 客户端连接结构
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub管理所有客户端连接
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("客户端已连接，当前连接数: %d", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("客户端已断开，当前连接数: %d", len(h.clients))
			}

		case message := <-h.broadcast:
			// 广播消息给所有客户端
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// 处理WebSocket连接
func handleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接为WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	// 创建客户端
	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	// 注册客户端
	hub.register <- client

	// 启动读写协程
	go client.writePump(hub)
	client.readPump(hub) // 主协程运行readPump，保持连接
}

// 读取客户端消息
func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	// 设置读取超时
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket错误: %v", err)
			}
			break
		}

		log.Printf("收到消息: %s", message)

		// 广播消息给所有客户端
		hub.broadcast <- message
	}
}

// 写入消息到客户端
func (c *Client) writePump(hub *Hub) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("写入消息失败: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// 简单的客户端示例
func clientExample() {
	// 连接到WebSocket服务器
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		log.Fatal("连接失败:", err)
	}
	defer conn.Close()

	// 发送消息
	err = conn.WriteMessage(websocket.TextMessage, []byte("Hello from client!"))
	if err != nil {
		log.Printf("发送消息失败: %v", err)
		return
	}

	// 读取消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("读取消息失败: %v", err)
			break
		}
		fmt.Printf("收到消息: %s\n", message)
	}
}

// 提供静态HTML页面测试
func homePage(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket测试</title>
</head>
<body>
    <div>
        <input type="text" id="messageInput" placeholder="输入消息...">
        <button onclick="sendMessage()">发送</button>
    </div>
    <div id="messages"></div>

    <script>
        const ws = new WebSocket('ws://localhost:8080/ws');
        const messages = document.getElementById('messages');

        ws.onopen = function(event) {
            const div = document.createElement('div');
            div.textContent = '连接已建立';
            div.style.color = 'green';
            messages.appendChild(div);
        };

        ws.onmessage = function(event) {
            const div = document.createElement('div');
            div.textContent = '收到: ' + event.data;
            messages.appendChild(div);
        };

        ws.onclose = function(event) {
            const div = document.createElement('div');
            div.textContent = '连接已关闭: ' + event.code + ' ' + event.reason;
            div.style.color = 'red';
            messages.appendChild(div);
        };

        ws.onerror = function(error) {
            const div = document.createElement('div');
            div.textContent = '连接错误: ' + error;
            div.style.color = 'red';
            messages.appendChild(div);
        };

        function sendMessage() {
            const input = document.getElementById('messageInput');
            if (input.value) {
                ws.send(input.value);
                input.value = '';
            }
        }

        document.getElementById('messageInput').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func main_demo() {
	// 创建Hub
	hub := newHub()
	go hub.run()

	// 路由设置
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r)
	})

	// 启动定时广播（可选）
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				message := fmt.Sprintf("服务器时间: %s", time.Now().Format("15:04:05"))
				hub.broadcast <- []byte(message)
			}
		}
	}()

	log.Println("WebSocket服务器启动在 :8080")
	log.Println("访问 http://localhost:8080 测试")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	fmt.Println("conn")

	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Expected WebSocket upgrade request", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, r.Header)
	if err != nil {
		log.Printf("failed to upgrade: %v\n", err)
		return
	}

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("failed to read message: %v\n", err)
			break
		}
		fmt.Println(messageType)
		fmt.Println(string(message))
		conn.WriteMessage(websocket.TextMessage, []byte("222"))
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleWebSocket)

	http.ListenAndServe(":8888", mux)
}

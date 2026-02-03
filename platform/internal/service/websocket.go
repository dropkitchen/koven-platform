package service

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Send pings to peer with this period to keep connection alive
	pingPeriod = 54 * time.Second
)

// Client represents a single WebSocket connection
type Client struct {
	// The WebSocket connection
	conn *websocket.Conn

	// The hub this client belongs to
	hub *Hub

	// Buffered channel of outbound messages
	send chan []byte
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
}

// writePump sends messages and pings to the WebSocket connection
// This is the only goroutine needed since communication is unidirectional (server -> client)
// It detects disconnections through write errors
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if err := c.conn.Close(); err != nil {
			log.Printf("WebSocket close error: %v", err)
		}
		c.hub.unregister <- c
		log.Printf("WebSocket client closed")
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("WebSocket write deadline error: %v", err)
				return
			}
			if !ok {
				// The hub closed the channel
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("WebSocket close message error: %v", err)
				}
				return
			}

			// Write the message as JSON text
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			// Send periodic ping to keep connection alive and detect disconnections
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("WebSocket write deadline error: %v", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("WebSocket ping error: %v", err)
				return
			}
		}
	}
}

// Start begins the write pump for this client
func (c *Client) Start() {
	go c.writePump()
}

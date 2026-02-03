package service

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/dropkitchen/koven-platform/platform/internal/protocol"
)

// EventMessage represents a JSON-formatted event message sent to WebSocket clients
type EventMessage struct {
	Type                  string `json:"type"`
	State                 string `json:"state"`
	CurrentTemperature    string `json:"current_temperature"`
	RemainingTime         string `json:"remaining_time"`
	ProgrammedDuration    string `json:"programmed_duration"`
	ProgrammedTemperature string `json:"programmed_temperature"`
}

// Hub maintains the set of active WebSocket clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from MQTT to broadcast to clients
	broadcast chan *protocol.EventPayload

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Shutdown channel to stop the hub
	shutdown chan struct{}

	// Done channel to signal shutdown completion
	done chan struct{}

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *protocol.EventPayload, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shutdown:   make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	defer close(h.done)

	for {
		select {
		case <-h.shutdown:
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			log.Printf("WebSocket hub shut down, all clients disconnected")
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			// Convert protocol event to JSON message
			message := &EventMessage{
				Type:                  "event",
				State:                 protocol.StateToString(event.State),
				CurrentTemperature:    formatTemperature(event.CurrentTemperature),
				RemainingTime:         formatTime(event.RemainingTime),
				ProgrammedDuration:    formatProgrammedTime(event.ProgrammedDuration),
				ProgrammedTemperature: formatProgrammedTemperature(event.ProgrammedTemperature),
			}

			jsonData, err := json.Marshal(message)
			if err != nil {
				log.Printf("Failed to marshal event to JSON: %v", err)
				continue
			}

			// Broadcast to all connected clients
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- jsonData:
				default:
					// Client's send buffer is full, close it
					close(client.send)
					delete(h.clients, client)
					log.Printf("WebSocket client buffer full, disconnecting")
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastEvent sends an event to all connected WebSocket clients
func (h *Hub) BroadcastEvent(event *protocol.EventPayload) {
	select {
	case h.broadcast <- event:
	default:
		log.Printf("Warning: broadcast channel full, dropping event")
	}
}

// RegisterClient registers a new WebSocket client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// GetClientCount returns the number of connected WebSocket clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// formatTemperature formats current temperature, showing "--" for invalid values
func formatTemperature(temp int16) string {
	if temp < 0 {
		return "--"
	}
	return fmt.Sprintf("%d°C", temp)
}

// formatTime formats remaining time, showing "--" for invalid values
func formatTime(time int16) string {
	if time < 0 {
		return "--"
	}
	return fmt.Sprintf("%ds", time)
}

// formatProgrammedTemperature formats programmed temperature, showing "Not set" for invalid values
func formatProgrammedTemperature(temp int16) string {
	if temp < 0 {
		return "Not set"
	}
	return fmt.Sprintf("%d°C", temp)
}

// formatProgrammedTime formats programmed duration, showing "Not set" for invalid values
func formatProgrammedTime(time int16) string {
	if time < 0 {
		return "Not set"
	}
	return fmt.Sprintf("%ds", time)
}

// Close gracefully shuts down the hub and all connected clients
func (h *Hub) Close() error {
	close(h.shutdown)
	<-h.done
	return nil
}

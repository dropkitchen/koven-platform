package service

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/dropkitchen/koven-platform/platform/internal/mqtt"
	"github.com/dropkitchen/koven-platform/platform/internal/protocol"
	"github.com/gorilla/websocket"
)

//go:embed web
var webFS embed.FS

// MQTTClient interface for dependency injection and testing
type MQTTClient interface {
	IsConnected() bool
	SendCommand(cmd *protocol.CommandPayload) error
	SetEventCallback(callback mqtt.EventCallback)
}

// Service represents the API server
type Service struct {
	mqttClient MQTTClient
	wsHub      *Hub
	upgrader   websocket.Upgrader
	closeOnce  sync.Once
}

// StartCommandRequest represents the payload for starting a command
type StartCommandRequest struct {
	Temperature string `json:"temperature"`
	Duration    string `json:"duration"`
}

// NewService creates a new API server
func NewService(serverAddr string, mqttClient MQTTClient) *Service {
	wsHub := NewHub()
	go wsHub.Run()

	mqttClient.SetEventCallback(wsHub.BroadcastEvent)

	s := &Service{
		mqttClient: mqttClient,
		wsHub:      wsHub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all connections
			},
		},
	}
	return s
}

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.healthcheckHandler)
	mux.HandleFunc("/start", s.startCommandHandler)
	mux.HandleFunc("/stop", s.stopCommandHandler)
	mux.HandleFunc("/ws/events", s.websocketHandler)

	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to get web subdirectory: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	return mux
}

func (s *Service) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{"status": s.mqttClient.IsConnected(), "websocket_clients": s.wsHub.GetClientCount()}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("Failed to write healthcheck response: %v", err)
	}
}

func (s *Service) startCommandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode start command request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	temperature, err := strconv.Atoi(req.Temperature)
	if err != nil {
		log.Printf("Invalid temperature value: %v", err)
		http.Error(w, "Invalid temperature value", http.StatusBadRequest)
		return
	}

	duration, err := strconv.Atoi(req.Duration)
	if err != nil {
		log.Printf("Invalid duration value: %v", err)
		http.Error(w, "Invalid duration value", http.StatusBadRequest)
		return
	}
	cmd := &protocol.CommandPayload{
		Action:      protocol.ActionStart,
		Temperature: int16(temperature),
		Duration:    int16(duration),
	}

	if err := s.mqttClient.SendCommand(cmd); err != nil {
		log.Printf("Failed to send command: %v", err)
		http.Error(w, "Failed to send command", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.Printf("Failed to write start command response: %v", err)
	}
}

func (s *Service) stopCommandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := &protocol.CommandPayload{
		Action: protocol.ActionStop,
	}

	if err := s.mqttClient.SendCommand(cmd); err != nil {
		log.Printf("Failed to send command: %v", err)
		http.Error(w, "Failed to send command", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.Printf("Failed to write stop command response: %v", err)
	}
}

func (s *Service) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection to WebSocket: %v", err)
		return
	}

	// Create new WebSocket client and register it with the hub
	client := NewWebSocketClient(s.wsHub, conn)
	s.wsHub.RegisterClient(client)
	client.Start()
}

// Close gracefully shuts down the service
func (s *Service) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		log.Println("Closing WebSocket hub...")
		if err := s.wsHub.Close(); err != nil {
			log.Printf("Error closing WebSocket hub: %v", err)
			closeErr = err
		}
	})
	return closeErr
}

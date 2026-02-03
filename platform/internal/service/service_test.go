package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dropkitchen/koven-platform/platform/internal/mqtt"
	"github.com/dropkitchen/koven-platform/platform/internal/protocol"
)

// MockMQTTClient for testing service layer
type MockMQTTClient struct {
	connected     bool
	sentCommands  []*protocol.CommandPayload
	sendError     error
	eventCallback mqtt.EventCallback
}

func NewMockMQTTClient(connected bool) *MockMQTTClient {
	return &MockMQTTClient{
		connected:    connected,
		sentCommands: make([]*protocol.CommandPayload, 0),
	}
}

func (m *MockMQTTClient) IsConnected() bool {
	return m.connected
}

func (m *MockMQTTClient) Connect() error {
	m.connected = true
	return nil
}

func (m *MockMQTTClient) Disconnect() {
	m.connected = false
}

func (m *MockMQTTClient) SendCommand(cmd *protocol.CommandPayload) error {
	if m.sendError != nil {
		return m.sendError
	}
	m.sentCommands = append(m.sentCommands, cmd)
	return nil
}

func (m *MockMQTTClient) SetEventCallback(callback mqtt.EventCallback) {
	m.eventCallback = callback
}

func (m *MockMQTTClient) GetSentCommands() []*protocol.CommandPayload {
	return m.sentCommands
}

func (m *MockMQTTClient) ClearSentCommands() {
	m.sentCommands = make([]*protocol.CommandPayload, 0)
}

// TestNewService tests service creation
func TestNewService(t *testing.T) {
	mockClient := NewMockMQTTClient(true)
	service := NewService("localhost:8080", mockClient)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}

	if service.mqttClient != mockClient {
		t.Error("Service MQTT client not set correctly")
	}

	if service.wsHub == nil {
		t.Error("Service WebSocket hub not initialized")
	}

	// Verify event callback was set on MQTT client
	if mockClient.eventCallback == nil {
		t.Error("Event callback not set on MQTT client")
	}
}

// TestHealthcheckHandler tests the health check endpoint
func TestHealthcheckHandler(t *testing.T) {
	tests := []struct {
		name           string
		connected      bool
		expectedStatus int
		checkBody      bool
	}{
		{
			name:           "connected",
			connected:      true,
			expectedStatus: http.StatusOK,
			checkBody:      true,
		},
		{
			name:           "disconnected",
			connected:      false,
			expectedStatus: http.StatusOK,
			checkBody:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockMQTTClient(tt.connected)
			service := NewService("localhost:8080", mockClient)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()

			service.healthcheckHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.expectedStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %s, want application/json", contentType)
			}

			if tt.checkBody {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				status, ok := response["status"].(bool)
				if !ok {
					t.Error("Response missing 'status' field or wrong type")
				}

				if status != tt.connected {
					t.Errorf("Status = %v, want %v", status, tt.connected)
				}
			}
		})
	}
}

// TestStartCommandHandler tests the start command endpoint
func TestStartCommandHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		contentType    string
		connected      bool
		expectedStatus int
		checkCommand   bool
		expectedTemp   int16
		expectedDur    int16
	}{
		{
			name:           "valid request",
			method:         http.MethodPost,
			body:           `{"temperature":"200","duration":"3600"}`,
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusOK,
			checkCommand:   true,
			expectedTemp:   200,
			expectedDur:    3600,
		},
		{
			name:           "method not allowed",
			method:         http.MethodGet,
			body:           "",
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusMethodNotAllowed,
			checkCommand:   false,
		},
		{
			name:           "invalid json",
			method:         http.MethodPost,
			body:           `{invalid}`,
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusBadRequest,
			checkCommand:   false,
		},
		{
			name:           "invalid temperature",
			method:         http.MethodPost,
			body:           `{"temperature":"abc","duration":"3600"}`,
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusBadRequest,
			checkCommand:   false,
		},
		{
			name:           "invalid duration",
			method:         http.MethodPost,
			body:           `{"temperature":"200","duration":"xyz"}`,
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusBadRequest,
			checkCommand:   false,
		},
		{
			name:           "zero values",
			method:         http.MethodPost,
			body:           `{"temperature":"0","duration":"0"}`,
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusOK,
			checkCommand:   true,
			expectedTemp:   0,
			expectedDur:    0,
		},
		{
			name:           "negative temperature",
			method:         http.MethodPost,
			body:           `{"temperature":"-10","duration":"100"}`,
			contentType:    "application/json",
			connected:      true,
			expectedStatus: http.StatusOK,
			checkCommand:   true,
			expectedTemp:   -10,
			expectedDur:    100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockMQTTClient(tt.connected)
			service := NewService("localhost:8080", mockClient)

			req := httptest.NewRequest(tt.method, "/start", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			service.startCommandHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.checkCommand {
				commands := mockClient.GetSentCommands()
				if len(commands) != 1 {
					t.Fatalf("Expected 1 command, got %d", len(commands))
				}

				cmd := commands[0]
				if cmd.Action != protocol.ActionStart {
					t.Errorf("Action = %d, want %d", cmd.Action, protocol.ActionStart)
				}
				if cmd.Temperature != tt.expectedTemp {
					t.Errorf("Temperature = %d, want %d", cmd.Temperature, tt.expectedTemp)
				}
				if cmd.Duration != tt.expectedDur {
					t.Errorf("Duration = %d, want %d", cmd.Duration, tt.expectedDur)
				}

				// Check response
				var response map[string]string
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response["status"] != "success" {
					t.Errorf("Response status = %s, want success", response["status"])
				}
			}
		})
	}
}

// TestStopCommandHandler tests the stop command endpoint
func TestStopCommandHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		connected      bool
		expectedStatus int
		checkCommand   bool
	}{
		{
			name:           "valid request",
			method:         http.MethodPost,
			connected:      true,
			expectedStatus: http.StatusOK,
			checkCommand:   true,
		},
		{
			name:           "method not allowed",
			method:         http.MethodGet,
			connected:      true,
			expectedStatus: http.StatusMethodNotAllowed,
			checkCommand:   false,
		},
		{
			name:           "mqtt not connected",
			method:         http.MethodPost,
			connected:      false,
			expectedStatus: http.StatusOK, // SendCommand still gets called
			checkCommand:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockMQTTClient(tt.connected)
			service := NewService("localhost:8080", mockClient)

			req := httptest.NewRequest(tt.method, "/stop", nil)
			w := httptest.NewRecorder()

			service.stopCommandHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.checkCommand && tt.method == http.MethodPost {
				commands := mockClient.GetSentCommands()
				if len(commands) != 1 {
					t.Fatalf("Expected 1 command, got %d", len(commands))
				}

				cmd := commands[0]
				if cmd.Action != protocol.ActionStop {
					t.Errorf("Action = %d, want %d", cmd.Action, protocol.ActionStop)
				}

				// Check response
				var response map[string]string
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if response["status"] != "success" {
					t.Errorf("Response status = %s, want success", response["status"])
				}
			}
		})
	}
}

// TestStartCommandHandlerWithSendError tests error handling when MQTT send fails
func TestStartCommandHandlerWithSendError(t *testing.T) {
	mockClient := NewMockMQTTClient(true)
	mockClient.sendError = http.ErrHandlerTimeout // Use any error
	service := NewService("localhost:8080", mockClient)

	body := `{"temperature":"200","duration":"3600"}`
	req := httptest.NewRequest(http.MethodPost, "/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.startCommandHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestStopCommandHandlerWithSendError tests error handling when MQTT send fails
func TestStopCommandHandlerWithSendError(t *testing.T) {
	mockClient := NewMockMQTTClient(true)
	mockClient.sendError = http.ErrHandlerTimeout
	service := NewService("localhost:8080", mockClient)

	req := httptest.NewRequest(http.MethodPost, "/stop", nil)
	w := httptest.NewRecorder()

	service.stopCommandHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestStartCommandHandlerEdgeCases tests edge cases for start command
func TestStartCommandHandlerEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "empty body",
			body:           "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing temperature",
			body:           `{"duration":"3600"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing duration",
			body:           `{"temperature":"200"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty string values",
			body:           `{"temperature":"","duration":""}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "very large temperature",
			body:           `{"temperature":"32767","duration":"3600"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "very large duration",
			body:           `{"temperature":"200","duration":"32767"}`,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockMQTTClient(true)
			service := NewService("localhost:8080", mockClient)

			req := httptest.NewRequest(http.MethodPost, "/start", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			service.startCommandHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

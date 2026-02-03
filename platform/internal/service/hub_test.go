package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dropkitchen/koven-platform/platform/internal/protocol"
)

// TestNewHub tests hub creation
func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub() returned nil")
	}

	if hub.clients == nil {
		t.Error("Hub clients map not initialized")
	}

	if hub.broadcast == nil {
		t.Error("Hub broadcast channel not initialized")
	}

	if hub.register == nil {
		t.Error("Hub register channel not initialized")
	}

	if hub.unregister == nil {
		t.Error("Hub unregister channel not initialized")
	}
}

// TestHubBroadcastEvent tests event broadcasting
func TestHubBroadcastEvent(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock clients
	client1 := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	client2 := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}

	// Register clients
	hub.RegisterClient(client1)
	hub.RegisterClient(client2)

	// Give the hub time to register clients
	time.Sleep(10 * time.Millisecond)

	// Create a test event
	event := &protocol.EventPayload{
		State:                 protocol.StateBaking,
		CurrentTemperature:    180,
		RemainingTime:         1800,
		ProgrammedDuration:    3600,
		ProgrammedTemperature: 200,
	}

	// Broadcast the event
	hub.BroadcastEvent(event)

	// Wait for broadcast to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify both clients received the message
	expectedMsg := &EventMessage{
		Type:                  "event",
		State:                 "BAKING",
		CurrentTemperature:    "180°C",
		RemainingTime:         "1800s",
		ProgrammedDuration:    "3600s",
		ProgrammedTemperature: "200°C",
	}

	select {
	case msg1 := <-client1.send:
		verifyEventMessage(t, msg1, expectedMsg)
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 1 did not receive broadcast message")
	}

	select {
	case msg2 := <-client2.send:
		verifyEventMessage(t, msg2, expectedMsg)
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 2 did not receive broadcast message")
	}
}

// TestHubRegisterUnregisterClient tests client registration and unregistration
func TestHubRegisterUnregisterClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}

	// Register client
	hub.RegisterClient(client)
	time.Sleep(10 * time.Millisecond)

	if hub.GetClientCount() != 1 {
		t.Errorf("GetClientCount() = %d, want 1 after registration", hub.GetClientCount())
	}

	// Unregister client
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	if hub.GetClientCount() != 0 {
		t.Errorf("GetClientCount() = %d, want 0 after unregistration", hub.GetClientCount())
	}

	// Verify channel was closed
	select {
	case _, ok := <-client.send:
		if ok {
			t.Error("Client send channel should be closed after unregistration")
		}
	default:
		t.Error("Client send channel not closed")
	}
}

// TestHubMultipleClientsRegistration tests multiple client registrations
func TestHubMultipleClientsRegistration(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	numClients := 10
	clients := make([]*Client, numClients)

	// Register multiple clients
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:  hub,
			send: make(chan []byte, 256),
		}
		hub.RegisterClient(clients[i])
	}

	time.Sleep(50 * time.Millisecond)

	if hub.GetClientCount() != numClients {
		t.Errorf("GetClientCount() = %d, want %d", hub.GetClientCount(), numClients)
	}

	// Unregister all clients
	for _, client := range clients {
		hub.unregister <- client
	}

	time.Sleep(50 * time.Millisecond)

	if hub.GetClientCount() != 0 {
		t.Errorf("GetClientCount() = %d, want 0 after all unregistrations", hub.GetClientCount())
	}
}

// TestHubBroadcastToMultipleClients tests broadcasting to many clients
func TestHubBroadcastToMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	numClients := 20
	clients := make([]*Client, numClients)

	// Register multiple clients
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:  hub,
			send: make(chan []byte, 256),
		}
		hub.RegisterClient(clients[i])
	}

	time.Sleep(50 * time.Millisecond)

	// Create a test event
	event := &protocol.EventPayload{
		State:                 protocol.StatePreheating,
		CurrentTemperature:    100,
		RemainingTime:         3600,
		ProgrammedDuration:    3600,
		ProgrammedTemperature: 220,
	}

	// Broadcast the event
	hub.BroadcastEvent(event)

	time.Sleep(100 * time.Millisecond)

	// Verify all clients received the message
	expectedMsg := &EventMessage{
		Type:                  "event",
		State:                 "PREHEATING",
		CurrentTemperature:    "100°C",
		RemainingTime:         "3600s",
		ProgrammedDuration:    "3600s",
		ProgrammedTemperature: "220°C",
	}

	received := 0
	for _, client := range clients {
		select {
		case msg := <-client.send:
			verifyEventMessage(t, msg, expectedMsg)
			received++
		default:
			t.Errorf("Client did not receive broadcast message")
		}
	}

	if received != numClients {
		t.Errorf("Received messages = %d, want %d", received, numClients)
	}
}

// TestHubClientBufferFull tests behavior when client buffer is full
func TestHubClientBufferFull(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create client with small buffer
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}

	hub.RegisterClient(client)
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	event := &protocol.EventPayload{
		State:                 protocol.StateIdle,
		CurrentTemperature:    25,
		RemainingTime:         0,
		ProgrammedDuration:    0,
		ProgrammedTemperature: 0,
	}

	// Send enough events to overflow the buffer
	for i := 0; i < 5; i++ {
		hub.BroadcastEvent(event)
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)

	// Client should have been removed due to full buffer
	if hub.GetClientCount() > 1 {
		t.Error("Client with full buffer should have been removed")
	}
}

// TestFormatTemperature tests temperature formatting
func TestFormatTemperature(t *testing.T) {
	tests := []struct {
		name     string
		temp     int16
		expected string
	}{
		{"zero", 0, "0°C"},
		{"positive", 180, "180°C"},
		{"max value", 32767, "32767°C"},
		{"negative", -5, "--"},
		{"negative large", -100, "--"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTemperature(tt.temp)
			if result != tt.expected {
				t.Errorf("formatTemperature(%d) = %s, want %s", tt.temp, result, tt.expected)
			}
		})
	}
}

// TestFormatTime tests time formatting
func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		time     int16
		expected string
	}{
		{"zero", 0, "0s"},
		{"positive", 1800, "1800s"},
		{"max value", 32767, "32767s"},
		{"negative", -5, "--"},
		{"negative large", -1000, "--"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatTime(%d) = %s, want %s", tt.time, result, tt.expected)
			}
		})
	}
}

// TestFormatProgrammedTemperature tests programmed temperature formatting
func TestFormatProgrammedTemperature(t *testing.T) {
	tests := []struct {
		name     string
		temp     int16
		expected string
	}{
		{"zero", 0, "0°C"},
		{"positive", 200, "200°C"},
		{"max value", 32767, "32767°C"},
		{"negative", -5, "Not set"},
		{"negative large", -100, "Not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatProgrammedTemperature(tt.temp)
			if result != tt.expected {
				t.Errorf("formatProgrammedTemperature(%d) = %s, want %s", tt.temp, result, tt.expected)
			}
		})
	}
}

// TestFormatProgrammedTime tests programmed time formatting
func TestFormatProgrammedTime(t *testing.T) {
	tests := []struct {
		name     string
		time     int16
		expected string
	}{
		{"zero", 0, "0s"},
		{"positive", 3600, "3600s"},
		{"max value", 32767, "32767s"},
		{"negative", -5, "Not set"},
		{"negative large", -1000, "Not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatProgrammedTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatProgrammedTime(%d) = %s, want %s", tt.time, result, tt.expected)
			}
		})
	}
}

// TestHubEventConversion tests conversion of protocol event to JSON event
func TestHubEventConversion(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}

	hub.RegisterClient(client)
	time.Sleep(10 * time.Millisecond)

	tests := []struct {
		name     string
		event    *protocol.EventPayload
		expected *EventMessage
	}{
		{
			name: "IDLE state",
			event: &protocol.EventPayload{
				State:                 protocol.StateIdle,
				CurrentTemperature:    25,
				RemainingTime:         0,
				ProgrammedDuration:    0,
				ProgrammedTemperature: 0,
			},
			expected: &EventMessage{
				Type:                  "event",
				State:                 "IDLE",
				CurrentTemperature:    "25°C",
				RemainingTime:         "0s",
				ProgrammedDuration:    "0s",
				ProgrammedTemperature: "0°C",
			},
		},
		{
			name: "PREHEATING state",
			event: &protocol.EventPayload{
				State:                 protocol.StatePreheating,
				CurrentTemperature:    100,
				RemainingTime:         3600,
				ProgrammedDuration:    3600,
				ProgrammedTemperature: 220,
			},
			expected: &EventMessage{
				Type:                  "event",
				State:                 "PREHEATING",
				CurrentTemperature:    "100°C",
				RemainingTime:         "3600s",
				ProgrammedDuration:    "3600s",
				ProgrammedTemperature: "220°C",
			},
		},
		{
			name: "BAKING state",
			event: &protocol.EventPayload{
				State:                 protocol.StateBaking,
				CurrentTemperature:    200,
				RemainingTime:         1800,
				ProgrammedDuration:    3600,
				ProgrammedTemperature: 200,
			},
			expected: &EventMessage{
				Type:                  "event",
				State:                 "BAKING",
				CurrentTemperature:    "200°C",
				RemainingTime:         "1800s",
				ProgrammedDuration:    "3600s",
				ProgrammedTemperature: "200°C",
			},
		},
		{
			name: "COOLING_DOWN state",
			event: &protocol.EventPayload{
				State:                 protocol.StateCoolingDown,
				CurrentTemperature:    80,
				RemainingTime:         0,
				ProgrammedDuration:    3600,
				ProgrammedTemperature: 200,
			},
			expected: &EventMessage{
				Type:                  "event",
				State:                 "COOLING_DOWN",
				CurrentTemperature:    "80°C",
				RemainingTime:         "0s",
				ProgrammedDuration:    "3600s",
				ProgrammedTemperature: "200°C",
			},
		},
		{
			name: "negative values - formatted as not set",
			event: &protocol.EventPayload{
				State:                 protocol.StateIdle,
				CurrentTemperature:    -5,
				RemainingTime:         -10,
				ProgrammedDuration:    -1,
				ProgrammedTemperature: -1,
			},
			expected: &EventMessage{
				Type:                  "event",
				State:                 "IDLE",
				CurrentTemperature:    "--",
				RemainingTime:         "--",
				ProgrammedDuration:    "Not set",
				ProgrammedTemperature: "Not set",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub.BroadcastEvent(tt.event)
			time.Sleep(20 * time.Millisecond)

			select {
			case msg := <-client.send:
				verifyEventMessage(t, msg, tt.expected)
			case <-time.After(100 * time.Millisecond):
				t.Error("Did not receive broadcast message")
			}
		})
	}
}

// Helper function to verify event message content
func verifyEventMessage(t *testing.T, msgBytes []byte, expected *EventMessage) {
	t.Helper()

	var got EventMessage
	if err := json.Unmarshal(msgBytes, &got); err != nil {
		t.Fatalf("Failed to unmarshal event message: %v", err)
	}

	if got.Type != expected.Type {
		t.Errorf("Type = %s, want %s", got.Type, expected.Type)
	}

	if got.State != expected.State {
		t.Errorf("State = %s, want %s", got.State, expected.State)
	}

	if got.CurrentTemperature != expected.CurrentTemperature {
		t.Errorf("CurrentTemperature = %s, want %s", got.CurrentTemperature, expected.CurrentTemperature)
	}

	if got.RemainingTime != expected.RemainingTime {
		t.Errorf("RemainingTime = %s, want %s", got.RemainingTime, expected.RemainingTime)
	}

	if got.ProgrammedDuration != expected.ProgrammedDuration {
		t.Errorf("ProgrammedDuration = %s, want %s", got.ProgrammedDuration, expected.ProgrammedDuration)
	}

	if got.ProgrammedTemperature != expected.ProgrammedTemperature {
		t.Errorf("ProgrammedTemperature = %s, want %s", got.ProgrammedTemperature, expected.ProgrammedTemperature)
	}
}

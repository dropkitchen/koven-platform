package mqtt

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dropkitchen/koven-platform/platform/internal/protocol"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	TopicCommands = "cmds/koven"
	TopicEvents   = "events/koven"
	QoS           = 1
)

// EventCallback is a function type for handling received events
type EventCallback func(*protocol.EventPayload)

// Client manages MQTT communication with Koven devices
type Client struct {
	client        mqtt.Client
	mu            sync.RWMutex
	connected     bool
	eventCallback EventCallback
}

// NewClient creates a new MQTT client
func NewClient(brokerURL string, clientID string) (*Client, error) {
	c := &Client{
		connected:     false,
		eventCallback: nil,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	opts.SetKeepAlive(20 * time.Second)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetOnConnectHandler(c.onConnect)
	opts.SetConnectionLostHandler(c.onConnectionLost)

	c.client = mqtt.NewClient(opts)

	return c, nil
}

// Connect establishes connection to the MQTT broker
func (c *Client) Connect() error {
	log.Printf("Connecting to MQTT broker...")

	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	log.Printf("Connected to MQTT broker")
	return nil
}

// Disconnect closes the connection to the MQTT broker
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		c.client.Disconnect(250)
		c.connected = false
		log.Printf("Disconnected from MQTT broker")
	}
}

// IsConnected returns the connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SetEventCallback sets the callback function for received events
func (c *Client) SetEventCallback(callback EventCallback) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventCallback = callback
}

// onConnect is called when the client connects to the broker
func (c *Client) onConnect(client mqtt.Client) {
	log.Printf("MQTT client connected, subscribing to %s", TopicEvents)

	token := client.Subscribe(TopicEvents, QoS, c.messageHandler)
	if token.Wait() && token.Error() != nil {
		log.Printf("Failed to subscribe to %s: %v", TopicEvents, token.Error())
		return
	}

	log.Printf("Subscribed to %s", TopicEvents)
}

// onConnectionLost is called when the connection to the broker is lost
func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	log.Printf("MQTT connection lost: %v", err)
}

// messageHandler processes incoming MQTT messages
func (c *Client) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received event from %s (%d bytes)", msg.Topic(), len(msg.Payload()))

	event, err := protocol.UnmarshallEventFrame(msg.Payload())
	if err != nil {
		log.Printf("Failed to unmarshal event frame: %v", err)
		return
	}

	log.Printf("Event parsed: state=%s, temp=%d°C, remaining=%ds, programmed_temp=%d°C, programmed_duration=%ds",
		protocol.StateToString(event.State),
		event.CurrentTemperature,
		event.RemainingTime,
		event.ProgrammedTemperature,
		event.ProgrammedDuration)

	// Call the event callback if set
	c.mu.RLock()
	callback := c.eventCallback
	c.mu.RUnlock()

	if callback != nil {
		callback(event)
	}
}

// SendCommand sends a command to the Koven device
func (c *Client) SendCommand(cmd *protocol.CommandPayload) error {
	if !c.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	frame, err := protocol.MarshallCommandFrame(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshall command: %w", err)
	}

	log.Printf("Sending command: action=%s, temperature=%d°C, duration=%ds",
		protocol.ActionToString(cmd.Action),
		cmd.Temperature,
		cmd.Duration)

	token := c.client.Publish(TopicCommands, QoS, false, frame)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish command: %w", token.Error())
	}

	log.Printf("Command sent successfully")
	return nil
}

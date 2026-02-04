# Koven Platform Service

Go-based HTTP API and WebSocket service for managing Koven smart oven devices via MQTT.

## Overview

The platform service provides:

- REST API for sending commands to the oven emulator
- WebSocket endpoint for real-time event streaming
- Embedded web UI for oven control
- MQTT client for device communication

## Architecture

```
┌─────────────────────────────────────────┐
│           main.go                       │
│   HTTP server + graceful shutdown       │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│         internal/service/               │
│  ┌────────────────────────────────┐     │
│  │  service.go - HTTP handlers    │     │
│  │  - /health                     │     │
│  │  - /start (POST)               │     │
│  │  - /stop (POST)                │     │
│  │  - /ws/events (WebSocket)      │     │
│  │  - / (static web UI)           │     │
│  └────┬────────────────────┬──────┘     │
│       │                    │            │
│  ┌────▼─────┐       ┌──────▼──────┐     │
│  │  hub.go  │       │ websocket.go│     │
│  │ WS hub   │       │ WS client   │     │
│  └──────────┘       └─────────────┘     │
└───────────────────────┬─────────────────┘
                        │
        ┌───────────────▼───────────────┐
        │    internal/mqtt/mqtt.go      │
        │  - Connect to broker          │
        │  - Subscribe to events/koven  │
        │  - Publish to cmds/koven      │
        └───────────┬───────────────────┘
                    │
        ┌───────────▼───────────────────┐
        │  internal/protocol/           │
        │  Binary protocol codec        │
        │  - Marshal commands           │
        │  - Unmarshal events           │
        │  - CRC-16/USB                 │
        └───────────────────────────────┘
```

## Building

### Prerequisites

- Go 1.25+
- MQTT broker (Mosquitto recommended)

### Local Build

```bash
go build -o koven-platform .
```

### Run Tests

```bash
go test ./...
```

### Docker Build

```bash
docker build -f ../docker/Dockerfile.platform -t koven-platform .
```

## Running

### Local Execution

```bash
# Start with defaults (localhost:8080, localhost MQTT broker)
./koven-platform
```

### With Docker Compose

```bash
# From repository root
docker compose -f docker/compose.yaml up platform
```

## API Reference

### GET /health

Health check endpoint returning connection status.

**Response:**

```json
{
  "status": true,
  "websocket_clients": 2
}
```

### POST /start

Start the oven with specified temperature and duration.

**Request:**

```json
{
  "temperature": "180",
  "duration": "3600"
}
```

**Parameters:**

- `temperature`: Target temperature in °C (string)
- `duration`: Baking duration in seconds (string)

**Response:**

```json
{
  "status": "success"
}
```

**Errors:**

- `400 Bad Request`: Invalid parameters
- `405 Method Not Allowed`: Non-POST request
- `500 Internal Server Error`: MQTT publish failure

### POST /stop

Stop the oven immediately.

**Response:**

```json
{
  "status": "success"
}
```

**Errors:**

- `405 Method Not Allowed`: Non-POST request
- `500 Internal Server Error`: MQTT publish failure

### WebSocket /ws/events

Real-time event stream from connected ovens.

**Event Message Format:**

```json
{
  "type": "event",
  "state": "BAKING",
  "current_temperature": "180°C",
  "remaining_time": "3540s",
  "programmed_duration": "3600s",
  "programmed_temperature": "180°C"
}
```

**Special Values:**

- `"--"` - Used for current temperature/time when not applicable
- `"Not set"` - Used for programmed values when not set

## Code Structure

### `main.go`

Entry point that:

- Parses command-line flags
- Initializes MQTT client
- Creates HTTP server with routes
- Handles graceful shutdown on SIGINT/SIGTERM

### `internal/service/`

#### `service.go`

HTTP service with:

- Route handlers for REST API
- Embedded web UI serving (via `//go:embed web`)
- MQTT client integration
- WebSocket upgrade handling

#### `hub.go`

WebSocket hub managing:

- Client registration/unregistration
- Event broadcasting to all connected clients
- Thread-safe client map operations

#### `websocket.go`

WebSocket client wrapper:

- Message writing to individual clients
- Automatic cleanup on disconnect

### `internal/mqtt/`

#### `mqtt.go`

MQTT client wrapper:

- Connection management with auto-reconnect
- Subscribe to `events/koven` topic
- Publish commands to `cmds/koven` topic
- Event callback registration
- Thread-safe connection status

### `internal/protocol/`

Binary protocol implementation matching firmware:

- Command frame marshalling
- Event frame unmarshalling
- CRC-16/USB checksum validation
- Little-endian encoding/decoding

## Web UI

The embedded web UI (`internal/service/web/`) provides:

- Real-time oven status display
- Temperature and time input controls
- START/STOP command buttons
- WebSocket-based live updates

Access at: `http://localhost:8080/`

## MQTT Topics

| Topic          | Direction | QoS | Purpose                      |
| -------------- | --------- | --- | ---------------------------- |
| `cmds/koven`   | Publish   | 1   | Send commands to firmware    |
| `events/koven` | Subscribe | 1   | Receive events from firmware |

## Testing

### Unit Tests

```bash
go test ./...
```

# Koven Firmware Emulator

A C-based oven firmware emulator with MQTT communication and state machine design.

## Overview

The emulator simulates a smart oven with:

- Four-state finite state machine (IDLE, PREHEATING, BAKING, COOLING_DOWN)
- MQTT-based command/event messaging
- Binary protocol with CRC-16 checksums
- 1 Hz tick-based event reporting

## Architecture

```
┌─────────────────────────────────┐
│         main.c                  │
│  Entry point & initialization   │
└───────────┬─────────────────────┘
            │
┌───────────▼─────────────────────┐
│      mqtt_client.c              │
│  MQTT connection & event loop   │
│  - Subscribe to cmds/koven      │
│  - Publish to events/koven      │
│  - 1 Hz tick timer              │
└───────┬─────────────────┬───────┘
        │                 │
┌───────▼─────────┐  ┌────▼───────┐
│   protocol.c    │  │  koven.c   │
│  Binary codec   │  │ State      │
│  - Frame/parse  │  │ machine    │
│  - CRC-16/USB   │  │ logic      │
└─────────────────┘  └────────────┘
```

## Building

### Prerequisites

- CMake 3.10+
- GCC or Clang
- Paho MQTT C client library

### Local Build

```bash
mkdir build
cd build
cmake ..
make
```

### Run Tests

```bash
ctest --output-on-failure
```

### Docker Build

```bash
docker build -f ../docker/Dockerfile.koven -t koven-firmware .
```

## Running

### Local Execution

Ensure an MQTT broker is running (default: `localhost:1883`):

```bash
./build/koven
```

### With Docker Compose

```bash
# From repository root
docker compose -f docker/compose.yaml up koven
```

## Code Structure

### `main.c`

Entry point that initializes the Koven state and starts the MQTT client event loop.

### `koven.c/h`

Core oven state machine implementation:

- `koven_init()`: Initialize to IDLE state at 25°C (room temperature)
- `koven_execute()`: Process incoming commands (START/STOP)
- `koven_tick()`: Update state every second and generate events

### `mqtt_client.c/h`

MQTT communication layer:

- Connects to broker and subscribes to `cmds/koven`
- Publishes events to `events/koven` every second
- Deserializes incoming command frames
- Serializes outgoing event frames

### `protocol.c/h`

Binary protocol implementation:

- Frame marshalling/unmarshalling
- CRC-16/USB checksum calculation (polynomial: 0x8005)
- Little-endian encoding for multi-byte fields

## State Machine Details

### States

| State        | Code | Behavior                                    |
| ------------ | ---- | ------------------------------------------- |
| IDLE         | 0    | Room temperature (25°C), awaiting commands  |
| PREHEATING   | 1    | Heating +1°C/sec to target temperature      |
| BAKING       | 2    | Maintaining target temp, counting down time |
| COOLING_DOWN | 3    | Cooling -1°C/sec to 25°C                    |

### Commands

| Command | Code | Parameters            | Action                                   |
| ------- | ---- | --------------------- | ---------------------------------------- |
| START   | 1    | temperature, duration | Begin preheating to temp for duration    |
| STOP    | 2    | (ignored)             | Stop current operation and begin cooling |

### Events

Events are sent every second with the current oven state:

```c
typedef struct {
    uint8_t state;                  // Current state (0-3)
    int16_t current_temperature;    // Current temp in °C
    int16_t remaining_time;         // Seconds left (-1 if N/A)
    int16_t programmed_duration;    // Total duration (-1 if N/A)
    int16_t programmed_temperature; // Target temp (-1 if N/A)
} EventPayload;
```

## Protocol Details

### Frame Structure

All messages start with a common header:

| Offset | Size | Field    | Description                       |
| ------ | ---- | -------- | --------------------------------- |
| 0      | 1    | msg_type | 0x01 (Command) or 0x02 (Event)    |
| 1      | 2    | size     | Payload size (little-endian)      |
| 3      | N    | payload  | Command (5 bytes) or Event (9)    |
| 3+N    | 2    | crc      | CRC-16/USB over type+size+payload |

### Example: START Command

Start oven at 180°C for 60 seconds:

```
Hex: 01 05 00 01 B4 00 3C 00 8C 7E

01       - msg_type: COMMAND
05 00    - size: 5 bytes (little-endian)
01       - action: START
B4 00    - temperature: 180°C (little-endian)
3C 00    - duration: 60 seconds (little-endian)
8C 7E    - CRC-16/USB checksum
```

## Configuration

Environment variables (defaults shown):

| Variable    | Default              | Description       |
| ----------- | -------------------- | ----------------- |
| MQTT_BROKER | tcp://localhost:1883 | MQTT broker URL   |
| DEVICE_ID   | koven_001            | Device identifier |

## Testing

The `tests/` directory contains unit tests for:

- Protocol encoding/decoding
- CRC calculation
- State machine transitions
- Command handling

Run with: `ctest --output-on-failure`

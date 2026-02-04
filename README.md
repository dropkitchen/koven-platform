# Koven Platform

A demonstration platform for firmware-cloud communication using MQTT, built for live coding sessions with firmware engineers at Fresco. The platform simulates a smart oven with a C-based firmware emulator communicating with a Go backend service and web UI.

## Architecture

```
  ┌──────────┐            ┌──────────┐            ┌──────────┐
  │  Koven   │ <──cmds─── │   MQTT   │ <──cmds─── │  Koven   │
  │ Firmware │ ───evts──> │  Broker  │ ───evts──> │ Platform │
  └──────────┘            └──────────┘            └──────────┘
       C                   Mosquitto                Go + Web UI
```

The system consists of three main components:

- **Koven Firmware Emulator** (`koven/`): C-based oven emulator with state machine
- **Koven Platform Service** (`platform/`): Go HTTP API and WebSocket server with embedded web UI
- **MQTT Broker**: Mosquitto broker for async message routing

Communication uses a custom binary protocol over MQTT with CRC-16 checksums for data integrity.

## Quick Start

Run the full stack with Docker Compose:

```bash
docker compose -f docker/compose.yaml up --build
```

Access the web UI at http://localhost:8080

## Components

### Koven Firmware (`koven/`)

C-based oven emulator implementing a state machine with four states (IDLE, PREHEATING, BAKING, COOLING_DOWN). See `koven/README.md` for details.

### Koven Platform (`platform/`)

Go service providing HTTP API, WebSocket events, and web UI for controlling ovens. See `platform/README.md` for details.

### Docker (`docker/`)

Contains Dockerfiles for both components and `compose.yaml` orchestrating the full stack with Mosquitto MQTT broker.

## Communication Protocol

Binary protocol with little-endian encoding, CRC-16/USB checksums, and two message types:

- **Commands** (Platform → Firmware): START/STOP actions with temperature and duration
- **Events** (Firmware → Platform): State updates with current temperature and remaining time

### Frame Format

| Field    | Size    | Description                         |
| -------- | ------- | ----------------------------------- |
| msg_type | 1 byte  | Message type (0x01=cmd, 0x02=event) |
| size     | 2 bytes | Payload size (little-endian)        |
| payload  | N bytes | Message payload                     |
| crc      | 2 bytes | CRC-16/USB checksum (little-endian) |

### Command Payload (5 bytes)

| Field       | Type  | Description             |
| ----------- | ----- | ----------------------- |
| action      | uint8 | 1=START, 2=STOP         |
| temperature | int16 | Target temperature (°C) |
| duration    | int16 | Duration (seconds)      |

### Event Payload (9 bytes)

| Field                  | Type  | Description                                    |
| ---------------------- | ----- | ---------------------------------------------- |
| state                  | uint8 | 0=IDLE, 1=PREHEATING, 2=BAKING, 3=COOLING_DOWN |
| current_temperature    | int16 | Current oven temperature (°C)                  |
| remaining_time         | int16 | Remaining time (seconds, -1 if N/A)            |
| programmed_duration    | int16 | Programmed duration (seconds, -1 if N/A)       |
| programmed_temperature | int16 | Programmed temperature (°C, -1 if N/A)         |

### CRC-16/USB

- Polynomial: `0x8005`
- Initial: `0xFFFF`
- Reflected I/O: Yes
- XOR out: `0xFFFF`

## Oven Behavior

The oven operates in four states:

1. **IDLE**: Waiting for commands at room temperature (25°C)
2. **PREHEATING**: Heating at 1°C/sec to target temperature
3. **BAKING**: Maintaining target temperature for programmed duration
4. **COOLING_DOWN**: Cooling at 1°C/sec back to 25°C

Commands:

- **START**: Begin heating to target temperature for specified duration
- **STOP**: Immediately stop and begin cooling

```
   temp
     ▲
90°C │             ┌────────┐
     │            ╱          ╲
     │           ╱            ╲
     │          ╱              ╲
     │         ╱                ╲
25°C │────────┘                  └──────►
     │  idle  │pre│ baking │cool│ idle
              heat           down
     └────────────────────────────────► time
```

## Development

Each component can be developed independently:

- **Firmware**: See `koven/README.md` for building and testing with CMake
- **Platform**: See `platform/README.md` for running the Go service and web UI

Both components communicate via MQTT topics:

- `cmds/koven` - Commands sent to firmware
- `events/koven` - Events sent from firmware

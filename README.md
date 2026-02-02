# The Koven Platform

The goal of this repository is to have a working environment for live coding sessions with firmware engineers at Fresco. The Koven platform share some of the high level ideas behind the Fresco firmware architecture, but is not intended to be a full representation of it.

The high level architecture of the Koven platform is as follows:

```
  ┌──────────┐            ┌──────────┐            ┌──────────┐
  │  Koven   │ <──cmds─── │   MQTT   │ <──cmds─── │  Koven   │
  │ Firmware │ ───evts──> │  Broker  │ ───evts──> │ Platform │
  └──────────┘            └──────────┘            └──────────┘        
```

## Contents

The repository contains the following main components:

- `koven`: This directory contains the Koven firmware emulator for an oven, it is written in C and use CMake as build system. See below for more details.
- `platform`: This directory contains a Golang project that implements a service to manage Koven devices.
- `docker`: This directory contains Dockerfiles to build images for the Koven firmware emulator and the platform service. This is used by the `compose.yaml` file to run the full stack locally with a Mosquitto MQTT broker, which is used for the communication between the Koven devices and the platform service.

## Building and Running
To build and run the Koven platform locally, you can use Docker Compose. Make sure you have Docker and Docker Compose installed on your machine. Then just run the following command from the root of the repository:

```bash
docker-compose -f docker/compose.yaml up --build
```

### Oven behavior

The oven currently supports two commands:
- **START**: Start the oven with a target temperature (celsius) and duration (seconds).
- **STOP**: Stop the oven.

The oven has four states:
1. **IDLE State**: Initial state, waiting for commands.
1. **PREHEATING State**: Temperature increases by 1°C per second until reaching the programmed temperature (duration).
1. **BAKING State**: Counts down the remaining time, transitions to COOLING DOWNwhen complete.
1. **COOLING DOWN State**: Temperature decreases by 1°C per second until reaching room temperature (25°C), then transitions to IDLE.

When the oven receives a START command it will begin heating to the target temperature (preheating state) and then maintain that temperature for the specified duration (baking state). If a stop command is received, the oven will stop heating and will transition to the cooling down state until the oven temperature reaches the room temperare, after that it returns to idle state.

```
   temp
     ▲
     ├─────────────────────────────────────► time
     │
90ºC │             +────────+
     │            /.        .\
     │           / .        . \
     │          /  .        .  \
     │         /   .        .   \
     │        /    .        .    \
     │       /     .        .     \
25ºC │──────+      .        .      +──────►
     | idle | pre  | baking | cool | idle
              heat            down   
```

The oven sends events periodically to report its current state, temperature, remaining time, and programmed settings.

## Communication Protocol

The communication between the Koven platform service and the Koven firmware emulator uses a custom binary protocol over MQTT. The protocol defines messages for commands sent to the oven and events sent from the oven to the platform representing the oven's state.

Basically the protocol defines two types of messages:
1. **Commands**: Sent from the platform service to the oven to control its operation (start, stop, update temperature).
1. **Events**: Sent from the oven to the platform service to report its current state (idle, preheating, baking), current temperature, remaining time, etc.

The messages are framed with a simple header containing the message type, size, and a CRC-16/USB checksum for integrity verification and we use little-endian encoding for multi-byte fields.

### Message Frame Format

All messages use **little-endian** encoding:

| Field    | Size    | Description                          |
|----------|---------|--------------------------------------|
| msg_type | 1 byte  | Message type identifier              |
| size     | 2 bytes | Payload size (little-endian)         |
| payload  | N bytes | Message payload                      |
| crc      | 2 bytes | CRC-16/USB checksum (little-endian)  |

### Message Types

- `0x01` - Command (from client to oven)
- `0x02` - Event (from oven to clients)

### Command Payload (5 bytes)

| Field       | Type    | Description                    |
|-------------|---------|--------------------------------|
| action      | uint8   | Action code (1=start, 2=stop)  |
| temperature | int16   | Target temperature in °C       |
| duration    | int16   | Duration in seconds            |

**Action Codes:**
- `1` - START: Start the oven with given temperature and duration
- `2` - STOP: Stop the oven

### Event Payload (9 bytes)

| Field                  | Type    | Description                        |
|------------------------|---------|-------------------------------------|
| state                  | uint8   | Current state (0=idle, 1=preheating, 2=baking, 3=cooling down) |
| current_temperature    | int16   | Current oven temperature in °C |
| remaining_time         | int16   | Remaining time in seconds (-1 if N/A) |
| programmed_duration    | int16   | Programmed duration in seconds (-1 if N/A) |
| programmed_temperature | int16   | Programmed temperature in °C (-1 if N/A) |

**State Codes:**
- `0` - IDLE: Oven is idle
- `1` - PREHEATING: Heating to target temperature
- `2` - BAKING: Baking at target temperature
- `3` - COOLING DOWN: Cooling down to room temperature 

### Example Frame

**Command: START oven at 180°C for 60 seconds**

Hex: `0105000100B4003C8C7E`

Breakdown:
- `01` - Message type (COMMAND)
- `0500` - Payload size (5 bytes, little-endian)
- `01` - Action (START)
- `B400` - Temperature (180°C, little-endian)
- `3C00` - Duration (60 seconds, little-endian)
- `8C7E` - CRC-16/USB checksum

### CRC-16/USB

- Polynomial: `0x8005`
- Initial value: `0xFFFF`
- Input reflected: Yes
- Output reflected: Yes
- XOR output: `0xFFFF`

The CRC is calculated over the `msg_type`, `size`, and `payload` fields.

## Koven Firmware Emulator

The Koven firmware emulator is implemented in C and simulates the behavior of a smart oven. It connects to an MQTT broker to receive commands and send events. The source code can be found in the `koven` directory.

It basically has the following source files:
- `main.[c|h]`: Entry point of the emulator.
- `mqtt_client.[c|h]`: Handles MQTT communication and the main event loop.
- `koven.[c|h]`: Implements the oven state machine and behavior.
- `protocol.[c|h]`: Implements the binary protocol for commands and events.

There is a CMakeLists.txt file to build the project using CMake. You will need to have installed the Paho MQTT C client library to build the emulator. You can also use the provided Dockerfile in the `docker` directory to build a Docker image for the emulator.

## Koven Platform Service
TODO

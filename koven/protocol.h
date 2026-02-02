#ifndef PROTOCOL_H
#define PROTOCOL_H

#include <stddef.h>
#include <stdint.h>
#include "koven.h"

// Message type identifiers
#define MSG_TYPE_COMMAND  0x01
#define MSG_TYPE_EVENT    0x02

#define MAX_PAYLOAD_SIZE  32

// Frame structure (little-endian)
// [msg_type:1][size:2][payload:size][crc:2]
// The CRC is calculated over msg_type, size, and payload and uses CRC-16/USB
typedef struct __attribute__((packed)) {
    uint8_t msg_type;
    uint16_t size;
    uint8_t payload[MAX_PAYLOAD_SIZE];
    uint16_t crc;
} Frame;

uint16_t crc16_usb(const uint8_t *data, size_t length);

// Unmarshalls a command frame from raw bytes to a CommandPayload structure
// Returns 0 on success, -1 on error
int unmarshall_command_frame(const uint8_t *data, size_t len, CommandPayload *cmd);

// Marshalls an event frame from event payload structure to raw bytes
// Returns frame size on success, -1 on error
int marshall_event_frame(const EventPayload *event, uint8_t *buffer, size_t buffer_size);

// Helper to print frame in hex format (for debugging)
void print_frame_hex(const uint8_t *data, size_t len);

#endif /* PROTOCOL_H */

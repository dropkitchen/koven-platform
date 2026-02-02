#include "protocol.h"
#include <arpa/inet.h>
#include <stdio.h>
#include <string.h>

/* CRC-16/USB implementation
 * Polynomial: 0x8005
 * Initial value: 0xFFFF
 * Input reflected: Yes
 * Output reflected: Yes
 * XOR output: 0xFFFF
 */
uint16_t crc16_usb(const uint8_t *data, size_t length)
{
    uint16_t crc = 0xFFFF;

    for (size_t i = 0; i < length; i++)
    {
        crc ^= data[i];
        for (int j = 0; j < 8; j++)
        {
            if (crc & 0x0001)
            {
                crc = (crc >> 1) ^ 0xA001; /* Reflected polynomial */
            }
            else
            {
                crc = crc >> 1;
            }
        }
    }

    return crc ^ 0xFFFF;
}

// Helper functions for little-endian conversions

// Convert uint16_t to little-endian byte array
static void uint16_to_le(uint16_t value, uint8_t *bytes)
{
    bytes[0] = value & 0xFF;
    bytes[1] = (value >> 8) & 0xFF;
}

// Convert little-endian byte array to uint16_t
static uint16_t le_to_uint16(const uint8_t *bytes)
{
    return (uint16_t)bytes[0] | ((uint16_t)bytes[1] << 8);
}

// Convert little-endian byte array to int16_t
static int16_t le_to_int16(const uint8_t *bytes) { return (int16_t)le_to_uint16(bytes); }

// Convert int16_t to little-endian byte array
static void int16_to_le(int16_t value, uint8_t *bytes) { uint16_to_le((uint16_t)value, bytes); }

// Unmarshalls a command frame from raw bytes to a CommandPayload structure
// Returns 0 on success, -1 on error
int unmarshall_command_frame(const uint8_t *data, size_t len, CommandPayload *cmd)
{
    if (!data || !cmd || len < 10)
    { /* Minimum: 1+2+5+2 = 10 bytes */
        return -1;
    }

    // Extract frame header
    uint8_t msg_type = data[0];
    uint16_t payload_size = le_to_uint16(&data[1]);

    if (msg_type != MSG_TYPE_COMMAND)
    {
        fprintf(stderr,
                "Invalid message type: 0x%02X (expected 0x%02X)\n",
                msg_type,
                MSG_TYPE_COMMAND);
        return -1;
    }

    if (payload_size != sizeof(CommandPayload))
    {
        fprintf(stderr,
                "Invalid payload size: %u (expected %zu)\n",
                payload_size,
                sizeof(CommandPayload));
        return -1;
    }

    // Check total frame size: 1 (msg_type) + 2 (size) + payload_size + 2 (crc)
    size_t expected_len = 1 + 2 + payload_size + 2;
    if (len < expected_len)
    {
        fprintf(stderr, "Frame too short: %zu bytes (expected %zu)\n", len, expected_len);
        return -1;
    }

    // Verify CRC
    uint16_t received_crc = le_to_uint16(&data[3 + payload_size]);
    uint16_t calculated_crc = crc16_usb(data, 3 + payload_size);
    if (received_crc != calculated_crc)
    {
        fprintf(stderr,
                "CRC mismatch: received 0x%04X, calculated 0x%04X\n",
                received_crc,
                calculated_crc);
        return -1;
    }

    // Extract payload: action (1), temperature (2), duration (2)
    const uint8_t *payload = &data[3];
    cmd->action = payload[0];
    cmd->temperature = le_to_int16(&payload[1]);
    cmd->duration = le_to_int16(&payload[3]);

    return 0;
}

// Marshalls an event frame from event payload structure to raw bytes
// Returns frame size on success, -1 on error
int marshall_event_frame(const EventPayload *event, uint8_t *buffer, size_t buffer_size)
{
    if (!event || !buffer)
    {
        return -1;
    }

    size_t payload_size = sizeof(EventPayload);
    size_t frame_size = 1 + 2 + payload_size + 2; // msg_type + size + payload + crc
    if (buffer_size < frame_size)
    {
        fprintf(stderr, "Buffer too small: %zu bytes (need %zu)\n", buffer_size, frame_size);
        return -1;
    }

    // Build frame header
    buffer[0] = MSG_TYPE_EVENT;
    uint16_to_le((uint16_t)payload_size, &buffer[1]);

    // Build payload: state (1), current_temperature (2), remaining_time (2),
    // programmed_duration (2), programmed_temperature (2)
    uint8_t *payload = &buffer[3];
    payload[0] = event->state;
    int16_to_le(event->current_temperature, &payload[1]);
    int16_to_le(event->remaining_time, &payload[3]);
    int16_to_le(event->programmed_duration, &payload[5]);
    int16_to_le(event->programmed_temperature, &payload[7]);

    // Calculate and append CRC
    uint16_t crc = crc16_usb(buffer, 3 + payload_size);
    uint16_to_le(crc, &buffer[3 + payload_size]);

    return (int)frame_size;
}

// Helper to print frame in hex format (for debugging)
void print_frame_hex(const uint8_t *data, size_t len)
{
    printf("Frame (%zu bytes): ", len);
    for (size_t i = 0; i < len; i++)
    {
        printf("%02X", data[i]);
    }
    printf("\n");
}

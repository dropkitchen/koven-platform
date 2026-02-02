#include "../external/unity.h"
#include "../koven.h"
#include "../protocol.h"
#include <string.h>

void setUp(void) {}

void tearDown(void) {}

void test_crc16_usb_known_values(void)
{
    // Test with known values from CRC-16/USB standard
    // "123456789" should produce 0xB4C8
    const uint8_t test_data[] = "123456789";
    uint16_t crc = crc16_usb(test_data, 9);
    TEST_ASSERT_EQUAL_HEX16(0xB4C8, crc);
}

void test_crc16_usb_empty_data(void)
{
    uint16_t crc = crc16_usb(NULL, 0);
    // With length 0, the CRC should return the initial value XORed with output XOR
    TEST_ASSERT_EQUAL_HEX16(0x0000, crc);
}

void test_crc16_usb_single_byte(void)
{
    const uint8_t data[] = {0xAA};
    uint16_t crc = crc16_usb(data, 1);
    // Just verify it doesn't crash and returns a value
    TEST_ASSERT_NOT_EQUAL(0, crc);
}

void test_crc16_usb_all_zeros(void)
{
    const uint8_t data[10] = {0};
    uint16_t crc = crc16_usb(data, 10);
    TEST_ASSERT_NOT_EQUAL(0, crc);
}

void test_crc16_usb_all_ones(void)
{
    const uint8_t data[10] = {0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF};
    uint16_t crc = crc16_usb(data, 10);
    TEST_ASSERT_NOT_EQUAL(0, crc);
}

void test_crc16_usb_deterministic(void)
{
    const uint8_t data[] = {0x01, 0x02, 0x03, 0x04, 0x05};
    uint16_t crc1 = crc16_usb(data, 5);
    uint16_t crc2 = crc16_usb(data, 5);
    TEST_ASSERT_EQUAL_HEX16(crc1, crc2);
}

void test_unmarshall_command_frame_valid_start(void)
{
    // Create a valid START command frame
    // Frame: [msg_type:1][size:2][action:1][temp:2][duration:2][crc:2]
    uint8_t frame[10];
    frame[0] = MSG_TYPE_COMMAND; // msg_type
    frame[1] = 0x05;             // size low byte (5 bytes payload)
    frame[2] = 0x00;             // size high byte
    frame[3] = ACTION_START;     // action
    frame[4] = 0xC8;             // temperature low byte (200°C = 0x00C8)
    frame[5] = 0x00;             // temperature high byte
    frame[6] = 0xE0;             // duration low byte (480s = 0x01E0)
    frame[7] = 0x01;             // duration high byte

    // Calculate CRC over first 8 bytes
    uint16_t crc = crc16_usb(frame, 8);
    frame[8] = crc & 0xFF;        // crc low byte
    frame[9] = (crc >> 8) & 0xFF; // crc high byte

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);

    TEST_ASSERT_EQUAL_INT(0, result);
    TEST_ASSERT_EQUAL_UINT8(ACTION_START, cmd.action);
    TEST_ASSERT_EQUAL_INT16(200, cmd.temperature);
    TEST_ASSERT_EQUAL_INT16(480, cmd.duration);
}

void test_unmarshall_command_frame_valid_stop(void)
{
    // Create a valid STOP command frame
    uint8_t frame[10];
    frame[0] = MSG_TYPE_COMMAND;
    frame[1] = 0x05;
    frame[2] = 0x00;
    frame[3] = ACTION_STOP;
    frame[4] = 0x00; // temperature (ignored for STOP)
    frame[5] = 0x00;
    frame[6] = 0x00; // duration (ignored for STOP)
    frame[7] = 0x00;

    uint16_t crc = crc16_usb(frame, 8);
    frame[8] = crc & 0xFF;
    frame[9] = (crc >> 8) & 0xFF;

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);

    TEST_ASSERT_EQUAL_INT(0, result);
    TEST_ASSERT_EQUAL_UINT8(ACTION_STOP, cmd.action);
}

void test_unmarshall_command_frame_negative_temperature(void)
{
    // Test with negative temperature value
    uint8_t frame[10];
    frame[0] = MSG_TYPE_COMMAND;
    frame[1] = 0x05;
    frame[2] = 0x00;
    frame[3] = ACTION_START;
    frame[4] = 0xF6; // -10 in int16_t little-endian (0xFFF6)
    frame[5] = 0xFF;
    frame[6] = 0x2C; // 300 seconds (0x012C)
    frame[7] = 0x01;

    uint16_t crc = crc16_usb(frame, 8);
    frame[8] = crc & 0xFF;
    frame[9] = (crc >> 8) & 0xFF;

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);

    TEST_ASSERT_EQUAL_INT(0, result);
    TEST_ASSERT_EQUAL_INT16(-10, cmd.temperature);
}

void test_unmarshall_command_frame_null_data(void)
{
    CommandPayload cmd;
    int result = unmarshall_command_frame(NULL, 10, &cmd);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_unmarshall_command_frame_null_cmd(void)
{
    uint8_t frame[10] = {0};
    int result = unmarshall_command_frame(frame, 10, NULL);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_unmarshall_command_frame_too_short(void)
{
    uint8_t frame[5] = {MSG_TYPE_COMMAND, 0x05, 0x00, ACTION_START, 0x00};
    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 5, &cmd);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_unmarshall_command_frame_invalid_msg_type(void)
{
    uint8_t frame[10];
    frame[0] = 0xFF; // Invalid message type
    frame[1] = 0x05;
    frame[2] = 0x00;
    frame[3] = ACTION_START;
    frame[4] = 0xC8;
    frame[5] = 0x00;
    frame[6] = 0xE0;
    frame[7] = 0x01;

    uint16_t crc = crc16_usb(frame, 8);
    frame[8] = crc & 0xFF;
    frame[9] = (crc >> 8) & 0xFF;

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_unmarshall_command_frame_invalid_payload_size(void)
{
    uint8_t frame[10];
    frame[0] = MSG_TYPE_COMMAND;
    frame[1] = 0x0A; // Wrong size (10 instead of 5)
    frame[2] = 0x00;
    frame[3] = ACTION_START;
    frame[4] = 0xC8;
    frame[5] = 0x00;
    frame[6] = 0xE0;
    frame[7] = 0x01;

    uint16_t crc = crc16_usb(frame, 8);
    frame[8] = crc & 0xFF;
    frame[9] = (crc >> 8) & 0xFF;

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_unmarshall_command_frame_crc_mismatch(void)
{
    uint8_t frame[10];
    frame[0] = MSG_TYPE_COMMAND;
    frame[1] = 0x05;
    frame[2] = 0x00;
    frame[3] = ACTION_START;
    frame[4] = 0xC8;
    frame[5] = 0x00;
    frame[6] = 0xE0;
    frame[7] = 0x01;
    frame[8] = 0xFF; // Wrong CRC
    frame[9] = 0xFF;

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_unmarshall_command_frame_boundary_values(void)
{
    // Test with maximum positive int16_t values
    uint8_t frame[10];
    frame[0] = MSG_TYPE_COMMAND;
    frame[1] = 0x05;
    frame[2] = 0x00;
    frame[3] = ACTION_START;
    frame[4] = 0xFF; // 32767 (0x7FFF)
    frame[5] = 0x7F;
    frame[6] = 0xFF; // 32767 (0x7FFF)
    frame[7] = 0x7F;

    uint16_t crc = crc16_usb(frame, 8);
    frame[8] = crc & 0xFF;
    frame[9] = (crc >> 8) & 0xFF;

    CommandPayload cmd;
    int result = unmarshall_command_frame(frame, 10, &cmd);

    TEST_ASSERT_EQUAL_INT(0, result);
    TEST_ASSERT_EQUAL_INT16(32767, cmd.temperature);
    TEST_ASSERT_EQUAL_INT16(32767, cmd.duration);
}

void test_marshall_event_frame_idle_state(void)
{
    EventPayload event;
    event.state = STATE_IDLE;
    event.current_temperature = 25;
    event.remaining_time = -1;
    event.programmed_duration = -1;
    event.programmed_temperature = -1;

    uint8_t buffer[20];
    int result = marshall_event_frame(&event, buffer, 20);

    TEST_ASSERT_GREATER_THAN(0, result);
    TEST_ASSERT_EQUAL_UINT8(MSG_TYPE_EVENT, buffer[0]);
    TEST_ASSERT_EQUAL_UINT8(9, buffer[1]); // sizeof(EventPayload) = 9
    TEST_ASSERT_EQUAL_UINT8(0, buffer[2]);
    TEST_ASSERT_EQUAL_UINT8(STATE_IDLE, buffer[3]);

    // Verify CRC is correct
    uint16_t expected_crc = crc16_usb(buffer, 3 + 9);
    uint16_t actual_crc = buffer[12] | (buffer[13] << 8);
    TEST_ASSERT_EQUAL_HEX16(expected_crc, actual_crc);
}

void test_marshall_event_frame_preheating_state(void)
{
    EventPayload event;
    event.state = STATE_PREHEATING;
    event.current_temperature = 100;
    event.remaining_time = -1;
    event.programmed_duration = 600;
    event.programmed_temperature = 180;

    uint8_t buffer[20];
    int result = marshall_event_frame(&event, buffer, 20);

    TEST_ASSERT_GREATER_THAN(0, result);
    TEST_ASSERT_EQUAL_UINT8(STATE_PREHEATING, buffer[3]);

    // Verify the frame size is correct (1 + 2 + 9 + 2 = 14)
    TEST_ASSERT_EQUAL_INT(14, result);
}

void test_marshall_event_frame_baking_state(void)
{
    EventPayload event;
    event.state = STATE_BAKING;
    event.current_temperature = 180;
    event.remaining_time = 300;
    event.programmed_duration = 600;
    event.programmed_temperature = 180;

    uint8_t buffer[20];
    int result = marshall_event_frame(&event, buffer, 20);

    TEST_ASSERT_GREATER_THAN(0, result);
    TEST_ASSERT_EQUAL_UINT8(STATE_BAKING, buffer[3]);

    // Decode and verify temperature field (little-endian at offset 4)
    int16_t decoded_temp = buffer[4] | (buffer[5] << 8);
    TEST_ASSERT_EQUAL_INT16(180, decoded_temp);

    // Decode and verify remaining time (little-endian at offset 6)
    int16_t decoded_time = buffer[6] | (buffer[7] << 8);
    TEST_ASSERT_EQUAL_INT16(300, decoded_time);
}

void test_marshall_event_frame_cooling_down_state(void)
{
    EventPayload event;
    event.state = STATE_COOLING_DOWN;
    event.current_temperature = 80;
    event.remaining_time = -1;
    event.programmed_duration = -1;
    event.programmed_temperature = -1;

    uint8_t buffer[20];
    int result = marshall_event_frame(&event, buffer, 20);

    TEST_ASSERT_GREATER_THAN(0, result);
    TEST_ASSERT_EQUAL_UINT8(STATE_COOLING_DOWN, buffer[3]);
}

void test_marshall_event_frame_null_event(void)
{
    uint8_t buffer[20];
    int result = marshall_event_frame(NULL, buffer, 20);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_marshall_event_frame_null_buffer(void)
{
    EventPayload event;
    event.state = STATE_IDLE;
    event.current_temperature = 25;
    event.remaining_time = -1;
    event.programmed_duration = -1;
    event.programmed_temperature = -1;

    int result = marshall_event_frame(&event, NULL, 20);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_marshall_event_frame_buffer_too_small(void)
{
    EventPayload event;
    event.state = STATE_IDLE;
    event.current_temperature = 25;
    event.remaining_time = -1;
    event.programmed_duration = -1;
    event.programmed_temperature = -1;

    uint8_t buffer[5]; // Too small
    int result = marshall_event_frame(&event, buffer, 5);
    TEST_ASSERT_EQUAL_INT(-1, result);
}

void test_marshall_event_frame_boundary_values(void)
{
    EventPayload event;
    event.state = STATE_BAKING;
    event.current_temperature = 32767; // Max int16_t
    event.remaining_time = 32767;
    event.programmed_duration = 32767;
    event.programmed_temperature = 32767;

    uint8_t buffer[20];
    int result = marshall_event_frame(&event, buffer, 20);

    TEST_ASSERT_GREATER_THAN(0, result);

    // Verify values are encoded correctly
    int16_t decoded_temp = buffer[4] | (buffer[5] << 8);
    TEST_ASSERT_EQUAL_INT16(32767, decoded_temp);
}

void test_marshall_event_frame_negative_values(void)
{
    EventPayload event;
    event.state = STATE_IDLE;
    event.current_temperature = -10;
    event.remaining_time = -1;
    event.programmed_duration = -1;
    event.programmed_temperature = -1;

    uint8_t buffer[20];
    int result = marshall_event_frame(&event, buffer, 20);

    TEST_ASSERT_GREATER_THAN(0, result);

    // Verify negative value is encoded correctly
    int16_t decoded_temp = (int16_t)(buffer[4] | (buffer[5] << 8));
    TEST_ASSERT_EQUAL_INT16(-10, decoded_temp);
}

int main(void)
{
    UNITY_BEGIN();

    // CRC Tests
    RUN_TEST(test_crc16_usb_known_values);
    RUN_TEST(test_crc16_usb_empty_data);
    RUN_TEST(test_crc16_usb_single_byte);
    RUN_TEST(test_crc16_usb_all_zeros);
    RUN_TEST(test_crc16_usb_all_ones);
    RUN_TEST(test_crc16_usb_deterministic);

    // Unmarshall Command Frame Tests
    RUN_TEST(test_unmarshall_command_frame_valid_start);
    RUN_TEST(test_unmarshall_command_frame_valid_stop);
    RUN_TEST(test_unmarshall_command_frame_negative_temperature);
    RUN_TEST(test_unmarshall_command_frame_null_data);
    RUN_TEST(test_unmarshall_command_frame_null_cmd);
    RUN_TEST(test_unmarshall_command_frame_too_short);
    RUN_TEST(test_unmarshall_command_frame_invalid_msg_type);
    RUN_TEST(test_unmarshall_command_frame_invalid_payload_size);
    RUN_TEST(test_unmarshall_command_frame_crc_mismatch);
    RUN_TEST(test_unmarshall_command_frame_boundary_values);

    // Marshall Event Frame Tests
    RUN_TEST(test_marshall_event_frame_idle_state);
    RUN_TEST(test_marshall_event_frame_preheating_state);
    RUN_TEST(test_marshall_event_frame_baking_state);
    RUN_TEST(test_marshall_event_frame_cooling_down_state);
    RUN_TEST(test_marshall_event_frame_null_event);
    RUN_TEST(test_marshall_event_frame_null_buffer);
    RUN_TEST(test_marshall_event_frame_buffer_too_small);
    RUN_TEST(test_marshall_event_frame_boundary_values);
    RUN_TEST(test_marshall_event_frame_negative_values);

    return UNITY_END();
}

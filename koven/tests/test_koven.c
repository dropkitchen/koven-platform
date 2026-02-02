#include "../external/unity.h"
#include "../koven.h"
#include <string.h>

void setUp(void) {}

void tearDown(void) {}

void assert_koven_state(Koven *koven,
                        int expected_state,
                        int expected_current_temp,
                        int expected_remaining_time,
                        int expected_programmed_temp,
                        int expected_programmed_duration)
{
    TEST_ASSERT_NOT_NULL(koven);
    TEST_ASSERT_EQUAL_INT(expected_state, koven->state);
    TEST_ASSERT_EQUAL_INT16(expected_current_temp, koven->current_temperature);
    TEST_ASSERT_EQUAL_INT16(expected_remaining_time, koven->remaining_time);
    TEST_ASSERT_EQUAL_INT16(expected_programmed_temp, koven->programmed_temperature);
    TEST_ASSERT_EQUAL_INT16(expected_programmed_duration, koven->programmed_duration);
}

void assert_event_payload(EventPayload *event,
                          int expected_state,
                          int expected_current_temp,
                          int expected_remaining_time,
                          int expected_programmed_temp,
                          int expected_programmed_duration)
{
    TEST_ASSERT_NOT_NULL(event);
    TEST_ASSERT_EQUAL_INT(expected_state, event->state);
    TEST_ASSERT_EQUAL_INT16(expected_current_temp, event->current_temperature);
    TEST_ASSERT_EQUAL_INT16(expected_remaining_time, event->remaining_time);
    TEST_ASSERT_EQUAL_INT16(expected_programmed_temp, event->programmed_temperature);
    TEST_ASSERT_EQUAL_INT16(expected_programmed_duration, event->programmed_duration);
}

void test_koven_init_state(void)
{
    Koven koven;
    koven_init(&koven);

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
}

void test_koven_init_from_dirty_state(void)
{
    Koven koven;
    koven.state = STATE_BAKING;
    koven.current_temperature = 200;
    koven.remaining_time = 100;
    koven.programmed_duration = 600;
    koven.programmed_temperature = 180;

    koven_init(&koven);

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
}

void test_koven_execute_start_from_idle(void)
{
    Koven koven;
    koven_init(&koven);

    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 180;
    cmd.duration = 600;

    koven_execute(&koven, &cmd);

    assert_koven_state(&koven, STATE_PREHEATING, 25, -1, 180, 600);
}

void test_koven_execute_start_while_preheating_ignored(void)
{
    Koven koven;
    koven_init(&koven);
    koven.state = STATE_PREHEATING;
    koven.programmed_temperature = 180;
    koven.programmed_duration = 600;

    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 200;
    cmd.duration = 300;

    koven_execute(&koven, &cmd);

    // Should still be in preheating with original values
    assert_koven_state(&koven, STATE_PREHEATING, 25, -1, 180, 600);
}

void test_koven_execute_start_while_baking_ignored(void)
{
    Koven koven;
    koven_init(&koven);
    koven.state = STATE_BAKING;
    koven.current_temperature = 180;
    koven.remaining_time = 300;
    koven.programmed_temperature = 180;
    koven.programmed_duration = 600;

    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 200;
    cmd.duration = 400;

    koven_execute(&koven, &cmd);

    // Should still be baking with original values
    assert_koven_state(&koven, STATE_BAKING, 180, 300, 180, 600);
}

void test_koven_execute_null_koven(void)
{
    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 180;
    cmd.duration = 600;

    // Should not crash
    koven_execute(NULL, &cmd);
    TEST_ASSERT_TRUE(1); // If we get here, it didn't crash
}

void test_koven_execute_null_command(void)
{
    Koven koven;
    koven_init(&koven);

    // Should not crash
    koven_execute(&koven, NULL);
    TEST_ASSERT_TRUE(1); // If we get here, it didn't crash
}

void test_koven_execute_invalid_action_ignored(void)
{
    Koven koven;
    koven_init(&koven);

    CommandPayload cmd;
    cmd.action = 99; // Invalid action
    cmd.temperature = 180;
    cmd.duration = 600;

    koven_execute(&koven, &cmd);

    // Should remain in IDLE state
    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
}

void test_koven_tick_idle_no_change(void)
{
    Koven koven;
    koven_init(&koven);

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);

    EventPayload event;
    for (int i = 0; i < 10; i++)
    {
        koven_tick(&koven, &event);
    }

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
    assert_event_payload(&event, STATE_IDLE, 25, -1, -1, -1);
}

void test_koven_tick_preheating_increases_temperature(void)
{
    Koven koven;
    koven_init(&koven);

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);

    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 90;
    cmd.duration = 600;
    koven_execute(&koven, &cmd);

    EventPayload event;
    for (int i = 1; i <= 10; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_PREHEATING, 25 + i, -1, 90, 600);
        assert_event_payload(&event, STATE_PREHEATING, 25 + i, -1, 90, 600);
    }
}

void test_koven_tick_preheating_reaches_target_transitions_to_baking_and_decreases_remaining_time(
    void)
{
    Koven koven;
    koven_init(&koven);

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);

    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 30;
    cmd.duration = 600;
    koven_execute(&koven, &cmd);

    EventPayload event;

    // Heat up towards target (5 ticks gets us to 30°C)
    for (int i = 1; i <= 5; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_PREHEATING, 25 + i, -1, 30, 600);
        assert_event_payload(&event, STATE_PREHEATING, 25 + i, -1, 30, 600);
    }

    // Five more ticks at 30°C transitions to baking and starts counting down time
    for (int i = 0; i < 5; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_BAKING, 30, 600 - i, 30, 600);
        assert_event_payload(&event, STATE_BAKING, 30, 600 - i, 30, 600);
    }
}

void test_koven_tick_baking_completes_transitions_to_cooling(void)
{
    Koven koven;
    koven_init(&koven);
    koven.state = STATE_BAKING;
    koven.current_temperature = 180;
    koven.remaining_time = 1;
    koven.programmed_temperature = 180;
    koven.programmed_duration = 600;

    EventPayload event;

    // First tick decrements time to 0 but still baking
    koven_tick(&koven, &event);

    assert_koven_state(&koven, STATE_BAKING, 180, 0, 180, 600);
    assert_event_payload(&event, STATE_BAKING, 180, 0, 180, 600);

    // Second tick with time=0 transitions to cooling
    koven_tick(&koven, &event);

    assert_koven_state(&koven, STATE_COOLING_DOWN, 180, 0, -1, -1);
    assert_event_payload(&event, STATE_COOLING_DOWN, 180, 0, -1, -1);
}

void test_koven_tick_cooling_down_decreases_temperature(void)
{
    Koven koven;
    koven_init(&koven);
    koven.state = STATE_COOLING_DOWN;
    koven.current_temperature = 180;
    koven.remaining_time = 0;
    koven.programmed_temperature = -1;
    koven.programmed_duration = -1;

    EventPayload event;
    for (int i = 1; i <= 5; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_COOLING_DOWN, 180 - i, 0, -1, -1);
        assert_event_payload(&event, STATE_COOLING_DOWN, 180 - i, 0, -1, -1);
    }
}

void test_koven_tick_cooling_down_reaches_room_temp_transitions_to_idle(void)
{
    Koven koven;
    koven_init(&koven);
    koven.state = STATE_COOLING_DOWN;
    koven.remaining_time = 0;
    koven.current_temperature = 26;

    EventPayload event;

    // First tick cools down to 25 but still cooling
    koven_tick(&koven, &event);

    assert_koven_state(&koven, STATE_COOLING_DOWN, 25, 0, -1, -1);
    assert_event_payload(&event, STATE_COOLING_DOWN, 25, 0, -1, -1);

    // Second tick at room temp transitions to idle
    koven_tick(&koven, &event);

    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
    assert_event_payload(&event, STATE_IDLE, 25, -1, -1, -1);
}

void test_koven_tick_null_koven(void)
{
    EventPayload event;

    // Should not crash
    koven_tick(NULL, &event);
    TEST_ASSERT_TRUE(1);
}

void test_koven_tick_null_event(void)
{
    Koven koven;
    koven_init(&koven);

    // Should not crash
    koven_tick(&koven, NULL);
    TEST_ASSERT_EQUAL_INT(STATE_IDLE, koven.state);
}

void test_complete_workflow(void)
{
    Koven koven;
    koven_init(&koven);

    // Start baking at 30°C for 3 seconds
    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 30;
    cmd.duration = 3;
    koven_execute(&koven, &cmd);

    EventPayload event;

    // Preheat (5 ticks gets to 30°C)
    for (int i = 1; i <= 5; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_PREHEATING, 25 + i, -1, 30, 3);
        assert_event_payload(&event, STATE_PREHEATING, 25 + i, -1, 30, 3);
    }

    // 6th tick at target temp transitions to baking
    koven_tick(&koven, &event);

    assert_koven_state(&koven, STATE_BAKING, 30, 3, 30, 3);
    assert_event_payload(&event, STATE_BAKING, 30, 3, 30, 3);

    // Bake for 3 seconds (counts down to 0, then one more tick to transition)
    for (int i = 1; i <= 3; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_BAKING, 30, 3 - i, 30, 3);
        assert_event_payload(&event, STATE_BAKING, 30, 3 - i, 30, 3);
    }

    // One more tick with time=0 transitions to cooling
    koven_tick(&koven, &event);
    assert_koven_state(&koven, STATE_COOLING_DOWN, 30, 0, -1, -1);
    assert_event_payload(&event, STATE_COOLING_DOWN, 30, 0, -1, -1);

    // Cool down (5 ticks to go from 30 to 25, then one more to transition)
    for (int i = 1; i <= 5; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_COOLING_DOWN, 30 - i, 0, -1, -1);
        assert_event_payload(&event, STATE_COOLING_DOWN, 30 - i, 0, -1, -1);
    }

    // One more tick at room temp transitions to idle
    koven_tick(&koven, &event);
    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
    assert_event_payload(&event, STATE_IDLE, 25, -1, -1, -1);
}

void test_stop_during_preheating_should_cool_down(void)
{
    Koven koven;
    koven_init(&koven);

    CommandPayload cmd;
    cmd.action = ACTION_START;
    cmd.temperature = 180;
    cmd.duration = 600;
    koven_execute(&koven, &cmd);

    EventPayload event;

    // Preheat for a while to heat up significantly
    for (int i = 1; i <= 50; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_PREHEATING, 25 + i, -1, 180, 600);
        assert_event_payload(&event, STATE_PREHEATING, 25 + i, -1, 180, 600);
    }

    // Stop the oven while it's hot
    cmd.action = ACTION_STOP;
    koven_execute(&koven, &cmd);

    assert_koven_state(&koven, STATE_COOLING_DOWN, 75, -1, -1, -1);

    // Verify it eventually cools down to room temperature
    for (int i = 1; i <= 50; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_COOLING_DOWN, 75 - i, -1, -1, -1);
        assert_event_payload(&event, STATE_COOLING_DOWN, 75 - i, -1, -1, -1);
    }

    koven_tick(&koven, &event);
    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
    assert_event_payload(&event, STATE_IDLE, 25, -1, -1, -1);
}

void test_stop_during_baking_should_cool_down(void)
{
    Koven koven;
    koven_init(&koven);

    // Set up oven in BAKING state at high temperature
    koven.state = STATE_BAKING;
    koven.current_temperature = 75;
    koven.remaining_time = 300;
    koven.programmed_temperature = 75;
    koven.programmed_duration = 600;

    CommandPayload cmd;
    cmd.action = ACTION_STOP;
    koven_execute(&koven, &cmd);

    assert_koven_state(&koven, STATE_COOLING_DOWN, 75, -1, -1, -1);

    EventPayload event;
    // Verify it eventually cools down to room temperature
    for (int i = 1; i <= 50; i++)
    {
        koven_tick(&koven, &event);

        assert_koven_state(&koven, STATE_COOLING_DOWN, 75 - i, -1, -1, -1);
        assert_event_payload(&event, STATE_COOLING_DOWN, 75 - i, -1, -1, -1);
    }

    koven_tick(&koven, &event);
    assert_koven_state(&koven, STATE_IDLE, 25, -1, -1, -1);
    assert_event_payload(&event, STATE_IDLE, 25, -1, -1, -1);
}

void test_state_to_string_all_states(void)
{
    TEST_ASSERT_EQUAL_STRING("idle", state_to_string(STATE_IDLE));
    TEST_ASSERT_EQUAL_STRING("preheating", state_to_string(STATE_PREHEATING));
    TEST_ASSERT_EQUAL_STRING("baking", state_to_string(STATE_BAKING));
    TEST_ASSERT_EQUAL_STRING("cooling down", state_to_string(STATE_COOLING_DOWN));
}

void test_state_to_string_invalid_state(void)
{
    TEST_ASSERT_EQUAL_STRING("unknown", state_to_string(99));
}

void test_action_to_string_all_actions(void)
{
    TEST_ASSERT_EQUAL_STRING("start", action_to_string(ACTION_START));
    TEST_ASSERT_EQUAL_STRING("stop", action_to_string(ACTION_STOP));
}

void test_action_to_string_invalid_action(void)
{
    TEST_ASSERT_EQUAL_STRING("unknown", action_to_string(99));
}

int main(void)
{
    UNITY_BEGIN();

    // koven_init Tests
    RUN_TEST(test_koven_init_state);
    RUN_TEST(test_koven_init_from_dirty_state);

    // koven_execute Tests - ACTION_START
    RUN_TEST(test_koven_execute_start_from_idle);
    RUN_TEST(test_koven_execute_start_while_preheating_ignored);
    RUN_TEST(test_koven_execute_start_while_baking_ignored);

    // koven_execute Tests - NULL handling
    RUN_TEST(test_koven_execute_null_koven);
    RUN_TEST(test_koven_execute_null_command);
    RUN_TEST(test_koven_execute_invalid_action_ignored);

    // koven_tick Tests - IDLE state
    RUN_TEST(test_koven_tick_idle_no_change);

    // koven_tick Tests - PREHEATING state
    RUN_TEST(test_koven_tick_preheating_increases_temperature);
    RUN_TEST(
        test_koven_tick_preheating_reaches_target_transitions_to_baking_and_decreases_remaining_time);

    // koven_tick Tests - BAKING state
    RUN_TEST(test_koven_tick_baking_completes_transitions_to_cooling);

    // koven_tick Tests - COOLING_DOWN state
    RUN_TEST(test_koven_tick_cooling_down_decreases_temperature);
    RUN_TEST(test_koven_tick_cooling_down_reaches_room_temp_transitions_to_idle);

    // koven_tick Tests - NULL handling
    RUN_TEST(test_koven_tick_null_koven);
    RUN_TEST(test_koven_tick_null_event);

    // Complete Workflow Tests
    RUN_TEST(test_complete_workflow);

    // TODO: we need to fix these scenarios
    RUN_TEST(test_stop_during_preheating_should_cool_down);
    RUN_TEST(test_stop_during_baking_should_cool_down);

    // Helper function tests
    RUN_TEST(test_state_to_string_all_states);
    RUN_TEST(test_state_to_string_invalid_state);
    RUN_TEST(test_action_to_string_all_actions);
    RUN_TEST(test_action_to_string_invalid_action);

    return UNITY_END();
}

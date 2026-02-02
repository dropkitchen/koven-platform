#include "koven.h"
#include <stdio.h>
#include <string.h>

#define ROOM_TEMPERATURE 25
// Initialize the Koven state to default values
// Starts in IDLE state at room temperature (25°C)
// No programmed temperature or duration (-1)
void koven_init(Koven *koven)
{
    koven->state = STATE_IDLE;
    koven->current_temperature = ROOM_TEMPERATURE; // Start at room temperature
    koven->remaining_time = -1;
    koven->programmed_duration = -1;
    koven->programmed_temperature = -1;
}

// Execute a command on the Koven
// Handles ACTION_START and ACTION_STOP
// Ignores invalid commands for the current state
// For ACTION_START, sets the programmed temperature and duration
// For ACTION_STOP, resets the state to IDLE but keeps current temperature
void koven_execute(Koven *koven, const CommandPayload *cmd)
{
    if (!koven || !cmd)
    {
        return;
    }

    switch (cmd->action)
    {
    case ACTION_START:
        if (koven->state == STATE_IDLE)
        {
            koven->state = STATE_PREHEATING;
            koven->programmed_duration = cmd->duration;
            koven->programmed_temperature = cmd->temperature;
        }
        break;

    case ACTION_STOP:
        koven->state = STATE_IDLE;
        koven->remaining_time = -1;
        koven->programmed_temperature = -1;
        koven->programmed_duration = -1;
        break;

    default:
        break;
    }
}

// Simulate a time tick for the Koven it represents one second passing
// Updates the current temperature and remaining time based on the state
// Builds an event payload reflecting the current state after the tick
// - In PREHEATING, increases temperature by 1°C per second until target reached
// - In BAKING, decreases remaining time by 1 second until zero
// - When baking completes, resets to IDLE state
void koven_tick(Koven *koven, EventPayload *event)
{
    if (!koven || !event)
    {
        return;
    }

    if (koven->state == STATE_PREHEATING)
    {
        if (koven->current_temperature < koven->programmed_temperature)
        {
            koven->current_temperature += 1; // Heat up 1°C per second
        }
        else
        {
            koven->state = STATE_BAKING;
            koven->remaining_time = koven->programmed_duration;
        }
    }
    else if (koven->state == STATE_BAKING)
    {
        if (koven->remaining_time > 0)
        {
            koven->remaining_time -= 1;
        }
        else
        {
            if (koven->current_temperature > ROOM_TEMPERATURE)
            {
                koven->state = STATE_COOLING_DOWN;
                koven->programmed_temperature = -1;
                koven->programmed_duration = -1;
            }
            else
            {
                koven_init(koven);
            }
        }
    }
    else if (koven->state == STATE_COOLING_DOWN)
    {
        if (koven->current_temperature > ROOM_TEMPERATURE)
        {
            koven->current_temperature -= 1; // Cool down 1°C per second
        }
        else
        {
            koven_init(koven);
        }
    }

    event->state = koven->state;
    event->current_temperature = koven->current_temperature;
    event->remaining_time = koven->remaining_time;
    event->programmed_duration = koven->programmed_duration;
    event->programmed_temperature = koven->programmed_temperature;
}

const char *state_to_string(State state)
{
    switch (state)
    {
    case STATE_IDLE:
        return "idle";
    case STATE_PREHEATING:
        return "preheating";
    case STATE_BAKING:
        return "baking";
    case STATE_COOLING_DOWN:
        return "cooling down";
    default:
        return "unknown";
    }
}

const char *action_to_string(Action action)
{
    switch (action)
    {
    case ACTION_START:
        return "start";
    case ACTION_STOP:
        return "stop";
    default:
        return "unknown";
    }
}

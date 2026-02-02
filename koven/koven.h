#ifndef KOVEN_H
#define KOVEN_H

#include <stdint.h>

// Oven state
typedef enum {
    STATE_IDLE = 0,
    STATE_PREHEATING = 1,
    STATE_BAKING = 2,
    STATE_COOLING_DOWN = 3
} State;

// Actions allowed by the oven
typedef enum {
    ACTION_START = 1,
    ACTION_STOP = 2,
} Action;

// Command payload structure
// It always contains the action to perform, target temperature, and duration
// The temperature and duration fields are ignored for ACTION_STOP
// The temperature is in degrees Celsius and duration in seconds.
typedef struct __attribute__((packed)) {
    uint8_t action;
    int16_t temperature;
    int16_t duration;
} CommandPayload;

// Event payload structure sent by the oven
// -1 value indicates that the field is not applicable for the current state
typedef struct __attribute__((packed)) {
    uint8_t state;
    int16_t current_temperature;
    int16_t remaining_time;
    int16_t programmed_duration;
    int16_t programmed_temperature;
} EventPayload;

// Internal Koven state structure
typedef struct {
    State state;
    int16_t current_temperature;
    int16_t remaining_time;
    int16_t programmed_duration;
    int16_t programmed_temperature;
} Koven;

void koven_init(Koven *koven);
void koven_execute(Koven *koven, const CommandPayload *cmd);
void koven_tick(Koven *koven, EventPayload *event);

const char* state_to_string(State state);
const char* action_to_string(Action action);

#endif /* KOVEN_H */

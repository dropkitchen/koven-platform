#ifndef MQTT_CLIENT_H
#define MQTT_CLIENT_H

#include "koven.h"

#define MQTT_BROKER "mqtt"
#define MQTT_PORT 1883
#define MQTT_CLIENT_ID "koven_client"
#define MQTT_TOPIC_COMMANDS "cmds/koven"
#define MQTT_TOPIC_EVENTS "events/koven"
#define MQTT_QOS 1
#define MQTT_TIMEOUT 10000L

int mqtt_client_run(Koven *koven);

#endif /* MQTT_CLIENT_H */

#include "mqtt_client.h"
#include "protocol.h"
#include <MQTTClient.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static volatile int running = 1;

// Signal handler to gracefully stop the koven emulator
void handle_signal(int sig)
{
    (void)sig;
    running = 0;
}

// Callback for incoming MQTT messages on the subscribed topics
int message_arrived(void *context, char *topicName, int topicLen, MQTTClient_message *message)
{
    (void)topicLen;
    Koven *koven = (Koven *)context;

    if (!message->payload || message->payloadlen == 0)
    {
        MQTTClient_freeMessage(&message);
        MQTTClient_free(topicName);
        return 1;
    }

    printf("Received binary command (%d bytes): ", message->payloadlen);
    print_frame_hex((const uint8_t *)message->payload, message->payloadlen);

    CommandPayload cmd;
    if (unmarshall_command_frame((const uint8_t *)message->payload, message->payloadlen, &cmd) == 0)
    {
        printf("Command parsed: action=%s, temperature=%d°C, duration=%ds\n",
               action_to_string((Action)cmd.action),
               cmd.temperature,
               cmd.duration);

        koven_execute(koven, &cmd);
        printf("Command executed successfully\n");
    }
    else
    {
        printf("Failed to parse command frame\n");
    }

    MQTTClient_freeMessage(&message);
    MQTTClient_free(topicName);

    return 1;
}

// Callback for connection loss with the MQTT broker
void connection_lost(void *context, char *cause)
{
    (void)context;
    printf("Connection lost: %s\n", cause ? cause : "unknown");
    running = 0;
}

// Main function to run the MQTT client loop
int mqtt_client_run(Koven *koven)
{
    MQTTClient client;
    MQTTClient_connectOptions conn_opts = MQTTClient_connectOptions_initializer;
    int rc;

    signal(SIGINT, handle_signal);
    signal(SIGTERM, handle_signal);

    char address[256];
    snprintf(address, sizeof(address), "tcp://%s:%d", MQTT_BROKER, MQTT_PORT);

    MQTTClient_create(&client, address, MQTT_CLIENT_ID, MQTTCLIENT_PERSISTENCE_NONE, NULL);
    MQTTClient_setCallbacks(client, koven, connection_lost, message_arrived, NULL);

    conn_opts.keepAliveInterval = 20;
    conn_opts.cleansession = 1;

    printf("Connecting to MQTT broker at %s...\n", address);
    if ((rc = MQTTClient_connect(client, &conn_opts)) != MQTTCLIENT_SUCCESS)
    {
        printf("Failed to connect to MQTT broker, return code %d\n", rc);
        MQTTClient_destroy(&client);
        return -1;
    }

    printf("Connected to MQTT broker\n");
    printf("Subscribing to topic: %s\n", MQTT_TOPIC_COMMANDS);
    if ((rc = MQTTClient_subscribe(client, MQTT_TOPIC_COMMANDS, MQTT_QOS)) != MQTTCLIENT_SUCCESS)
    {
        printf("Failed to subscribe, return code %d\n", rc);
        MQTTClient_disconnect(client, MQTT_TIMEOUT);
        MQTTClient_destroy(&client);
        return -1;
    }

    printf("Subscribed to %s\n", MQTT_TOPIC_COMMANDS);
    printf("Koven is running...\n");

    while (running)
    {
        sleep(1);

        EventPayload event;
        koven_tick(koven, &event);

        uint8_t frame_buffer[64];
        int frame_size = marshall_event_frame(&event, frame_buffer, sizeof(frame_buffer));

        if (frame_size > 0)
        {
            printf("Publishing event: state=%s, temp=%d°C, remaining=%ds, programmed_temp=%d°C, "
                   "programmed_duration=%ds\n",
                   state_to_string(event.state),
                   event.current_temperature,
                   event.remaining_time,
                   event.programmed_temperature,
                   event.programmed_duration);
            print_frame_hex(frame_buffer, frame_size);

            MQTTClient_message pubmsg = MQTTClient_message_initializer;
            pubmsg.payload = frame_buffer;
            pubmsg.payloadlen = frame_size;
            pubmsg.qos = MQTT_QOS;
            pubmsg.retained = 0;

            MQTTClient_deliveryToken token;
            rc = MQTTClient_publishMessage(client, MQTT_TOPIC_EVENTS, &pubmsg, &token);

            if (rc == MQTTCLIENT_SUCCESS)
            {
                MQTTClient_waitForCompletion(client, token, MQTT_TIMEOUT);
            }
            else
            {
                printf("Failed to publish message, return code %d\n", rc);
            }
        }
        else
        {
            printf("Failed to build event frame\n");
        }
    }

    printf("\nShutting down...\n");
    MQTTClient_unsubscribe(client, MQTT_TOPIC_COMMANDS);
    MQTTClient_disconnect(client, MQTT_TIMEOUT);
    MQTTClient_destroy(&client);

    return 0;
}

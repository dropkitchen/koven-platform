#include "koven.h"
#include "mqtt_client.h"
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char *argv[])
{
    (void)argc;
    (void)argv;

    // Disable stdout buffering for Docker container compatibility
    setbuf(stdout, NULL);
    setbuf(stderr, NULL);

    printf("Starting Koven...\n");

    Koven koven;
    koven_init(&koven);

    int result = mqtt_client_run(&koven);

    if (result != 0)
    {
        fprintf(stderr, "Koven exited with error code: %d\n", result);
        return EXIT_FAILURE;
    }

    printf("Koven stopped successfully\n");
    return EXIT_SUCCESS;
}

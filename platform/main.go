package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dropkitchen/koven-platform/platform/internal/mqtt"
	"github.com/dropkitchen/koven-platform/platform/internal/service"
)

func main() {
	serverAddr := flag.String("addr", "localhost:8080", "HTTP server address")
	mqttBroker := flag.String("mqtt-broker", "tcp://localhost:1883", "MQTT broker URL")
	flag.Parse()

	mqttClient, err := mqtt.NewClient(*mqttBroker, "koven_platform")
	if err != nil {
		log.Fatalf("Failed to create MQTT client: %v", err)
	}

	if err := mqttClient.Connect(); err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}

	svc := service.NewService(*serverAddr, mqttClient)

	httpServer := &http.Server{
		Addr:         *serverAddr,
		Handler:      svc.Routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on %s", *serverAddr)
		serverErrors <- httpServer.ListenAndServe()
	}()

	// Wait for shutdown signal and perform graceful shutdown
	waitForShutdown(httpServer, svc, mqttClient, serverErrors)
}

// waitForShutdown blocks until receiving an interrupt signal or server error,
// then performs graceful shutdown of all components
func waitForShutdown(httpServer *http.Server, svc *service.Service, mqttClient *mqtt.Client, serverErrors <-chan error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	case sig := <-sigChan:
		log.Printf("\nReceived signal: %v. Shutting down gracefully...", sig)
		performShutdown(httpServer, svc, mqttClient)
	}
}

// performShutdown executes the graceful shutdown sequence for all components
func performShutdown(httpServer *http.Server, svc *service.Service, mqttClient *mqtt.Client) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server gracefully
	log.Println("Shutting down HTTP server...")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during HTTP server shutdown: %v", err)
		// Force close if graceful shutdown fails
		if err := httpServer.Close(); err != nil {
			log.Printf("Error forcing HTTP server close: %v", err)
		}
	} else {
		log.Println("HTTP server shutdown complete")
	}

	// Stop the service (closes WebSocket hub and clients)
	log.Println("Stopping service...")
	if err := svc.Close(); err != nil {
		log.Printf("Error closing service: %v", err)
	} else {
		log.Println("Service stopped")
	}

	// Disconnect MQTT client
	log.Println("Disconnecting MQTT client...")
	mqttClient.Disconnect()
	log.Println("MQTT client disconnected")

	log.Println("Shutdown complete")
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Printf("Starting WebSocket server instance: %s\n", serverID)

	if err := manager.connectRabbitMQ(rabbitMQURI); err != nil {
		log.Fatalf("Failed to initialize RabbitMQ: %v. Server cannot start.", err)
	}

	if err := manager.connectRedis(redisAddr); err != nil {
		log.Fatalf("Failed to initialize Redis: %v. Server functions relying on Redis may fail.", err)
	}


	go manager.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWsWithBackplane(&manager, w, r)
	})

	srv := &http.Server{ Addr: ":8080", Handler: nil }

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutdown signal received. Shutting down server...")
		close(manager.shutdownChan)

		ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 35*time.Second) // Increased for backplane cleanup
		defer cancelShutdown()

		if err := srv.Shutdown(ctxShutdown); err != nil {
			log.Fatalf("Server Shutdown Failed:%+v", err)
		}
		log.Println("Server gracefully stopped")
	}()

	log.Println("HTTP server starting on :8080")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe error: %v", err)
	}
	log.Println("Server exiting")
}

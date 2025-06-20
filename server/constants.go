package main

import (
	"os"
	"time"
)

const (
	writeWait            = 10 * time.Second
	pongWait             = 60 * time.Second
	pingPeriod           = (pongWait * 9) / 10
	maxMessageSize       = 512
	broadcastMessageSize = 2048
)

var (
	PORT              = getEnvOrDefault("PORT", ":8080")
	rabbitMQURI       = getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	broadcastExchange = getEnvOrDefault("RABBITMQ_BROADCAST_EXCHANGE", "ws_broadcast_exchange")
	topicExchange     = getEnvOrDefault("RABBITMQ_TOPIC_EXCHANGE", "ws_topic_exchange")
	redisAddr         = getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	serverID          = getEnvOrDefault("SERVER_ID", "server-1")
)

// getEnvOrDefault returns the value of an environment variable or a default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

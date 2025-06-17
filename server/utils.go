package main

import (
	"context"
	"encoding/json"
	"time"
)

type HealthCheckResponse struct {
	IsOK           bool   `json:"is_ok"`
	RedisStatus    bool   `json:"redis_status"`
	RabbitMQStatus bool   `json:"rabbitmq_status"`
	Message        string `json:"message"`
}

func (h HealthCheckResponse) ToJSON() ([]byte, error) {
	return json.Marshal(h)
}

func healthCheck() HealthCheckResponse {
	status := HealthCheckResponse{
		IsOK:           true,
		RedisStatus:    true,
		RabbitMQStatus: true,
		Message:        "All services healthy",
	}

	// Redis ping
	if manager.redisClient == nil {
		status.IsOK = false
		status.RedisStatus = false
		status.Message = "Redis client not initialized"
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := manager.redisClient.Ping(ctx).Err(); err != nil {
			status.IsOK = false
			status.RedisStatus = false
			if status.Message == "All services healthy" {
				status.Message = "Redis connection failed"
			} else {
				status.Message += "; Redis connection failed"
			}
		}
	}

	// RabbitMQ ping
	if manager.amqpConn == nil || manager.amqpChannel == nil {
		status.IsOK = false
		status.RabbitMQStatus = false
		if status.Message == "All services healthy" {
			status.Message = "RabbitMQ connection not initialized"
		} else {
			status.Message += "; RabbitMQ connection not initialized"
		}
	} else if manager.amqpConn.IsClosed() {
		status.IsOK = false
		status.RabbitMQStatus = false
		if status.Message == "All services healthy" {
			status.Message = "RabbitMQ connection closed"
		} else {
			status.Message += "; RabbitMQ connection closed"
		}
	}

	return status
}

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8" // Redis client
)

var (
	ctx      = context.Background()                            // Context for Redis
	serverID = fmt.Sprintf("server-%d", time.Now().UnixNano()) // Unique ID for this server instance
)

// --- Redis Helper Functions ---
func (m *Manager) connectRedis(addr string) error {
	m.redisClient = redis.NewClient(&redis.Options{
		Addr: addr,
	})
	_, err := m.redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Println("Redis connected.")
	return nil
}

// Presence Tracking with Redis
func (m *Manager) setUserPresence(clientID string, serverID string) {
	if m.redisClient == nil {
		return
	}
	// Track which server a user is on
	err := m.redisClient.Set(ctx, "user:"+clientID+":server", serverID, 0).Err()
	if err != nil {
		log.Printf("Redis: Failed to set user server: %v\n", err)
	}
	// Add user to this server's set of active users
	err = m.redisClient.SAdd(ctx, "server:"+serverID+":users", clientID).Err()
	if err != nil {
		log.Printf("Redis: Failed to add user to server set: %v\n", err)
	}
	log.Printf("Redis: User %s registered on server %s\n", clientID, serverID)
}
func (m *Manager) removeUserPresence(clientID string, serverID string) {
	if m.redisClient == nil {
		return
	}
	// Remove user's server mapping
	err := m.redisClient.Del(ctx, "user:"+clientID+":server").Err()
	if err != nil {
		log.Printf("Redis: Failed to delete user server mapping: %v\n", err)
	}
	// Remove user from this server's set
	err = m.redisClient.SRem(ctx, "server:"+serverID+":users", clientID).Err()
	if err != nil {
		log.Printf("Redis: Failed to remove user from server set: %v\n", err)
	}
	log.Printf("Redis: User %s unregistered from server %s\n", clientID, serverID)
}

// Optional: Redis Pub/Sub for very specific, low-latency, non-persistent messages.
// For most inter-server WebSocket communication, RabbitMQ is generally preferred due to features.
func (m *Manager) subscribeToRedisChannel(channelName string) {
	if m.redisClient == nil {
		log.Println("Cannot subscribe to Redis: client not initialized.")
		return
	}
	pubsub := m.redisClient.Subscribe(ctx, channelName)
	// Wait for confirmation that subscription is created before publishing anything.
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Printf("Error setting up Redis subscription to %s: %v\n", channelName, err)
		return
	}
	ch := pubsub.Channel()
	log.Printf("Subscribed to Redis channel: %s\n", channelName)

	for {
		select {
		case <-m.shutdownChan:
			log.Printf("Redis Pub/Sub for %s shutting down.\n", channelName)
			pubsub.Close()
			return
		case msg, ok := <-ch:
			if !ok {
				log.Printf("Redis Pub/Sub channel %s closed.\n", channelName)
				return
			}
			log.Printf("Received from Redis Pub/Sub channel '%s': %s\n", msg.Channel, msg.Payload)
			// Example: Forward to local broadcast, similar to RabbitMQ messages
			// m.broadcast <- []byte(msg.Payload)
		}
	}
}

// --- End Redis Helper Functions ---

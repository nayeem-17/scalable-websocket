package main

import (
	"fmt"
	"time"
)

type Session struct {
	Nickname    string `redis:"nickname"`
	Status      string `redis:"status"`
	CurrentRoom string `redis:"current_room"`
}

// saveSession stores the entire Session struct into a Redis Hash.
func (m *Manager) saveSession(clientID string, session *Session) error {
	if m.redisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}

	key := "session:" + clientID

	// Create a map from the struct fields to be saved.
	// This manually does the conversion that the library was failing to do automatically.
	sessionData := map[string]interface{}{
		"nickname":     session.Nickname,
		"status":       session.Status,
		"current_room": session.CurrentRoom,
	}

	// Use HSet with the map. This is the idiomatic way to set multiple fields in a hash.
	if err := m.redisClient.HSet(ctx, key, sessionData).Err(); err != nil {
		return err
	}

	// After saving, always refresh the expiration time to prevent old sessions from piling up.
	return m.redisClient.Expire(ctx, key, 24*time.Hour).Err()
}

func (m *Manager) getSession(clientID string) (*Session, error) {
	if m.redisClient == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	key := "session:" + clientID
	res, err := m.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	// If the hash is empty, it means no session exists for this user.
	if len(res) == 0 {
		return nil, nil // Return nil, nil to indicate "not found" without an error.
	}
	session := &Session{}
	if val, ok := res["nickname"]; ok {
		session.Nickname = val
	}
	if val, ok := res["status"]; ok {
		session.Status = val
	}
	if val, ok := res["current_room"]; ok {
		session.CurrentRoom = val
	}
	// After getting a session, it's good practice to refresh its TTL
	// to keep it alive since the user is active.
	m.redisClient.Expire(ctx, key, 24*time.Hour)

	return session, nil

}

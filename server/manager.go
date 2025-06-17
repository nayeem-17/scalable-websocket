package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8" // Redis client
	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serveWsWithBackplane(m *Manager, w http.ResponseWriter, r *http.Request) {
	log.Println("New connection attempt from:", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection for %s: %v\n", r.RemoteAddr, err)
		return
	}

	client := &Client{conn: conn, id: conn.RemoteAddr().String() + "-" + serverID}
	m.register <- client

	go client.writeLoop(m)
	go client.readLoop(m)
}

type Client struct {
	conn *websocket.Conn
	id   string
}

type Manager struct {
	clients      map[*Client]bool
	clientsMu    sync.Mutex
	broadcast    chan []byte
	register     chan *Client
	unregister   chan *Client
	shutdownChan chan struct{}
	amqpConn     *amqp.Connection
	amqpChannel  *amqp.Channel
	redisClient  *redis.Client
}

type Message struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Sender  string `json:"sender,omitempty"`
}

var manager = Manager{
	clients:      make(map[*Client]bool),
	broadcast:    make(chan []byte, broadcastMessageSize),
	register:     make(chan *Client),
	unregister:   make(chan *Client),
	shutdownChan: make(chan struct{}),
}

func (m *Manager) unregisterClient(client *Client) {
	m.clientsMu.Lock()
	if _, ok := m.clients[client]; ok {
		delete(m.clients, client)
		err := client.conn.Close()
		if err != nil {
			log.Println("Error occurred while closing the client websocket connection.")
			return
		}
	}
	m.clientsMu.Unlock()
}

// Manager's main loop
func (m *Manager) run() {
	log.Println("Client Manager started")
	defer func() {
		var errors []error

		if m.amqpChannel != nil {
			if err := m.amqpChannel.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close AMQP channel: %w", err))
			}
		}

		if m.amqpConn != nil {
			if err := m.amqpConn.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close AMQP connection: %w", err))
			}
		}

		if m.redisClient != nil {
			if err := m.redisClient.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close Redis client: %w", err))
			}
		}

		if len(errors) > 0 {
			for _, err := range errors {
				log.Printf("Error during cleanup: %v", err)
			}
		} else {
			log.Println("Client Manager stopped and resources released successfully")
		}
	}()

	if m.amqpChannel != nil {
		go m.consumeFromRabbitMQ()
	} else {
		log.Println("Skipping RabbitMQ consumer: channel not initialized.")
	}

	// Example: Optional Redis Pub/Sub listener
	// if m.redisClient != nil {
	//    go m.subscribeToRedisChannel("some_realtime_updates")
	// }

	for {
		select {
		case <-m.shutdownChan:
			m.clientsMu.Lock()
			log.Println("Manager shutting down. Closing client connections...")
			for client := range m.clients {
				client.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server is shutting down"))
				client.conn.Close()
				delete(m.clients, client)
				m.removeUserPresence(client.id, serverID) // Remove from Redis on shutdown
			}
			m.clientsMu.Unlock()
			return

		case client := <-m.register:
			m.clientsMu.Lock()
			m.clients[client] = true
			m.clientsMu.Unlock()
			m.setUserPresence(client.id, serverID) // Set presence in Redis
			log.Printf("Client %s registered on %s. Total local clients: %d\n", client.id, serverID, len(m.clients))

			joinMsg := Message{Type: "user_join", Sender: serverID, Payload: fmt.Sprintf("User %s joined server %s", client.id, serverID)}
			localJoinMsg := Message{Type: "local user_join", Sender: serverID, Payload: fmt.Sprintf("User %s joined server %s", client.id, serverID)}
			joinMsgJSON, _ := json.Marshal(joinMsg)
			localJoinMsgJSON, _ := json.Marshal(localJoinMsg)

			// Publish join message to RabbitMQ for other servers
			if err := m.publishToRabbitMQ(broadcastExchange, "", joinMsgJSON); err != nil {
				log.Printf("Failed to publish join message to RabbitMQ: %v\n", err)
			}
			// Also broadcast locally (RabbitMQ will also send it back if no-local is false, but direct is faster for local)
			select {
			case m.broadcast <- localJoinMsgJSON:
			default:
				log.Println("Local broadcast channel full. Dropping user_join message.")
			}

		case client := <-m.unregister:
			log.Println("Broadcasting message............................")
			m.clientsMu.Lock()
			if _, ok := m.clients[client]; ok {
				delete(m.clients, client)
				m.clientsMu.Unlock()                      // Unlock before Redis call and RabbitMQ publish
				m.removeUserPresence(client.id, serverID) // Remove from Redis
				log.Printf("Client %s unregistered from %s. Total local clients: %d\n", client.id, serverID, len(m.clients))

				leaveMsg := Message{Type: "user_leave", Sender: serverID, Payload: fmt.Sprintf("User %s left server %s", client.id, serverID)}
				leaveMsgJSON, _ := json.Marshal(leaveMsg)
				if err := m.publishToRabbitMQ(broadcastExchange, "", leaveMsgJSON); err != nil {
					log.Printf("Failed to publish leave message to RabbitMQ: %v\n", err)
				}
				// Local broadcast is handled by RabbitMQ consumer now for leave messages too
				// unless you specifically want to bypass for local optimization.
			} else {
				m.clientsMu.Unlock()
			}

		case message := <-m.broadcast: // Messages from RabbitMQ or direct local broadcasts
			m.clientsMu.Lock()
			currentClients := make([]*Client, 0, len(m.clients))
			for client := range m.clients {
				currentClients = append(currentClients, client)
			}
			m.clientsMu.Unlock()

			for _, client := range currentClients {
				client.conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
					log.Printf("Error writing to client %s: %v. Unregistering.\n", client.id, err)
					// Re-trigger unregistration through the channel to ensure consistent handling
					// but be careful of deadlock if this select case is blocked.
					// A direct call to a safe unregister method might be better here.
					go func(cl *Client) { m.unregister <- cl }(client) // Non-blocking send
				}
			}
		}
	}
}

// Client readLoop: Now publishes to RabbitMQ instead of directly to local manager.broadcast
func (c *Client) readLoop(m *Manager) {
	defer func() {
		m.unregister <- c
		c.conn.Close()
		log.Printf("Client %s readLoop ended on %s.\n", c.id, serverID)
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, p, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Client %s read error on %s: %v\n", c.id, serverID, err)
			} else {
				log.Printf("Client %s connection closed on %s: %v\n", c.id, serverID, err)
			}
			break
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if messageType == websocket.TextMessage {
			// log.Printf("Raw message from client %s on %s: %s\n", c.id, serverID, string(p))
			var receivedMsg Message
			if err := json.Unmarshal(p, &receivedMsg); err == nil {
				receivedMsg.Sender = serverID

				finalMsgJSON, err := json.Marshal(receivedMsg)
				if err != nil {
					log.Printf("Error marshalling message for RabbitMQ: %v\n", err)
					continue
				}
				// Publish to RabbitMQ for global broadcast
				if err := m.publishToRabbitMQ(broadcastExchange, "", finalMsgJSON); err != nil {
					log.Printf("Failed to publish client message to RabbitMQ: %v\n", err)
				}
			} else {
				log.Printf("Error unmarshalling message from client %s on %s: %v. Publishing raw.\n", c.id, serverID, err)
				// Decide if raw byte arrays should also be published or just dropped/logged
				if err := m.publishToRabbitMQ(broadcastExchange, "", p); err != nil {
					log.Printf("Failed to publish raw client message to RabbitMQ: %v\n", err)
				}
			}
		} else {
			log.Printf("Received non-text message (type %d) from client %s on %s\n", messageType, c.id, serverID)
		}
	}
}

// Client writeLoop remains largely the same (primarily for pings)
func (c *Client) writeLoop(m *Manager) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		log.Printf("Client %s writeLoop ended on %s.\n", c.id, serverID)
	}()

	for {
		select {
		case <-pingTicker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Client %s ping error on %s: %v\n", c.id, serverID, err)
				return
			}
		case <-m.shutdownChan: // Listen for manager shutdown
			// log.Printf("Client %s writeLoop shutting down due to manager signal on %s.\n", c.id, serverID)
			return
		}
	}
}

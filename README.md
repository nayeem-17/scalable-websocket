# WebSocket Project Todo List

## Phase 1: Core WebSocket Server Development (Go)

### Task 1.1: Basic WebSocket Handler
- [x] Implement a simple Go WebSocket server that can accept connections
- [x] Add functionality to receive messages from connected clients
- [x] Add functionality to send messages back to connected clients
- [x] Use gorilla/websocket library for WebSocket implementation

### Task 1.2: Message Structures & Broadcasting Logic (Single Server)
- [x] Define message formats (JSON structure)
- [x] Implement logic for broadcasting messages to all connected clients
- [x] Test broadcasting functionality on a single server instance

### Task 1.3: Graceful Shutdown & Connection Management
- [x] Handle client disconnections properly
- [x] Implement graceful server shutdown mechanism
- [x] Add proper connection cleanup logic

## Phase 2: Backplane Integration (RabbitMQ & Redis)

### Task 2.1: RabbitMQ Integration for Inter-Server Messaging
- [x] Set up RabbitMQ message publishing for cross-server broadcasts
- [x] Configure fanout exchange for global broadcasts
- [x] Set up topic exchanges for targeted messages
- [x] Implement RabbitMQ message consuming for each WebSocket server instance
- [x] Add logic to forward RabbitMQ messages to locally connected clients

### Task 2.2: Redis Integration
- [ ] Implement Redis Pub/Sub for low-latency, non-persistent messages (optional)
- [ ] Set up presence tracking using Redis Sets or Hashes
- [ ] Track which users are connected to which server instance
- [ ] Implement session/state caching for stateless server design
- [ ] Add Redis for temporary session data sharing

## Phase 3: Load Balancing with Nginx

### Task 3.1: Nginx Setup & Basic HTTP Load Balancing
- [x] Install and configure Nginx
- [x] Set up basic HTTP load balancing across multiple Go application instances
- [x] Test basic load balancing with HTTP endpoints

### Task 3.2: Nginx Configuration for WebSocket Proxying
- [x] Configure Nginx to properly proxy WebSocket connections
- [x] Set up proper Upgrade and Connection headers
- [x] Implement sticky sessions using ip_hash method
- [x] Test WebSocket proxying functionality

## Phase 4: Scalability & OS Tuning

### Task 4.1: Horizontal Scaling Strategy
- [ ] Plan deployment strategy for multiple Go WebSocket server instances
- [ ] Choose deployment method (Docker, Kubernetes, or systemd)
- [ ] Document scaling procedures

### Task 4.2: OS File Descriptor Limits
- [ ] Research file descriptor limits for target operating systems
- [ ] Increase maximum open file descriptors to support 10,000+ connections
- [ ] Configure ulimit settings (temporary and permanent)
- [ ] Update /etc/security/limits.conf for permanent changes

## Phase 5: Testing with k6

### Task 5.1: Develop k6 Test Scripts
- [x] Write k6 scripts to simulate WebSocket connections
- [x] Add connection establishment testing
- [x] Add message sending functionality to tests
- [x] Add message receiving and validation logic
- [x] Implement concurrent user simulation
- [x] Add disconnection and reconnection testing scenarios

### Task 5.2: Incremental Load Testing
- [x] Test with 100 concurrent users
- [x] Test with 1,000 concurrent users
- [x] Test with 5,000 concurrent users
- [x] Test with 10,000 concurrent users
- [ ] Document performance results at each level

### Task 5.3: Performance Analysis & Bottleneck Identification
- [ ] Monitor server CPU usage during tests
- [ ] Monitor memory usage during tests
- [ ] Monitor network I/O performance
- [ ] Monitor Nginx performance metrics
- [ ] Monitor RabbitMQ performance metrics
- [ ] Monitor Redis performance metrics
- [ ] Identify and document system bottlenecks
- [ ] Implement solutions for identified bottlenecks

## Phase 6: Statelessness & Data Persistence

### Task 6.1: Ensure Server Statelessness
- [ ] Review WebSocket server code for memory-stored session state
- [ ] Move all shared state to Redis or primary database
- [ ] Ensure no critical session state is stored in single instance memory
- [ ] Test server instance failover scenarios

### Task 6.2: Database Integration
- [ ] Design database schemas for persistent data
- [ ] Create user accounts table/collection
- [ ] Create message history storage (if needed)
- [ ] Implement efficient database connection management in Go applications
- [ ] Test database integration with WebSocket servers

## Phase 7: Deployment & Monitoring

### Task 7.1: Production Deployment Strategy
- [ ] Choose deployment environment (cloud provider or on-premise)
- [ ] Set up CI/CD pipelines
- [ ] Create deployment scripts and documentation
- [ ] Test deployment process in staging environment

### Task 7.2: Comprehensive Monitoring & Logging
- [ ] Implement robust logging in Go applications
- [ ] Set up monitoring tools (Prometheus, Grafana, Datadog, or cloud tools)
- [ ] Monitor active WebSocket connections
- [ ] Monitor message rates (incoming and outgoing)
- [ ] Monitor error rates across all components
- [ ] Monitor resource utilization (CPU, memory, network)
- [ ] Monitor Nginx performance metrics
- [ ] Monitor RabbitMQ performance metrics
- [ ] Monitor Redis performance metrics
- [ ] Monitor latency metrics

### Task 7.3: Alerting
- [ ] Set up alerts for high error rates
- [ ] Set up alerts for server downtime
- [ ] Set up alerts for resource exhaustion
- [ ] Set up alerts for critical system failures
- [ ] Test alerting system functionality
- [ ] Document incident response procedures

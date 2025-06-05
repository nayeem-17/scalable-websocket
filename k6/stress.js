import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// Custom Metrics
const successfulConnections = new Counter('successful_connections'); // Counts connections that reached 'open'
const liveConnectionsHandshake = new Counter('live_connections_handshake'); // Counts connections that completed the initial message exchange

export const options = {
  // Define stages to ramp up VUs (connections)
  // Adjust these target numbers and durations based on your server's expected capacity
  // and how long you want to observe it at each level.
  stages: [
    { duration: '2s', target: 50 },    // Ramp up to 50 connections over 2 seconds
    { duration: '1m', target: 50 },    // Stay at 50 connections for 1 minute
    { duration: '2s', target: 200 },   // Ramp up to 200 connections over 2 seconds
    { duration: '1m', target: 200 },   // Stay at 200 connections for 1 minute
    { duration: '2s', target: 500 },   // Ramp up to 500 connections over 2 seconds
    { duration: '1m', target: 500 },   // Stay at 500 connections for 1 minute
    { duration: '2s', target: 1000 },  // Ramp up to 1000 connections over 2 seconds (adjust higher if needed)
    { duration: '1m', target: 1000 },  // Stay at 1000 connections for 1 minute
    // Add more stages if you expect to handle more connections
    { duration: '25s', target: 0 },     // Ramp down gracefully
  ],
  thresholds: {
    'ws_connecting': ['p(95)<2500'],     // 95% of connections should establish in < 2.5s (allow more time under stress)
    'ws_sessions': ['rate>0.98'],        // Success rate of WebSocket session establishment (HTTP 101) should be > 98%
    'successful_connections': ['rate>0.98'], // Rate of VUs reaching 'open' state
    'live_connections_handshake': ['rate>0.95'], // Rate of VUs completing initial handshake (if applicable)
    'checks': ['rate>0.95'],             // Overall checks pass rate (for message validation)
    'vus': ['value>=0'],                   // Ensure VUs are actually running
    // 'k6_errors': ['count<100'],       // Optional: Limit k6 internal errors
  },
};

export default function () {
  const url = 'ws://localhost:80/ws'; // Your WebSocket URL
  const params = { tags: { my_custom_tag: 'k6-connection-stress' } };

  const res = ws.connect(url, params, function (socket) {
    socket.on('open', () => {
      successfulConnections.add(1);
      // console.log(`VU ${__VU}: Connected! Iteration: ${__ITER}`); // Reduced logging for stress tests

      const messageToSend = JSON.stringify({
        event: 'test_message_connect', // Potentially a different event for connection test
        userId: __VU,
        timestamp: new Date().toISOString(),
        payload: `Connection check from k6 VU ${__VU}`,
      });
      socket.send(messageToSend);
      // console.log(`VU ${__VU}: Sent: ${messageToSend}`);
    });

    socket.on('message', (data) => {
      // console.log(`VU ${__VU}: Received: ${data}`); // Reduce logging during stress tests
      let allChecksPassedForThisMessage = false;
      let serverId = 'unknown_server';

      try {
        const receivedMsg = JSON.parse(data);
        if (receivedMsg && receivedMsg.sender) {
          // Assuming extractServerId function is defined elsewhere or simplified here if not needed for this specific test
          // For simplicity here, let's assume server ID extraction is not the primary focus of this connection test
          // serverId = extractServerId(receivedMsg.sender);
        }
        // responsesPerServer.add(1, { server_id: serverId }); // If you still want to count per server

        const messageChecks = {
          'initial server response is parsable JSON': (msg) => msg !== null,
          // Add a very basic check for the first response to ensure viability
          'initial response has a "type" property': (msg) => msg.hasOwnProperty('type'),
          'initial response has a "payload" property': (msg) => msg.hasOwnProperty('payload'),
        };
        allChecksPassedForThisMessage = check(receivedMsg, messageChecks);

      } catch (e) {
        // console.error(`VU ${__VU}: Failed to parse initial message: ${data} - Error: ${e}`);
        check(null, { 'initial message parsing': () => false }, { errorMessage: e.toString() });
        allChecksPassedForThisMessage = false;
      }

      if (allChecksPassedForThisMessage) {
        liveConnectionsHandshake.add(1); // Connection is established and received a valid first response
      }

      // DO NOT CLOSE THE SOCKET HERE: socket.close();
      // We want to keep the connection open for the stress test.
      // The socket will remain open until the VU finishes its lifecycle for the stage,
      // an error occurs, or the server closes it.

      // If your server requires periodic pings to keep connections alive, implement them here:
      // socket.setInterval(() => {
      //   socket.ping(); // or socket.send(JSON.stringify({type: 'ping'}));
      //   console.log(`VU ${__VU}: Sent ping`);
      // }, 25000); // e.g., every 25 seconds
    });

    socket.on('close', () => {
      // console.log(`VU ${__VU}: Disconnected. Code: ${event.code}, Reason: ${event.reason}`);
    });

    socket.on('error', (e) => {
      // console.error(`VU ${__VU}: WebSocket error: ${e.error()}`);
      check(null, { 'websocket error during session': () => false }, { vu: __VU, error: e.error() });
    });

    // REMOVE or significantly lengthen any socket.setTimeout that closes the connection.
    // Example: The original script had a 1-second timeout to close if no message.
    // For holding connections, this is not desired.
    // If you need a timeout for the *initial message only*, handle it carefully.
    // For this example, we'll rely on the initial send/receive happening quickly.
  });

  // Check if the initial HTTP upgrade request itself was successful
  check(res, { 'WebSocket connection upgrade successful (status 101)': (r) => r && r.status === 101 });

  // This sleep is important. If a VU's WebSocket connection drops (e.g., server closes it),
  // the VU will loop, wait for this sleep, and then attempt to reconnect.
  // This simulates VUs trying to maintain their connection count.
  // For very long-lived connections held by VUs for entire stages, this sleep might not
  // be hit often if connections are stable.
  sleep(5); // Sleep for 5 seconds before a VU might loop (e.g., if its connection dropped)
}

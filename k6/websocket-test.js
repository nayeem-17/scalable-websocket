import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics'; // Import Counter

// Existing Custom Metrics
const successfulConnections = new Counter('successful_connections');
const successfulInteractions = new Counter('successful_interactions');

// New Custom Metric to count responses per server
const responsesPerServer = new Counter('responses_per_server');

export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    'ws_connecting': ['p(95)<1500'],
    'ws_msgs_sent': ['count>=300'],
    'ws_msgs_received': ['count>=300'],
    'checks': ['rate>0.98'],
    'successful_connections': ['count>=295'], // Adjusted as per previous discussion
    'successful_interactions': ['count>=295'],// Adjusted as per previous discussion
    // You could add thresholds for responses_per_server if you have expectations,
    // e.g., 'responses_per_server{server_id:server-alpha}:count > 100'
  },
};

// Function to extract server ID from the sender string
// Example: "172.21.0.5:40322-server-1749067616873732057" -> "server-1749067616873732057"
function extractServerId(senderString) {
  if (!senderString || typeof senderString !== 'string') {
    return 'unknown_server';
  }
  const parts = senderString.split('-');
  if (parts.length > 1) {
    // Assuming the server ID starts with "server-" and is the last part or composed of last few parts
    // This logic might need adjustment based on the exact consistent format of your server ID
    let serverIdParts = [];
    for (let i = 0; i < parts.length; i++) {
      if (parts[i].startsWith('server')) {
        serverIdParts = parts.slice(i);
        break;
      }
    }
    if (serverIdParts.length > 0) {
      return serverIdParts.join('-');
    }
  }
  // Fallback if a clear "server-..." pattern isn't found after the first dash
  const lastDashIndex = senderString.lastIndexOf('-');
  if (lastDashIndex > -1 && lastDashIndex < senderString.length -1) {
    const potentialId = senderString.substring(lastDashIndex + 1);
    if (potentialId.startsWith('server')) return potentialId; // Or just return it as is
  }

  return 'parse_error_or_unknown_server'; // Fallback if parsing fails
}


export default function () {
  const url = 'ws://localhost:80/ws'; // Your WebSocket URL
  const params = { tags: { my_custom_tag: 'k6-websocket-server-id-test' } };

  const res = ws.connect(url, params, function (socket) {
    socket.on('open', () => {
      successfulConnections.add(1);
      console.log(`VU ${__VU} (${__ITER}): Connected!`);

      const messageToSend = JSON.stringify({
        event: 'test_message',
        userId: __VU,
        type:1,
        timestamp: new Date().toISOString(),
        payload: `Hello from k6 VU ${__VU}`,
      });
      socket.send(messageToSend);
      console.log(`VU ${__VU} (${__ITER}): Sent: ${messageToSend}`);
    });

    socket.on('message', (data) => {
      console.log(`VU ${__VU} (${__ITER}): Received: ${data}`);
      let allChecksPassedForThisMessage = false;
      let serverId = 'unknown_server'; // Default server ID

      try {
        const receivedMsg = JSON.parse(data);

        // Extract server ID if available (assuming it's in receivedMsg.sender)
        if (receivedMsg && receivedMsg.sender) {
          serverId = extractServerId(receivedMsg.sender);
        }
        // Increment the counter for this server ID
        // This will count every message received, tagged by server_id
        responsesPerServer.add(1, { server_id: serverId });

        const messageChecks = {
          'server responded with a parsable JSON': (msg) => msg !== null,
          'message has a "type" property': (msg) => msg.hasOwnProperty('type'),
          'message has a "payload" property': (msg) => msg.hasOwnProperty('payload'),
          // Remove the mandatory sender check since it's not always present
          // 'message has a "sender" property': (msg) => msg.hasOwnProperty('sender'),
        };

        if (receivedMsg.type === "TEXT_MESSAGE" && receivedMsg.payload && receivedMsg.payload.startsWith("Hello from k6 VU")) {
          messageChecks['is a broadcast of a test message'] = (msg) => true;
        } else if (receivedMsg.type === "USER_JOIN") {
          messageChecks['is a user_join message'] = (msg) => msg.payload.includes("joined server");
        }
        
        // Only add the server ID check if we actually received a sender property
        if (receivedMsg.sender) {
          messageChecks[`response from server ${serverId}`] = () => serverId !== 'unknown_server' && serverId !== 'parse_error_or_unknown_server';
        }

        allChecksPassedForThisMessage = check(receivedMsg, messageChecks);

      } catch (e) {
        console.error(`VU ${__VU} (${__ITER}): Failed to parse received message: ${data} - Error: ${e}`);
        check(null, { 'message parsing': () => false }, { errorMessage: e.toString() });
        allChecksPassedForThisMessage = false;
        responsesPerServer.add(1, { server_id: 'parse_error_or_unidentified' }); // Count parse errors separately
      }

      if (allChecksPassedForThisMessage) {
        successfulInteractions.add(1);
      }
      socket.close();
    });

    socket.on('close', () => {
      console.log(`VU ${__VU} (${__ITER}): Disconnected.`);
    });

    socket.on('error', (e) => {
      if (e.error() !== 'websocket: close sent') {
        console.error(`VU ${__VU} (${__ITER}): WebSocket error: ${e.error()}`);
      }
      check(null, { 'no unexpected websocket error': () => false }, { error: e.error() });
    });

    socket.setTimeout(() => {
      console.log(`VU ${__VU} (${__ITER}): Timed out waiting for message. Closing socket.`);
      socket.close();
    }, 1000); // 1 second timeout
  });

  check(res, { 'WebSocket connection upgrade successful (status 101)': (r) => r && r.status === 101 });
  sleep(1);
}

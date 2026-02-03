// Manual WebSocket connection for both device state and event stream
const wsUrl = `ws://${window.location.host}/ws/events`;
let ws;
let reconnectAttempts = 0;
const maxReconnectDelay = 30000;

function connectWebSocket() {
  ws = new WebSocket(wsUrl);
  const statusIndicator = document.getElementById("ws-status");
  const eventsContainer = document.getElementById("events");
  const deviceStateContainer = document.getElementById("device-state");

  ws.onopen = () => {
    console.log("WebSocket connected");
    statusIndicator.className = "connection-status connected";
    reconnectAttempts = 0;

    if (
      eventsContainer.children.length === 0 ||
      eventsContainer.querySelector('p[style*="italic"]')
    ) {
      eventsContainer.innerHTML =
        '<p style="color: #999; font-style: italic;">Connected. Waiting for events...</p>';
    }
  };

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);

      // Update device state with Mustache template
      if (data.type === "event" && deviceStateContainer) {
        updateDeviceState(data);
      }

      // Add event to stream
      const eventElement = document.createElement("div");
      eventElement.className = "event";

      const now = new Date().toLocaleTimeString();
      eventElement.innerHTML = `
        <div class="event-time">${now}</div>
        <div class="event-data">${JSON.stringify(data, null, 2)}</div>
      `;

      if (eventsContainer.querySelector('p[style*="italic"]')) {
        eventsContainer.innerHTML = "";
      }

      eventsContainer.insertBefore(eventElement, eventsContainer.firstChild);

      if (eventsContainer.children.length > 50) {
        eventsContainer.removeChild(eventsContainer.lastChild);
      }
    } catch (e) {
      console.error("Failed to parse event:", e);
    }
  };

  ws.onerror = (error) => {
    console.error("WebSocket error:", error);
    statusIndicator.className = "connection-status disconnected";
  };

  ws.onclose = () => {
    console.log("WebSocket disconnected");
    statusIndicator.className = "connection-status disconnected";

    const delay = Math.min(
      1000 * Math.pow(2, reconnectAttempts),
      maxReconnectDelay,
    );
    reconnectAttempts++;

    console.log(`Reconnecting in ${delay}ms...`);
    setTimeout(connectWebSocket, delay);
  };
}

function updateDeviceState(data) {
  const template = document.getElementById("device-state-template");
  const deviceStateContainer = document.getElementById("device-state");

  if (!template || !deviceStateContainer) {
    console.error("Template or container not found");
    return;
  }

  const state = data.state.toLowerCase();
  const templateData = {
    state: data.state,
    state_class: state,
    current_temperature: data.current_temperature,
    remaining_time: data.remaining_time,
    programmed_temperature: data.programmed_temperature,
    programmed_duration: data.programmed_duration,
  };

  const rendered = Mustache.render(template.innerHTML, templateData);
  deviceStateContainer.innerHTML = rendered;
}

// Start WebSocket connection when DOM is ready
connectWebSocket();

// // Handle htmx configRequest to convert form data to proper JSON types
// document.body.addEventListener("htmx:configRequest", (event) => {
//   if (event.detail.path === "/start") {
//     const params = event.detail.parameters;
//     // Convert string values to numbers
//     event.detail.parameters = {
//       temperature: parseInt(params.temperature, 10),
//       duration: parseInt(params.duration, 10),
//     };
//     // Set proper content type
//     event.detail.headers["Content-Type"] = "application/json";
//   }
// });

// Handle htmx form submissions
document.body.addEventListener("htmx:afterRequest", (event) => {
  const target = event.detail.target;
  if (event.detail.successful) {
    target.innerHTML =
      '<div class="status success">Command sent successfully</div>';
    setTimeout(() => {
      target.innerHTML = "";
    }, 3000);
  } else {
    target.innerHTML = '<div class="status error">Failed to send command</div>';
    setTimeout(() => {
      target.innerHTML = "";
    }, 5000);
  }
});

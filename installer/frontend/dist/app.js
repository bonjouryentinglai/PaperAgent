const host = document.querySelector("#host");
const password = document.querySelector("#password");
const message = document.querySelector("#message");
const grid = document.querySelector("#status-grid");
const readyBadge = document.querySelector("#ready-badge");
const discoverButton = document.querySelector("#discover");
const inspectButton = document.querySelector("#inspect");

function api(name, ...args) {
  const fn = window.go?.main?.App?.[name];
  if (!fn) return Promise.reject(new Error("Wails bridge is not ready"));
  return fn(...args);
}

function yes(value, positive = "Ready", negative = "Missing") {
  return value ? positive : negative;
}

function render(status) {
  const values = [
    ["Model", status.model || "Unknown"],
    ["OS", status.osVersion || "Unknown"],
    ["XOVI", yes(status.xoviInstalled, "Installed")],
    ["AppLoad", yes(status.appLoadInstalled, "Installed")],
    ["ChatGPT login", yes(status.chatGPTLoggedIn, "Signed in", "Not signed in")],
    ["Paper Agent", yes(status.paperAgent, status.serviceActive ? "Running" : "Installed", "Not installed")],
  ];
  const cards = values.map(([label, value]) => {
    const card = document.createElement("div");
    const name = document.createElement("span");
    const result = document.createElement("strong");
    name.textContent = label;
    result.textContent = value;
    card.append(name, result);
    return card;
  });
  grid.replaceChildren(...cards);
  const ready = status.supportedModel && status.developerMode;
  readyBadge.textContent = ready ? "Compatible Move" : "Needs attention";
  readyBadge.className = ready ? "badge" : "badge muted";
}

async function busy(button, action) {
  button.disabled = true;
  try {
    await action();
  } catch (error) {
    message.textContent = error?.message || String(error);
  } finally {
    button.disabled = false;
  }
}

discoverButton.addEventListener("click", () => busy(discoverButton, async () => {
  message.textContent = "Looking on USB and local networks…";
  const devices = await api("Discover");
  if (!devices || devices.length === 0) {
    message.textContent = "No awake developer-mode reMarkable was found.";
    return;
  }
  host.value = devices[0].address;
  message.textContent = devices.length === 1
    ? `Found ${devices[0].usb ? "USB " : ""}device at ${devices[0].address}.`
    : `Found ${devices.length} candidates. Using ${devices[0].address}; you can change it above.`;
}));

inspectButton.addEventListener("click", () => busy(inspectButton, async () => {
  message.textContent = "Checking model, OS, storage, dependencies, and login…";
  const status = await api("Inspect", host.value, password.value);
  password.value = "";
  render(status);
  message.textContent = status.paperAgent
    ? "Paper Agent installation detected."
    : "Device check complete. No changes were made.";
}));

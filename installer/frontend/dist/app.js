const host = document.querySelector("#host");
const password = document.querySelector("#password");
const message = document.querySelector("#message");
const grid = document.querySelector("#status-grid");
const readyBadge = document.querySelector("#ready-badge");
const connectButton = document.querySelector("#connect");
const forgetPasswordButton = document.querySelector("#forget-password");
const loginButton = document.querySelector("#login");
const cancelLoginButton = document.querySelector("#cancel-login");
const loginGuide = document.querySelector("#login-guide");
const loginURL = document.querySelector("#login-url");
const loginCode = document.querySelector("#login-code");
const loginMessage = document.querySelector("#login-message");
const installButton = document.querySelector("#install");
const updateButton = document.querySelector("#update");
const repairButton = document.querySelector("#repair");
const changeConfirm = document.querySelector("#change-confirm");
const checkReleaseButton = document.querySelector("#check-release");
const releaseVersion = document.querySelector("#release-version");
const operationStage = document.querySelector("#operation-stage");
const operationPercent = document.querySelector("#operation-percent");
const operationProgress = document.querySelector("#operation-progress");
const operationMessage = document.querySelector("#operation-message");
const uninstallButton = document.querySelector("#uninstall");
const uninstallConfirm = document.querySelector("#uninstall-confirm");
const uninstallMessage = document.querySelector("#uninstall-message");
let lastStatus = null;
let operationTimer = null;
let loginTimer = null;

function api(name, ...args) {
  const fn = window.go?.main?.App?.[name];
  if (!fn) return Promise.reject(new Error("Wails bridge is not ready"));
  return fn(...args);
}

function yes(value, positive = "Ready", negative = "Missing") {
  return value ? positive : negative;
}

function render(status) {
  lastStatus = status;
  const values = [
    ["Model", status.model || "Unknown"],
    ["OS", status.osVersion || "Unknown"],
    ["XOVI", yes(status.xoviInstalled, "Installed")],
    ["AppLoad", yes(status.appLoadInstalled, "Installed")],
    ["Node + Pi", status.nodeInstalled && status.piInstalled ? "Installed" : "Missing"],
    ["ChatGPT login", yes(status.chatGPTLoggedIn, "Signed in", "Not signed in")],
    ["Paper Agent", yes(
      status.paperAgent,
      `${status.serviceActive ? "Running" : "Installed"}${status.installedVersion ? ` · ${status.installedVersion}` : ""}`,
      "Not installed",
    )],
    ["Settings app", yes(status.settingsApp, "Installed")],
    ["Free storage", `${Math.max(0, status.freeSpaceKB / 1024).toFixed(0)} MiB`],
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
  if (status.chatGPTLoggedIn) {
    loginGuide.classList.add("hidden");
    loginMessage.textContent = "ChatGPT is signed in on this Move.";
  } else if (status.nodeInstalled && status.piInstalled) {
    loginMessage.textContent = "Start ChatGPT sign-in when you are ready.";
  } else {
    loginMessage.textContent = "Run Install once to add the runtime before signing in.";
  }
  updateActions();
}

function updateActions() {
  const ready = lastStatus?.supportedModel && lastStatus?.developerMode;
  const confirmed = changeConfirm.checked;
  const operationRunning = operationProgress.dataset.running === "true";
  const loginRunning = loginButton.dataset.running === "true";
  installButton.disabled = !(ready && !lastStatus?.paperAgent && confirmed && !operationRunning && !loginRunning);
  updateButton.disabled = !(ready && lastStatus?.paperAgent && confirmed && !operationRunning && !loginRunning);
  repairButton.disabled = !(ready && lastStatus?.paperAgent && confirmed && !operationRunning && !loginRunning);
  loginButton.disabled = !(
    ready &&
    lastStatus?.nodeInstalled &&
    lastStatus?.piInstalled &&
    !lastStatus?.chatGPTLoggedIn &&
    !loginButton.dataset.running
  );
  uninstallButton.disabled = !(
    ready &&
    lastStatus?.paperAgent &&
    uninstallConfirm.checked &&
    !operationRunning &&
    !loginRunning
  );
  forgetPasswordButton.disabled = password.value.length === 0;
}

async function busy(button, action, errorTarget = message) {
  button.disabled = true;
  try {
    await action();
  } catch (error) {
    errorTarget.textContent = error?.message || String(error);
  } finally {
    button.disabled = false;
    updateActions();
  }
}

connectButton.addEventListener("click", () => busy(connectButton, async () => {
  message.textContent = "Finding and checking your Move…";
  const devices = await api("Discover");
  if (devices && devices.length > 0) {
    const selected = devices.find((device) => device.usb) || devices[0];
    host.value = selected.address;
  } else if (!host.value.trim()) {
    message.textContent = "No awake developer-mode reMarkable was found. You can enter its address manually and try again.";
    return;
  }
  const status = await api("Inspect", host.value, password.value);
  render(status);
  message.textContent = status.paperAgent
    ? "Paper Agent installation detected. You can update, repair, or uninstall it below."
    : "Device check complete. You can install Paper Agent below; no changes have been made yet.";
}));

password.addEventListener("input", updateActions);
forgetPasswordButton.addEventListener("click", () => {
  password.value = "";
  password.focus();
  message.textContent = "Developer password forgotten for this installer session.";
  updateActions();
});
uninstallConfirm.addEventListener("change", updateActions);
changeConfirm.addEventListener("change", updateActions);

checkReleaseButton.addEventListener("click", () => busy(checkReleaseButton, async () => {
  releaseVersion.textContent = "Checking…";
  const release = await api("CheckRelease");
  releaseVersion.textContent = `${release.version} · ${(release.bundleBytes / 1024 / 1024).toFixed(1)} MiB`;
}));

function renderOperation(state) {
  operationProgress.dataset.running = state.running ? "true" : "false";
  operationProgress.value = state.percent || 0;
  operationStage.textContent = state.stage || (state.done ? "Complete" : "Ready");
  operationPercent.textContent = `${state.percent || 0}%`;
  operationMessage.textContent = state.error || state.message || "Working…";
  if (state.hasStatus) render(state.status);
  if (state.needsLogin) {
    loginMessage.textContent = "Runtime ready. Start ChatGPT sign-in, then continue installation.";
  }
  updateActions();
}

async function pollOperation() {
  try {
    const state = await api("GetOperationState");
    renderOperation(state);
    if (!state.running && operationTimer) {
      clearInterval(operationTimer);
      operationTimer = null;
    }
  } catch (error) {
    operationMessage.textContent = error?.message || String(error);
  }
}

async function startOperation(name, button) {
  await api(name, host.value, password.value, changeConfirm.checked);
  renderOperation({
    running: true,
    stage: "starting",
    percent: 1,
    message: "Starting…",
  });
  if (operationTimer) clearInterval(operationTimer);
  operationTimer = setInterval(pollOperation, 700);
  await pollOperation();
  button.blur();
}

installButton.addEventListener("click", () => busy(
  installButton,
  () => startOperation("StartInstall", installButton),
));
updateButton.addEventListener("click", () => busy(
  updateButton,
  () => startOperation("StartUpdate", updateButton),
));
repairButton.addEventListener("click", () => busy(
  repairButton,
  () => startOperation("StartRepair", repairButton),
));

function renderLogin(state) {
  loginButton.dataset.running = state.running ? "true" : "";
  cancelLoginButton.disabled = !state.running;
  loginMessage.textContent = state.error || state.message;
  if (state.url && state.code) {
    loginGuide.classList.remove("hidden");
    loginURL.href = state.url;
    loginURL.textContent = "Open ChatGPT sign-in";
    loginCode.textContent = state.code;
  }
  if (state.signedIn && lastStatus) {
    lastStatus.chatGPTLoggedIn = true;
    render(lastStatus);
  } else {
    updateActions();
  }
}

async function pollLogin() {
  try {
    const state = await api("GetLoginState");
    renderLogin(state);
    if (!state.running && loginTimer) {
      clearInterval(loginTimer);
      loginTimer = null;
    }
  } catch (error) {
    loginMessage.textContent = error?.message || String(error);
  }
}

loginButton.addEventListener("click", () => busy(loginButton, async () => {
  loginGuide.classList.add("hidden");
  loginMessage.textContent = "Starting device-code login on the Move…";
  await api("StartLogin", host.value, password.value);
  if (loginTimer) clearInterval(loginTimer);
  loginTimer = setInterval(pollLogin, 700);
  await pollLogin();
}));

loginURL.addEventListener("click", async (event) => {
  event.preventDefault();
  try {
    await api("OpenLoginURL", loginURL.href);
  } catch (error) {
    loginMessage.textContent = error?.message || String(error);
  }
});

cancelLoginButton.addEventListener("click", async () => {
  await api("CancelLogin");
  await pollLogin();
});

uninstallButton.addEventListener("click", () => busy(uninstallButton, async () => {
  uninstallMessage.textContent = "Backing up and removing Paper Agent…";
  const result = await api("Uninstall", host.value, password.value, uninstallConfirm.checked);
  uninstallConfirm.checked = false;
  render(result.status);
  uninstallMessage.textContent = result.summary;
}, uninstallMessage));

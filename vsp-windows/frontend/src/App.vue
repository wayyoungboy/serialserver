<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

// State
const version = ref('')
const loggedIn = ref(false)
const username = ref('')
const serverUrl = ref('http://localhost:9000')
const loginUsername = ref('')
const loginPassword = ref('')
const autoConnect = ref(false)
const devices = ref([])
const selectedDeviceKey = ref('')
const status = ref({
  connected: false,
  visible_port: '',
  device_online: false,
  bytes_sent: 0,
  bytes_received: 0,
  connected_since: '',
  error: ''
})
const loading = ref(false)
const error = ref('')
const driverInstalled = ref(true)

// View state
const currentView = ref('login') // 'login', 'devices', 'connected'

// Get version
async function getVersion() {
  version.value = await window.go.main.App.GetVersion()
}

// Check driver installation
async function checkDriver() {
  driverInstalled.value = await window.go.main.App.CheckCom0ComInstalled()
}

// Load saved config
async function loadConfig() {
  const config = await window.go.main.App.LoadConfig()
  if (config) {
    serverUrl.value = config.server_url || 'http://localhost:9000'
    loginUsername.value = config.username || ''
    autoConnect.value = config.auto_connect || false
  }
}

// Login
async function login() {
  loading.value = true
  error.value = ''

  try {
    // First save server config
    await window.go.main.App.SaveConfig(serverUrl.value, autoConnect.value)

    // Then login
    const user = await window.go.main.App.Login(loginUsername.value, loginPassword.value)
    if (user) {
      loggedIn.value = true
      username.value = user.username
      currentView.value = 'devices'
      await fetchDevices()
    }
  } catch (e) {
    error.value = String(e)
  }

  loading.value = false
}

// Logout
async function logout() {
  loading.value = true
  try {
    await window.go.main.App.Logout()
    loggedIn.value = false
    username.value = ''
    devices.value = []
    currentView.value = 'login'
  } catch (e) {
    error.value = String(e)
  }
  loading.value = false
}

// Fetch devices
async function fetchDevices() {
  loading.value = true
  error.value = ''

  try {
    const deviceList = await window.go.main.App.GetDevices()
    devices.value = deviceList || []
    if (devices.value.length > 0) {
      selectedDeviceKey.value = devices.value[0].device_key
    }
  } catch (e) {
    error.value = String(e)
    devices.value = []
  }

  loading.value = false
}

// Connect to device
async function connect() {
  if (!selectedDeviceKey.value) {
    error.value = 'Please select a device'
    return
  }

  loading.value = true
  error.value = ''

  try {
    await window.go.main.App.Connect(selectedDeviceKey.value)
    currentView.value = 'connected'
    await updateStatus()
  } catch (e) {
    error.value = String(e)
  }

  loading.value = false
}

// Disconnect
async function disconnect() {
  loading.value = true
  error.value = ''

  try {
    await window.go.main.App.Disconnect()
    currentView.value = 'devices'
    status.value = {
      connected: false,
      visible_port: '',
      device_online: false,
      bytes_sent: 0,
      bytes_received: 0,
      connected_since: '',
      error: ''
    }
  } catch (e) {
    error.value = String(e)
  }

  loading.value = false
}

// Update status
async function updateStatus() {
  const s = await window.go.main.App.GetStatus()
  status.value = s
}

// Format bytes for display
function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

// Status listener
let statusListener = null

// Setup event listener
function setupEventListeners() {
  statusListener = window.runtime.EventsOn('statusUpdate', (data) => {
    try {
      const parsed = JSON.parse(data)
      status.value = parsed
    } catch (e) {
      console.error('Failed to parse status update:', e)
    }
  })
}

// Cleanup event listener
function cleanupEventListeners() {
  if (statusListener) {
    window.runtime.EventsOff('statusUpdate')
  }
}

// Initialize
onMounted(async () => {
  await getVersion()
  await checkDriver()
  await loadConfig()
  setupEventListeners()

  // Check if already logged in
  const isLoggedIn = await window.go.main.App.IsLoggedIn()
  if (isLoggedIn) {
    loggedIn.value = true
    username.value = await window.go.main.App.GetCurrentUsername()
    currentView.value = 'devices'
    await fetchDevices()

    // Check connection status
    const s = await window.go.main.App.GetStatus()
    status.value = s
    if (s.connected) {
      currentView.value = 'connected'
    }
  }
})

onUnmounted(() => {
  cleanupEventListeners()
})
</script>

<template>
  <div class="app-container">
    <!-- Header -->
    <header class="header">
      <div class="header-left">
        <h1 class="title">VSP Manager</h1>
        <span class="version">v{{ version }}</span>
      </div>
      <div class="header-right">
        <span v-if="loggedIn" class="user-info">{{ username }}</span>
        <button v-if="loggedIn" class="btn btn-small btn-outline" @click="logout">Logout</button>
      </div>
    </header>

    <!-- Driver warning -->
    <div v-if="!driverInstalled" class="warning-banner">
      <strong>Warning:</strong> com0com driver not installed. Virtual serial ports will not work.
      Please install com0com from <a href="https://sourceforge.net/projects/com0com/" target="_blank">SourceForge</a>.
    </div>

    <!-- Error message -->
    <div v-if="error" class="error-banner">
      {{ error }}
      <button class="btn btn-small" @click="error = ''">Dismiss</button>
    </div>

    <!-- Loading overlay -->
    <div v-if="loading" class="loading-overlay">
      <div class="spinner"></div>
    </div>

    <!-- Login View -->
    <div v-if="currentView === 'login'" class="view login-view">
      <div class="card">
        <h2>Login</h2>

        <div class="form-group">
          <label>Server URL</label>
          <input type="text" v-model="serverUrl" placeholder="http://192.168.1.100:9000">
        </div>

        <div class="form-group">
          <label>Username</label>
          <input type="text" v-model="loginUsername" placeholder="admin">
        </div>

        <div class="form-group">
          <label>Password</label>
          <input type="password" v-model="loginPassword" placeholder="admin123">
        </div>

        <div class="form-group checkbox">
          <label>
            <input type="checkbox" v-model="autoConnect">
            Auto-connect on startup
          </label>
        </div>

        <button class="btn btn-primary btn-large" @click="login" :disabled="loading">
          Login
        </button>
      </div>
    </div>

    <!-- Devices View -->
    <div v-if="currentView === 'devices'" class="view devices-view">
      <div class="card">
        <h2>Available Devices</h2>

        <div class="toolbar">
          <button class="btn btn-small btn-outline" @click="fetchDevices" :disabled="loading">
            Refresh
          </button>
        </div>

        <div v-if="devices.length === 0" class="empty-state">
          No devices available. Create a device on the server first.
        </div>

        <div v-else class="device-list">
          <div
            v-for="device in devices"
            :key="device.device_key"
            class="device-item"
            :class="{ selected: selectedDeviceKey === device.device_key }"
            @click="selectedDeviceKey = device.device_key"
          >
            <div class="device-info">
              <span class="device-name">{{ device.name }}</span>
              <span class="device-status" :class="device.status">
                {{ device.status || 'offline' }}
              </span>
            </div>
            <div class="device-details">
              <span v-if="device.serial_port">{{ device.serial_port }} @ {{ device.baud_rate }}</span>
            </div>
          </div>
        </div>

        <button
          class="btn btn-primary btn-large"
          @click="connect"
          :disabled="loading || !selectedDeviceKey"
        >
          Connect
        </button>
      </div>
    </div>

    <!-- Connected View -->
    <div v-if="currentView === 'connected'" class="view connected-view">
      <div class="status-card">
        <div class="status-header">
          <h2>Connection Status</h2>
          <span class="connection-badge" :class="{ online: status.connected, offline: !status.connected }">
            {{ status.connected ? 'Connected' : 'Disconnected' }}
          </span>
        </div>

        <div class="status-grid">
          <div class="status-item">
            <label>Virtual COM Port</label>
            <span class="value">{{ status.visible_port || 'N/A' }}</span>
          </div>

          <div class="status-item">
            <label>Device Status</label>
            <span class="value" :class="{ online: status.device_online, offline: !status.device_online }">
              {{ status.device_online ? 'Online' : 'Offline' }}
            </span>
          </div>

          <div class="status-item">
            <label>Connected Since</label>
            <span class="value">{{ status.connected_since || 'N/A' }}</span>
          </div>

          <div class="status-item">
            <label>Data Sent</label>
            <span class="value">{{ formatBytes(status.bytes_sent) }}</span>
          </div>

          <div class="status-item">
            <label>Data Received</label>
            <span class="value">{{ formatBytes(status.bytes_received) }}</span>
          </div>
        </div>

        <div class="info-box">
          <p><strong>How to use:</strong></p>
          <p>Connect to <strong>{{ status.visible_port }}</strong> using any serial terminal application (e.g., PuTTY, RealTerm, or your PLC programming software).</p>
          <p>The virtual COM port will relay all data through the cloud server to the remote device.</p>
        </div>

        <button class="btn btn-danger btn-large" @click="disconnect" :disabled="loading">
          Disconnect
        </button>
      </div>
    </div>
  </div>
</template>

<style>
:root {
  --primary-color: #42b883;
  --primary-dark: #33a06f;
  --danger-color: #e74c3c;
  --danger-dark: #c0392b;
  --bg-color: #1b2636;
  --card-bg: #2d3a4f;
  --text-color: #f0f0f0;
  --text-muted: #8a9ba8;
  --border-color: #3d4f66;
  --online-color: #2ecc71;
  --offline-color: #e74c3c;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family: Inter, Avenir, Helvetica, Arial, sans-serif;
  font-size: 14px;
  line-height: 1.5;
  color: var(--text-color);
  background-color: var(--bg-color);
}

.app-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* Header */
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: var(--card-bg);
  border-bottom: 1px solid var(--border-color);
}

.header-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.title {
  font-size: 20px;
  font-weight: 700;
  color: var(--primary-color);
  margin: 0;
}

.version {
  font-size: 12px;
  color: var(--text-muted);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-info {
  color: var(--text-muted);
}

/* Banners */
.warning-banner {
  background: #f39c12;
  color: #1b2636;
  padding: 12px 24px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.warning-banner a {
  color: #1b2636;
  text-decoration: underline;
}

.error-banner {
  background: var(--danger-color);
  color: white;
  padding: 12px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/* Loading overlay */
.loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(27, 38, 54, 0.8);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid var(--border-color);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Views */
.view {
  flex: 1;
  display: flex;
  justify-content: center;
  padding: 24px;
}

/* Cards */
.card {
  background: var(--card-bg);
  border-radius: 8px;
  padding: 24px;
  width: 100%;
  max-width: 500px;
}

.card h2 {
  margin: 0 0 20px 0;
  font-size: 18px;
  font-weight: 600;
}

/* Form */
.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--text-muted);
}

.form-group input[type="text"],
.form-group input[type="password"],
.form-group input[type="number"] {
  width: 100%;
  padding: 10px 12px;
  font-size: 14px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-color);
  color: var(--text-color);
}

.form-group input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.checkbox label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.checkbox input {
  width: 16px;
  height: 16px;
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 600;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--primary-color);
  color: var(--bg-color);
}

.btn-primary:hover:not(:disabled) {
  background: var(--primary-dark);
}

.btn-danger {
  background: var(--danger-color);
  color: white;
}

.btn-danger:hover:not(:disabled) {
  background: var(--danger-dark);
}

.btn-outline {
  background: transparent;
  border: 1px solid var(--border-color);
  color: var(--text-color);
}

.btn-outline:hover:not(:disabled) {
  background: var(--bg-color);
}

.btn-small {
  padding: 6px 12px;
  font-size: 12px;
}

.btn-large {
  width: 100%;
  padding: 12px 24px;
  margin-top: 8px;
}

/* Toolbar */
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

/* Empty state */
.empty-state {
  text-align: center;
  padding: 24px;
  color: var(--text-muted);
}

/* Device list */
.device-list {
  margin-bottom: 16px;
}

.device-item {
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.device-item:hover {
  background: var(--bg-color);
}

.device-item.selected {
  border-color: var(--primary-color);
  background: rgba(66, 184, 131, 0.1);
}

.device-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.device-name {
  font-weight: 600;
}

.device-status {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 12px;
  background: var(--border-color);
}

.device-status.online {
  background: var(--online-color);
  color: var(--bg-color);
}

.device-status.offline {
  background: var(--offline-color);
  color: white;
}

.device-details {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
}

/* Status card */
.status-card {
  background: var(--card-bg);
  border-radius: 8px;
  padding: 24px;
  width: 100%;
  max-width: 600px;
}

.status-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.status-header h2 {
  margin: 0;
}

.connection-badge {
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
}

.connection-badge.online {
  background: var(--online-color);
  color: var(--bg-color);
}

.connection-badge.offline {
  background: var(--offline-color);
  color: white;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.status-item {
  background: var(--bg-color);
  padding: 12px;
  border-radius: 4px;
}

.status-item label {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 4px;
}

.status-item .value {
  font-size: 16px;
  font-weight: 600;
}

.status-item .value.online {
  color: var(--online-color);
}

.status-item .value.offline {
  color: var(--offline-color);
}

.info-box {
  background: var(--bg-color);
  padding: 16px;
  border-radius: 4px;
  margin-bottom: 20px;
  font-size: 13px;
}

.info-box p {
  margin: 0 0 8px 0;
}

.info-box p:last-child {
  margin: 0;
}
</style>
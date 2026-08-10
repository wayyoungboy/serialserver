<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'

const version = ref('')
const loggedIn = ref(false)
const username = ref('')
const serverUrl = ref('http://localhost:9000')
const loginUsername = ref('')
const loginPassword = ref('')
const autoConnect = ref(false)
const listenAddress = ref('127.0.0.1:7000')
const searchQuery = ref('')
const devices = ref([])
const mappings = ref([])
const selectedDeviceId = ref(0)
const selectedMappingId = ref('')
const loading = ref(false)
const error = ref('')
const status = ref(emptyStatus())
const sessionLog = ref([])
const locale = ref(loadInitialLocale())

let statusListener = null

const messages = {
  en: {
    signIn: 'Sign in',
    server: 'Server',
    username: 'Username',
    password: 'Password',
    autoConnect: 'Auto connect',
    connect: 'Connect',
    disconnect: 'Disconnect',
    logout: 'Logout',
    devices: 'Devices',
    refresh: 'Refresh',
    dismiss: 'Dismiss',
    searchDevices: 'Search devices',
    noDeviceSelected: 'No device selected',
    deviceId: 'Device ID',
    mappings: 'Mappings',
    lastSeen: 'Last seen',
    localTcp: 'Local TCP',
    listening: 'Listening',
    idle: 'Idle',
    mapping: 'Mapping',
    listenAddress: 'Listen address',
    relay: 'Relay',
    noSession: 'No session',
    status: 'Status',
    remote: 'Remote',
    connected: 'Connected',
    online: 'Online',
    offline: 'Offline',
    busy: 'Busy',
    tx: 'TX Local -> Remote',
    rx: 'RX Remote -> Local',
    sessionLog: 'Session log',
    clear: 'Clear',
    noOnlineMappings: 'No online mappings',
    noEvents: 'No events',
    localTcpReady: 'Local TCP ready on {address}',
    gatewayStopped: 'Gateway stopped',
    signedOut: 'Signed out',
    signedInAs: 'Signed in as {username}',
    loadedDevices: 'Loaded {count} devices',
    dashboardReady: 'Dashboard ready',
    relayReady: 'Relay session ready',
    waitingLocalClient: 'Waiting for local TCP client'
  },
  zh: {
    signIn: '登录',
    server: '服务器',
    username: '用户名',
    password: '密码',
    autoConnect: '自动连接',
    connect: '连接',
    disconnect: '断开',
    logout: '退出登录',
    devices: '设备',
    refresh: '刷新',
    dismiss: '关闭',
    searchDevices: '搜索设备',
    noDeviceSelected: '未选择设备',
    deviceId: '设备 ID',
    mappings: '映射',
    lastSeen: '最后在线',
    localTcp: '本地 TCP',
    listening: '监听中',
    idle: '空闲',
    mapping: '映射',
    listenAddress: '监听地址',
    relay: '中继',
    noSession: '无会话',
    status: '状态',
    remote: '远端',
    connected: '连接时间',
    online: '在线',
    offline: '离线',
    busy: '占用',
    tx: '发送 本地 -> 远端',
    rx: '接收 远端 -> 本地',
    sessionLog: '会话日志',
    clear: '清空',
    noOnlineMappings: '暂无在线映射',
    noEvents: '暂无事件',
    localTcpReady: '本地 TCP 已就绪：{address}',
    gatewayStopped: '网关已停止',
    signedOut: '已退出登录',
    signedInAs: '已登录：{username}',
    loadedDevices: '已加载 {count} 台设备',
    dashboardReady: '控制台已就绪',
    relayReady: '中继会话已就绪',
    waitingLocalClient: '等待本地 TCP 客户端'
  }
}

const bridge = computed(() => window.go?.main?.App)

const selectedDevice = computed(() => devices.value.find((device) => device.id === selectedDeviceId.value) || null)
const selectedMapping = computed(() => mappings.value.find((item) => item.mapping?.id === selectedMappingId.value) || null)
const filteredDevices = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return devices.value
  return devices.value.filter((device) => {
    return [device.name, String(device.id), device.status]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(query))
  })
})

const canConnect = computed(() => {
  return loggedIn.value && selectedDeviceId.value && selectedMappingId.value && listenAddress.value && !status.value.local_listening
})

function emptyStatus() {
  return {
    connected: false,
    local_listening: false,
    relay_connected: false,
    listen_address: '',
    device_id: 0,
    mapping_id: '',
    mapping_name: '',
    remote_port: '',
    session_id: '',
    bytes_sent: 0,
    bytes_received: 0,
    connected_since: '',
    error: '',
    last_event: ''
  }
}

function loadInitialLocale() {
  const saved = localStorage.getItem('vsp-manager-locale')
  if (saved === 'zh' || saved === 'en') return saved
  return navigator.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

function setLocale(value) {
  locale.value = value
  localStorage.setItem('vsp-manager-locale', value)
}

function t(key, params = {}) {
  const template = messages[locale.value]?.[key] || messages.en[key] || key
  return Object.entries(params).reduce((text, [name, value]) => {
    return text.replaceAll(`{${name}}`, String(value))
  }, template)
}

function addLog(level, message) {
  sessionLog.value.unshift({
    time: new Date().toLocaleTimeString(),
    level,
    message
  })
  sessionLog.value = sessionLog.value.slice(0, 80)
}

async function withLoading(task) {
  loading.value = true
  error.value = ''
  try {
    return await task()
  } catch (e) {
    error.value = String(e)
    addLog('ERROR', String(e))
  } finally {
    loading.value = false
  }
}

async function getVersion() {
  if (bridge.value) {
    version.value = await bridge.value.GetVersion()
  } else {
    version.value = '0.0.3'
  }
}

async function loadConfig() {
  if (!bridge.value) {
    seedMockData()
    return
  }

  const config = await bridge.value.LoadConfig()
  serverUrl.value = config?.server_url || 'http://localhost:9000'
  loginUsername.value = config?.username || ''
  autoConnect.value = Boolean(config?.auto_connect)
  listenAddress.value = config?.listen_address || '127.0.0.1:7000'
  selectedDeviceId.value = config?.device_id || 0
  selectedMappingId.value = config?.mapping_id || ''
}

async function login() {
  await withLoading(async () => {
    if (bridge.value) {
      await bridge.value.SaveConfig(serverUrl.value, autoConnect.value)
      const user = await bridge.value.Login(loginUsername.value, loginPassword.value)
      username.value = user?.username || loginUsername.value
    } else {
      username.value = loginUsername.value || 'engineer01'
    }
    loggedIn.value = true
    addLog('INFO', t('signedInAs', { username: username.value }))
    await fetchDevices()
  })
}

async function logout() {
  await withLoading(async () => {
    if (bridge.value) {
      await bridge.value.Logout()
    }
    loggedIn.value = false
    username.value = ''
    devices.value = []
    mappings.value = []
    selectedDeviceId.value = 0
    selectedMappingId.value = ''
    status.value = emptyStatus()
    addLog('INFO', t('signedOut'))
  })
}

async function fetchDevices() {
  await withLoading(async () => {
    if (bridge.value) {
      devices.value = await bridge.value.GetDevices() || []
    } else {
      seedMockData()
    }

    if (!selectedDeviceId.value && devices.value.length > 0) {
      selectedDeviceId.value = devices.value[0].id
    }
    if (selectedDeviceId.value) {
      await fetchMappings()
    }
    addLog('INFO', t('loadedDevices', { count: devices.value.length }))
  })
}

async function selectDevice(device) {
  selectedDeviceId.value = device.id
  selectedMappingId.value = ''
  await fetchMappings()
}

async function fetchMappings() {
  if (!selectedDeviceId.value) return

  if (bridge.value) {
    mappings.value = await bridge.value.GetMappings(selectedDeviceId.value) || []
  } else {
    mappings.value = mockMappingsFor(selectedDeviceId.value)
  }

  if (!selectedMappingId.value && mappings.value.length > 0) {
    const firstAvailable = mappings.value.find((item) => item.online && !item.busy) || mappings.value[0]
    selectedMappingId.value = firstAvailable.mapping.id
  }
}

async function connect() {
  if (!canConnect.value) return

  await withLoading(async () => {
    if (bridge.value) {
      await bridge.value.Connect(selectedDeviceId.value, selectedMappingId.value, listenAddress.value)
      status.value = await bridge.value.GetStatus()
    } else {
      status.value = {
        ...emptyStatus(),
        connected: true,
        local_listening: true,
        relay_connected: true,
        listen_address: listenAddress.value,
        device_id: selectedDeviceId.value,
        mapping_id: selectedMappingId.value,
        mapping_name: selectedMapping.value?.mapping?.name || selectedMappingId.value,
        remote_port: selectedMapping.value?.mapping?.serial?.port || '',
        session_id: 'demo-session',
        connected_since: new Date().toLocaleString(),
        last_event: t('relayReady')
      }
    }
    addLog('INFO', t('localTcpReady', { address: listenAddress.value }))
  })
}

async function disconnect() {
  await withLoading(async () => {
    if (bridge.value) {
      await bridge.value.Disconnect()
      status.value = await bridge.value.GetStatus()
    } else {
      status.value = emptyStatus()
    }
    addLog('INFO', t('gatewayStopped'))
  })
}

async function updateStatus() {
  if (!bridge.value) return
  status.value = await bridge.value.GetStatus()
}

function setupEventListeners() {
  if (!window.runtime?.EventsOn) return
  statusListener = window.runtime.EventsOn('statusUpdate', (data) => {
    try {
      const parsed = JSON.parse(data)
      status.value = parsed
      if (parsed.last_event) addLog(parsed.error ? 'ERROR' : 'INFO', parsed.last_event)
    } catch (e) {
      console.error('Failed to parse status update:', e)
    }
  })
}

function cleanupEventListeners() {
  if (statusListener && window.runtime?.EventsOff) {
    window.runtime.EventsOff('statusUpdate')
  }
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function serialLabel(mapping) {
  const serial = mapping?.mapping?.serial
  if (!serial) return ''
  return `${serial.port} ${serial.baud_rate || serial.baudRate},${serial.parity || 'N'},${serial.data_bits || serial.dataBits},${serial.stop_bits || serial.stopBits}`
}

function statusLabel(device) {
  return device.status || 'offline'
}

function displayState(value) {
  const state = String(value || 'offline').toLowerCase()
  if (state.includes('busy')) return t('busy')
  if (state.includes('online') || state === 'active') return t('online')
  return t('offline')
}

function displayEvent(message) {
  if (!message) return ''
  if (message === 'Waiting for local TCP client') return t('waitingLocalClient')
  if (message.startsWith('Relay session ready')) return t('relayReady')
  return message
}

function seedMockData() {
  devices.value = [
    { id: 1, name: 'PLC-01', status: 'online', last_online: '2026-08-10 18:40:12' },
    { id: 2, name: 'RTU-02', status: 'online', last_online: '2026-08-10 18:38:04' },
    { id: 3, name: 'METER-03', status: 'offline', last_online: '2026-08-09 22:10:09' }
  ]
  if (!selectedDeviceId.value) selectedDeviceId.value = 1
  mappings.value = mockMappingsFor(selectedDeviceId.value)
  if (!selectedMappingId.value) selectedMappingId.value = 'plc-fx3u'
}

function mockMappingsFor(deviceId) {
  if (deviceId === 3) return []
  return [
    {
      mapping: {
        id: deviceId === 1 ? 'plc-fx3u' : 'rtu-main',
        name: deviceId === 1 ? 'PLC FX3U' : 'RTU Main',
        serial: {
          port: deviceId === 1 ? 'COM3' : '/dev/ttyUSB0',
          baud_rate: deviceId === 1 ? 9600 : 115200,
          data_bits: 8,
          stop_bits: 1,
          parity: 'N',
          flow_control: 'none'
        }
      },
      online: true,
      busy: false
    },
    {
      mapping: {
        id: 'console',
        name: 'Console',
        serial: {
          port: deviceId === 1 ? 'COM4' : '/dev/ttyUSB1',
          baud_rate: 115200,
          data_bits: 8,
          stop_bits: 1,
          parity: 'N',
          flow_control: 'none'
        }
      },
      online: true,
      busy: deviceId === 2
    }
  ]
}

onMounted(async () => {
  await getVersion()
  await loadConfig()
  setupEventListeners()

  if (bridge.value) {
    loggedIn.value = await bridge.value.IsLoggedIn()
    if (loggedIn.value) {
      username.value = await bridge.value.GetCurrentUsername()
      await fetchDevices()
      await updateStatus()
    }
  } else {
    loggedIn.value = true
    username.value = 'engineer01'
    addLog('INFO', t('dashboardReady'))
  }
})

onUnmounted(() => {
  cleanupEventListeners()
})
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <div class="brand">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M6 7.5h12M6 12h12M6 16.5h12" />
          <path d="M4 5.5h16v14H4z" />
        </svg>
        <div>
          <h1>VSP Manager</h1>
          <span>v{{ version }}</span>
        </div>
      </div>

      <div class="account">
        <select class="language-select" :value="locale" @change="setLocale($event.target.value)">
          <option value="zh">中文</option>
          <option value="en">English</option>
        </select>
        <span v-if="loggedIn">{{ username }}</span>
        <button v-if="loggedIn" class="icon-button" :title="t('logout')" @click="logout">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M10 7V5a2 2 0 0 1 2-2h7v18h-7a2 2 0 0 1-2-2v-2" />
            <path d="M15 12H3m4-4-4 4 4 4" />
          </svg>
        </button>
      </div>
    </header>

    <main v-if="!loggedIn" class="login-layout">
      <section class="login-panel">
        <h2>{{ t('signIn') }}</h2>
        <label>
          <span>{{ t('server') }}</span>
          <input v-model="serverUrl" type="text" placeholder="http://localhost:9000" />
        </label>
        <label>
          <span>{{ t('username') }}</span>
          <input v-model="loginUsername" type="text" placeholder="admin" />
        </label>
        <label>
          <span>{{ t('password') }}</span>
          <input v-model="loginPassword" type="password" />
        </label>
        <label class="check-row">
          <input v-model="autoConnect" type="checkbox" />
          <span>{{ t('autoConnect') }}</span>
        </label>
        <button class="primary-action" :disabled="loading" @click="login">{{ t('connect') }}</button>
      </section>
    </main>

    <main v-else class="workspace">
      <aside class="device-rail">
        <div class="rail-heading">
          <h2>{{ t('devices') }}</h2>
          <button class="icon-button" :title="t('refresh')" :disabled="loading" @click="fetchDevices">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M20 12a8 8 0 0 1-14.8 4.2M4 12A8 8 0 0 1 18.8 7.8" />
              <path d="M20 5v5h-5M4 19v-5h5" />
            </svg>
          </button>
        </div>

        <div class="search-box">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="11" cy="11" r="7" />
            <path d="m16 16 4 4" />
          </svg>
          <input v-model="searchQuery" type="text" :placeholder="t('searchDevices')" />
        </div>

        <div class="device-list">
          <button
            v-for="device in filteredDevices"
            :key="device.id"
            class="device-row"
            :class="{ selected: selectedDeviceId === device.id }"
            @click="selectDevice(device)"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 5h16v6H4zM4 13h16v6H4z" />
              <path d="M8 8h.01M8 16h.01M12 8h4M12 16h4" />
            </svg>
            <span>
              <strong>{{ device.name }}</strong>
              <small>{{ t('deviceId') }} {{ device.id }}</small>
            </span>
            <em :class="['state-dot', statusLabel(device)]"></em>
          </button>
        </div>
      </aside>

      <section class="content-grid">
        <div v-if="error" class="alert">
          <span>{{ error }}</span>
          <button class="icon-button" :title="t('dismiss')" @click="error = ''">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M18 6 6 18M6 6l12 12" />
            </svg>
          </button>
        </div>

        <section class="device-summary panel">
          <div class="summary-title">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 5h16v14H4z" />
              <path d="M8 9h8M8 13h5" />
            </svg>
            <div>
              <h2>{{ selectedDevice?.name || t('noDeviceSelected') }}</h2>
              <span :class="['status-text', selectedDevice?.status || 'offline']">
                {{ displayState(selectedDevice?.status) }}
              </span>
            </div>
          </div>
          <dl>
            <div>
              <dt>{{ t('server') }}</dt>
              <dd>{{ serverUrl }}</dd>
            </div>
            <div>
              <dt>{{ t('deviceId') }}</dt>
              <dd>{{ selectedDevice?.id || '-' }}</dd>
            </div>
            <div>
              <dt>{{ t('mappings') }}</dt>
              <dd>{{ mappings.length }}</dd>
            </div>
            <div>
              <dt>{{ t('lastSeen') }}</dt>
              <dd>{{ selectedDevice?.last_online || '-' }}</dd>
            </div>
          </dl>
        </section>

        <section class="gateway-panel panel">
          <div class="panel-title">
            <h2>{{ t('localTcp') }}</h2>
            <span>{{ status.local_listening ? t('listening') : t('idle') }}</span>
          </div>
          <label>
            <span>{{ t('mapping') }}</span>
            <select v-model="selectedMappingId">
              <option v-for="item in mappings" :key="item.mapping.id" :value="item.mapping.id">
                {{ item.mapping.name || item.mapping.id }}
              </option>
            </select>
          </label>
          <label>
            <span>{{ t('listenAddress') }}</span>
            <input v-model="listenAddress" type="text" placeholder="127.0.0.1:7000" />
          </label>
          <div class="action-row">
            <button class="primary-action" :disabled="!canConnect || loading" @click="connect">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M7 7h10v10H7z" />
                <path d="M12 2v5M12 17v5M2 12h5M17 12h5" />
              </svg>
              {{ t('connect') }}
            </button>
            <button class="secondary-action" :disabled="!status.local_listening || loading" @click="disconnect">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M18 6 6 18M6 6l12 12" />
              </svg>
              {{ t('disconnect') }}
            </button>
          </div>
        </section>

        <section class="mappings-panel panel">
          <div class="panel-title">
            <h2>{{ t('mappings') }}</h2>
            <button class="text-button" :disabled="loading || !selectedDeviceId" @click="fetchMappings">{{ t('refresh') }}</button>
          </div>
          <div class="table">
            <div class="table-head">
              <span>{{ t('mapping') }}</span>
              <span>{{ t('remote') }}</span>
              <span>{{ t('status') }}</span>
              <span>{{ t('localTcp') }}</span>
            </div>
            <button
              v-for="item in mappings"
              :key="item.mapping.id"
              class="table-row"
              :class="{ selected: selectedMappingId === item.mapping.id }"
              @click="selectedMappingId = item.mapping.id"
            >
              <span>{{ item.mapping.name || item.mapping.id }}</span>
              <span>{{ serialLabel(item) }}</span>
              <span>
                <em :class="['state-dot', item.busy ? 'busy' : item.online ? 'online' : 'offline']"></em>
                {{ item.busy ? t('busy') : item.online ? t('online') : t('offline') }}
              </span>
              <span>{{ selectedMappingId === item.mapping.id ? listenAddress : '-' }}</span>
            </button>
            <div v-if="mappings.length === 0" class="empty-row">{{ t('noOnlineMappings') }}</div>
          </div>
        </section>

        <section class="relay-panel panel">
          <div class="panel-title">
            <h2>{{ t('relay') }}</h2>
            <span>{{ status.session_id || t('noSession') }}</span>
          </div>
          <dl>
            <div>
              <dt>{{ t('status') }}</dt>
              <dd :class="['status-text', status.relay_connected ? 'online' : 'offline']">
                {{ status.relay_connected ? t('online') : t('offline') }}
              </dd>
            </div>
            <div>
              <dt>{{ t('remote') }}</dt>
              <dd>{{ status.remote_port || selectedMapping?.mapping?.serial?.port || '-' }}</dd>
            </div>
            <div>
              <dt>{{ t('localTcp') }}</dt>
              <dd>{{ status.listen_address || listenAddress }}</dd>
            </div>
            <div>
              <dt>{{ t('connected') }}</dt>
              <dd>{{ status.connected_since || '-' }}</dd>
            </div>
          </dl>
          <div class="meter-grid">
            <div>
              <small>{{ t('tx') }}</small>
              <strong>{{ formatBytes(status.bytes_sent) }}</strong>
            </div>
            <div>
              <small>{{ t('rx') }}</small>
              <strong>{{ formatBytes(status.bytes_received) }}</strong>
            </div>
          </div>
        </section>

        <section class="log-panel panel">
          <div class="panel-title">
            <h2>{{ t('sessionLog') }}</h2>
            <button class="text-button" @click="sessionLog = []">{{ t('clear') }}</button>
          </div>
          <div class="log-table">
            <div v-for="entry in sessionLog" :key="entry.time + entry.message" class="log-row">
              <time>{{ entry.time }}</time>
              <span :class="entry.level.toLowerCase()">{{ entry.level }}</span>
              <p>{{ displayEvent(entry.message) }}</p>
            </div>
            <div v-if="sessionLog.length === 0" class="empty-row">{{ t('noEvents') }}</div>
          </div>
        </section>
      </section>
    </main>

    <div v-if="loading" class="loading-bar"></div>
  </div>
</template>

<style>
:root {
  --bg: #f5f7fa;
  --surface: #ffffff;
  --surface-soft: #f9fbfd;
  --border: #dce3ea;
  --border-strong: #c8d3dd;
  --text: #1f2937;
  --muted: #667085;
  --accent: #087f95;
  --accent-dark: #056579;
  --online: #138a3d;
  --busy: #b7790f;
  --offline: #cf2f35;
  --shadow: 0 12px 34px rgba(31, 41, 55, 0.08);
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family: Inter, "Segoe UI", Arial, sans-serif;
  font-size: 14px;
  color: var(--text);
  background: var(--bg);
  letter-spacing: 0;
}

button,
input,
select {
  font: inherit;
}

button {
  border: 0;
}

.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg);
}

.topbar {
  height: 58px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.brand,
.account,
.summary-title,
.rail-heading,
.panel-title,
.action-row {
  display: flex;
  align-items: center;
}

.brand {
  gap: 12px;
}

.brand svg,
.summary-title svg {
  width: 32px;
  height: 32px;
  color: var(--accent);
}

svg {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.brand h1 {
  margin: 0;
  font-size: 20px;
  line-height: 1.1;
  font-weight: 760;
  color: var(--accent);
}

.brand span,
.account span,
dt,
small {
  color: var(--muted);
}

.account {
  gap: 10px;
}

.language-select {
  width: 104px;
  min-height: 34px;
  padding: 6px 8px;
  color: var(--text);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 13px;
  font-weight: 650;
}

.icon-button {
  width: 34px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--muted);
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
}

.icon-button svg {
  width: 19px;
  height: 19px;
}

.icon-button:hover:not(:disabled) {
  background: var(--surface-soft);
  color: var(--accent);
}

.icon-button:disabled,
.text-button:disabled,
.primary-action:disabled,
.secondary-action:disabled {
  opacity: 0.52;
  cursor: not-allowed;
}

.login-layout {
  flex: 1;
  display: grid;
  place-items: center;
  padding: 32px;
}

.login-panel,
.panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: var(--shadow);
}

.login-panel {
  width: min(430px, 100%);
  padding: 28px;
}

.login-panel h2,
.panel h2,
.rail-heading h2 {
  margin: 0;
  font-size: 18px;
  line-height: 1.25;
  font-weight: 720;
}

label {
  display: grid;
  gap: 7px;
  margin-top: 16px;
  color: var(--muted);
  font-size: 13px;
  font-weight: 650;
}

input,
select {
  width: 100%;
  min-height: 40px;
  padding: 9px 11px;
  color: var(--text);
  background: #fff;
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  outline: 0;
}

input:focus,
select:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(8, 127, 149, 0.12);
}

.check-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 18px 0;
}

.check-row input {
  width: 16px;
  min-height: 16px;
}

.workspace {
  flex: 1;
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  min-height: 0;
}

.device-rail {
  min-height: calc(100vh - 58px);
  background: var(--surface);
  border-right: 1px solid var(--border);
}

.rail-heading {
  height: 62px;
  justify-content: space-between;
  padding: 0 16px;
}

.search-box {
  height: 42px;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 10px 10px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface-soft);
}

.search-box svg {
  width: 18px;
  height: 18px;
  color: var(--muted);
}

.search-box input {
  min-height: auto;
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.device-list {
  display: grid;
}

.device-row {
  width: 100%;
  display: grid;
  grid-template-columns: 28px 1fr 12px;
  align-items: center;
  gap: 12px;
  min-height: 72px;
  padding: 12px 16px;
  text-align: left;
  color: var(--text);
  background: transparent;
  border-top: 1px solid var(--border);
  cursor: pointer;
}

.device-row svg {
  width: 28px;
  height: 28px;
  color: #344054;
}

.device-row strong,
.device-row small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-row small {
  margin-top: 2px;
}

.device-row.selected {
  background: #e9f6f8;
  box-shadow: inset 4px 0 0 var(--accent);
}

.content-grid {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(420px, 1fr) 360px;
  grid-template-rows: auto auto minmax(190px, 1fr);
  gap: 10px;
  padding: 10px;
}

.panel {
  padding: 16px;
  min-width: 0;
  box-shadow: none;
}

.device-summary {
  grid-column: 1;
}

.gateway-panel {
  grid-column: 2;
  grid-row: 1;
}

.mappings-panel {
  grid-column: 1;
  grid-row: 2;
}

.relay-panel {
  grid-column: 2;
  grid-row: 2;
}

.log-panel {
  grid-column: 1 / -1;
  grid-row: 3;
  min-height: 210px;
}

.summary-title {
  gap: 12px;
  margin-bottom: 18px;
}

.summary-title h2 {
  font-size: 24px;
}

dl {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 26px;
  margin: 0;
}

dt {
  font-size: 12px;
  font-weight: 680;
  margin-bottom: 4px;
}

dd {
  margin: 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 650;
}

.panel-title {
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.panel-title span {
  color: var(--muted);
  font-size: 12px;
  font-weight: 650;
}

.action-row {
  gap: 10px;
  margin-top: 18px;
}

.primary-action,
.secondary-action {
  min-height: 40px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 16px;
  border-radius: 6px;
  font-weight: 730;
  cursor: pointer;
}

.primary-action {
  flex: 1;
  color: white;
  background: var(--accent);
}

.primary-action:hover:not(:disabled) {
  background: var(--accent-dark);
}

.secondary-action {
  color: var(--accent);
  background: white;
  border: 1px solid var(--accent);
}

.primary-action svg,
.secondary-action svg {
  width: 18px;
  height: 18px;
}

.text-button {
  padding: 6px 8px;
  color: var(--accent);
  background: transparent;
  border-radius: 6px;
  font-weight: 700;
  cursor: pointer;
}

.table,
.log-table {
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.table-head,
.table-row {
  display: grid;
  grid-template-columns: 1.1fr 1.5fr 0.9fr 1.2fr;
  align-items: center;
  min-height: 44px;
}

.table-head {
  color: var(--muted);
  background: var(--surface-soft);
  font-size: 12px;
  font-weight: 760;
}

.table-head span,
.table-row span {
  min-width: 0;
  padding: 0 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.table-row {
  width: 100%;
  color: var(--text);
  text-align: left;
  background: white;
  border-top: 1px solid var(--border);
  cursor: pointer;
}

.table-row.selected {
  background: #eef8fa;
}

.state-dot {
  width: 9px;
  height: 9px;
  display: inline-block;
  margin-right: 8px;
  border-radius: 50%;
  background: var(--offline);
}

.state-dot.online,
.state-dot.active {
  background: var(--online);
}

.state-dot.busy {
  background: var(--busy);
}

.state-dot.offline,
.state-dot.disabled {
  background: var(--offline);
}

.status-text {
  color: var(--offline);
  font-weight: 730;
  text-transform: capitalize;
}

.status-text.online,
.status-text.active {
  color: var(--online);
}

.status-text.busy {
  color: var(--busy);
}

.meter-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 16px;
}

.meter-grid div {
  display: grid;
  gap: 8px;
  padding: 12px;
  background: var(--surface-soft);
  border: 1px solid var(--border);
  border-radius: 6px;
}

.meter-grid strong {
  font-size: 18px;
}

.log-table {
  max-height: 220px;
  overflow: auto;
}

.log-row {
  display: grid;
  grid-template-columns: 92px 74px minmax(0, 1fr);
  align-items: start;
  min-height: 34px;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
}

.log-row:first-child {
  border-top: 0;
}

.log-row time,
.log-row span {
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}

.log-row span.error {
  color: var(--offline);
}

.log-row p {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.empty-row {
  padding: 18px;
  color: var(--muted);
  text-align: center;
}

.alert {
  grid-column: 1 / -1;
  min-height: 44px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  color: #8a1f24;
  background: #fff2f2;
  border: 1px solid #f2c6c8;
  border-radius: 6px;
}

.loading-bar {
  position: fixed;
  left: 0;
  right: 0;
  top: 58px;
  height: 3px;
  overflow: hidden;
  background: rgba(8, 127, 149, 0.14);
}

.loading-bar::after {
  content: "";
  display: block;
  width: 40%;
  height: 100%;
  background: var(--accent);
  animation: loading 1.15s infinite ease-in-out;
}

@keyframes loading {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(260%); }
}

@media (max-width: 980px) {
  .workspace {
    grid-template-columns: 1fr;
  }

  .device-rail {
    min-height: auto;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }

  .device-list {
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  }

  .content-grid {
    grid-template-columns: 1fr;
    grid-template-rows: auto;
  }

  .device-summary,
  .gateway-panel,
  .mappings-panel,
  .relay-panel,
  .log-panel {
    grid-column: 1;
    grid-row: auto;
  }
}
</style>

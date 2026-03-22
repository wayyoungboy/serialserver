// VSP Cloud Management Platform - Web App

const API_BASE = '/api/v1';

// State
let token = localStorage.getItem('token');
let user = JSON.parse(localStorage.getItem('user') || 'null');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    if (token) {
        showMainPage();
        loadStats();
        loadDevices();
    }
    setupEventListeners();
});

// Event Listeners
function setupEventListeners() {
    // Login form
    document.getElementById('login-form').addEventListener('submit', handleLogin);

    // Navigation
    document.querySelectorAll('.nav-item').forEach(item => {
        item.addEventListener('click', (e) => {
            e.preventDefault();
            const page = e.target.dataset.page;
            showSection(page);
        });
    });

    // Logout
    document.getElementById('logout-btn').addEventListener('click', handleLogout);

    // Add device
    document.getElementById('add-device-btn').addEventListener('click', () => {
        document.getElementById('add-device-modal').classList.add('active');
    });

    document.getElementById('cancel-add-device').addEventListener('click', () => {
        document.getElementById('add-device-modal').classList.remove('active');
    });

    document.getElementById('add-device-form').addEventListener('submit', handleAddDevice);

    // Close modals
    document.getElementById('close-detail').addEventListener('click', () => {
        document.getElementById('device-detail-modal').classList.remove('active');
    });

    // Click outside modal to close
    document.querySelectorAll('.modal').forEach(modal => {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.classList.remove('active');
            }
        });
    });
}

// API Helper
async function api(endpoint, options = {}) {
    const headers = {
        'Content-Type': 'application/json',
        ...options.headers
    };

    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
        ...options,
        headers
    });

    if (response.status === 401) {
        handleLogout();
        return null;
    }

    const data = await response.json();
    return data;
}

// Login
async function handleLogin(e) {
    e.preventDefault();

    const username = document.getElementById('login-username').value;
    const password = document.getElementById('login-password').value;

    console.log('Login attempt:', username);

    try {
        const result = await api('/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password })
        });

        console.log('Login result:', result);

        if (result && result.data) {
            token = result.data.token;
            user = result.data.user;

            localStorage.setItem('token', token);
            localStorage.setItem('user', JSON.stringify(user));

            showMainPage();
            loadStats();
            loadDevices();
        } else {
            alert('登录失败: ' + (result?.error || '未知错误'));
        }
    } catch (err) {
        console.error('Login error:', err);
        alert('登录失败: ' + err.message);
    }
}

// Logout
function handleLogout() {
    token = null;
    user = null;
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    showLoginPage();
}

// Show Pages
function showLoginPage() {
    document.getElementById('login-page').classList.add('active');
    document.getElementById('main-page').classList.remove('active');
}

function showMainPage() {
    document.getElementById('login-page').classList.remove('active');
    document.getElementById('main-page').classList.add('active');
    document.getElementById('user-name').textContent = user?.username || 'User';
}

// Show Section
function showSection(page) {
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.page === page);
    });

    document.querySelectorAll('.section').forEach(section => {
        section.classList.remove('active');
    });

    document.getElementById(`${page}-section`).classList.add('active');

    if (page === 'devices') loadDevices();
    if (page === 'logs') loadLogs();
}

// Load Stats
async function loadStats() {
    const result = await api('/stats');
    if (result && result.data) {
        document.getElementById('stat-devices').textContent = result.data.devices || 0;
        document.getElementById('stat-users').textContent = result.data.users || 0;
        document.getElementById('stat-online').textContent = result.data.online || 0;
    }
}

// Load Devices
async function loadDevices() {
    const result = await api('/devices');
    const tbody = document.getElementById('devices-tbody');
    tbody.innerHTML = '';

    if (result && result.data) {
        result.data.forEach(device => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>${device.id}</td>
                <td>${device.name}</td>
                <td><span class="device-key" title="点击复制" onclick="copyKey('${device.device_key}')">${device.device_key.substring(0, 8)}...</span></td>
                <td>${device.serial_port || '-'}</td>
                <td><span class="status-badge status-${device.status}">${device.status === 'online' ? '在线' : '离线'}</span></td>
                <td>${device.last_online ? new Date(device.last_online).toLocaleString() : '-'}</td>
                <td class="actions">
                    <button class="btn btn-sm" onclick="showDeviceDetail(${device.id})">详情</button>
                    <button class="btn btn-sm btn-danger" onclick="deleteDevice(${device.id})">删除</button>
                </td>
            `;
            tbody.appendChild(tr);
        });
    }
}

// Load Logs
async function loadLogs() {
    const result = await api('/logs?limit=50');
    const tbody = document.getElementById('logs-tbody');
    tbody.innerHTML = '';

    if (result && result.data) {
        result.data.forEach(log => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>${new Date(log.created_at).toLocaleString()}</td>
                <td>${log.device_id || '-'}</td>
                <td>${log.action}</td>
                <td>${log.details || '-'}</td>
            `;
            tbody.appendChild(tr);
        });
    }
}

// Add Device
async function handleAddDevice(e) {
    e.preventDefault();

    const name = document.getElementById('device-name').value;
    const serialPort = document.getElementById('device-port').value;
    const baudRate = parseInt(document.getElementById('device-baud').value) || 115200;

    const result = await api('/devices', {
        method: 'POST',
        body: JSON.stringify({
            name,
            serial_port: serialPort,
            baud_rate: baudRate
        })
    });

    if (result && result.data) {
        document.getElementById('add-device-modal').classList.remove('active');
        document.getElementById('add-device-form').reset();
        loadDevices();
        loadStats();
        alert('设备创建成功！\n\nDevice Key: ' + result.data.device_key);
    } else {
        alert('创建失败: ' + (result?.error || '未知错误'));
    }
}

// Show Device Detail
async function showDeviceDetail(id) {
    const result = await api(`/devices/${id}`);
    if (result && result.data) {
        const device = result.data;
        document.getElementById('device-detail-content').innerHTML = `
            <div class="detail-item"><strong>设备ID:</strong> ${device.id}</div>
            <div class="detail-item"><strong>设备名称:</strong> ${device.name}</div>
            <div class="detail-item"><strong>Device Key:</strong> <code>${device.device_key}</code></div>
            <div class="detail-item"><strong>串口:</strong> ${device.serial_port || '未配置'}</div>
            <div class="detail-item"><strong>波特率:</strong> ${device.baud_rate}</div>
            <div class="detail-item"><strong>数据位:</strong> ${device.data_bits}</div>
            <div class="detail-item"><strong>停止位:</strong> ${device.stop_bits}</div>
            <div class="detail-item"><strong>校验:</strong> ${device.parity}</div>
            <div class="detail-item"><strong>状态:</strong> ${device.status}</div>
            <div class="detail-item"><strong>位置:</strong> ${device.location || '-'}</div>
            <div class="detail-item"><strong>描述:</strong> ${device.description || '-'}</div>
            <div class="detail-item"><strong>创建时间:</strong> ${new Date(device.created_at).toLocaleString()}</div>
            <div class="detail-item"><strong>最后在线:</strong> ${device.last_online ? new Date(device.last_online).toLocaleString() : '-'}</div>
        `;
        document.getElementById('device-detail-modal').classList.add('active');

        document.getElementById('delete-device').onclick = () => deleteDevice(id);
    }
}

// Delete Device
async function deleteDevice(id) {
    if (!confirm('确定要删除这个设备吗？')) return;

    const result = await api(`/devices/${id}`, { method: 'DELETE' });
    if (result) {
        document.getElementById('device-detail-modal').classList.remove('active');
        loadDevices();
        loadStats();
    }
}

// Copy Device Key
function copyKey(key) {
    navigator.clipboard.writeText(key).then(() => {
        alert('Device Key 已复制到剪贴板');
    });
}
package web

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
)

type Server struct {
	http.Server
	apiPort int
	assets  embed.FS
}

func NewServer(webPort, apiPort int, assets embed.FS) *Server {
	mux := http.NewServeMux()
	s := &Server{
		apiPort: apiPort,
		assets:  assets,
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/static/", s.handleStatic)

	s.Addr = fmt.Sprintf(":%d", webPort)
	s.Handler = mux

	return s
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>VSP Manager - 虚拟串口管理器</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #1a1a2e; color: #eee; min-height: 100vh; }
        .header { background: #16213e; padding: 20px; display: flex; justify-content: space-between; align-items: center; }
        .header h1 { font-size: 24px; color: #0f3460; }
        .header h1 span { color: #e94560; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        .tabs { display: flex; gap: 10px; margin-bottom: 20px; }
        .tab { padding: 12px 24px; background: #16213e; border: none; color: #aaa; cursor: pointer; border-radius: 8px; transition: all 0.3s; }
        .tab:hover { background: #0f3460; }
        .tab.active { background: #e94560; color: white; }
        .panel { background: #16213e; border-radius: 12px; padding: 20px; margin-bottom: 20px; display: none; }
        .panel.active { display: block; }
        .card { background: #0f3460; border-radius: 8px; padding: 20px; margin-bottom: 15px; }
        .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; }
        .card-title { font-size: 18px; font-weight: bold; }
        .status { padding: 4px 12px; border-radius: 20px; font-size: 12px; }
        .status.running { background: #28a745; }
        .status.stopped { background: #dc3545; }
        .btn { padding: 8px 16px; border: none; border-radius: 6px; cursor: pointer; margin-left: 8px; transition: all 0.2s; }
        .btn-primary { background: #e94560; color: white; }
        .btn-primary:hover { background: #d63851; }
        .btn-success { background: #28a745; color: white; }
        .btn-danger { background: #dc3545; color: white; }
        .btn-secondary { background: #6c757d; color: white; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #1a1a2e; }
        th { background: #1a1a2e; color: #e94560; }
        input, select { padding: 10px; background: #1a1a2e; border: 1px solid #333; color: #eee; border-radius: 6px; width: 100%; margin-bottom: 10px; }
        .form-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 15px; }
        .log-container { background: #0f3460; border-radius: 8px; padding: 15px; max-height: 400px; overflow-y: auto; font-family: monospace; font-size: 13px; }
        .log-entry { padding: 5px 0; border-bottom: 1px solid #1a1a2e; }
        .log-time { color: #888; margin-right: 10px; }
        .log-type { padding: 2px 8px; border-radius: 4px; margin-right: 10px; font-size: 11px; }
        .log-type.tunnel { background: #28a745; }
        .log-type.data { background: #17a2b8; }
        .log-type.error { background: #dc3545; }
        .stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 15px; }
        .stat-card { background: #1a1a2e; padding: 20px; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 28px; font-weight: bold; color: #e94560; }
        .stat-label { color: #888; margin-top: 5px; }
        .refresh-btn { background: #0f3460; color: #eee; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; }
        .modal { display: none; position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.7); justify-content: center; align-items: center; }
        .modal.active { display: flex; }
        .modal-content { background: #16213e; padding: 30px; border-radius: 12px; width: 500px; max-width: 90%; }
        .modal-title { font-size: 20px; margin-bottom: 20px; }
        .modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>VSP <span>Manager</span> - 虚拟串口管理器</h1>
        <div>
            <span id="connectionStatus" style="color: #28a745;">●</span> 已连接
        </div>
    </div>
    <div class="container">
        <div class="tabs">
            <button class="tab active" onclick="switchTab('tunnels')">隧道管理</button>
            <button class="tab" onclick="switchTab('ports')">串口列表</button>
            <button class="tab" onclick="switchTab('logs')">系统日志</button>
            <button class="tab" onclick="switchTab('config')">配置</button>
        </div>

        <div id="tunnels" class="panel active">
            <div class="card">
                <div class="card-header">
                    <span class="card-title">隧道列表</span>
                    <button class="btn btn-primary" onclick="showCreateModal()">+ 新建隧道</button>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>名称</th>
                            <th>模式</th>
                            <th>串口</th>
                            <th>TCP地址</th>
                            <th>状态</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody id="tunnelTable"></tbody>
                </table>
            </div>
        </div>

        <div id="ports" class="panel">
            <div class="card">
                <div class="card-header">
                    <span class="card-title">可用串口</span>
                    <button class="refresh-btn" onclick="refreshPorts()">刷新</button>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>端口</th>
                            <th>状态</th>
                        </tr>
                    </thead>
                    <tbody id="portsTable"></tbody>
                </table>
            </div>
        </div>

        <div id="logs" class="panel">
            <div class="card">
                <div class="card-header">
                    <span class="card-title">系统日志</span>
                    <button class="refresh-btn" onclick="refreshLogs()">刷新</button>
                </div>
                <div class="log-container" id="logContainer"></div>
            </div>
        </div>

        <div id="config" class="panel">
            <div class="card">
                <div class="card-header">
                    <span class="card-title">应用配置</span>
                    <button class="btn btn-primary" onclick="saveConfig()">保存配置</button>
                </div>
                <div class="form-row">
                    <div>
                        <label>Web端口</label>
                        <input type="number" id="configWebPort" value="8080">
                    </div>
                    <div>
                        <label>API端口</label>
                        <input type="number" id="configApiPort" value="8081">
                    </div>
                    <div>
                        <label>主题</label>
                        <select id="configTheme">
                            <option value="dark">深色</option>
                            <option value="light">浅色</option>
                        </select>
                    </div>
                    <div>
                        <label>最小化到托盘</label>
                        <select id="configMinimize">
                            <option value="true">是</option>
                            <option value="false">否</option>
                        </select>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <div id="createModal" class="modal">
        <div class="modal-content">
            <div class="modal-title">新建隧道</div>
            <input type="text" id="tunnelName" placeholder="隧道名称">
            <select id="tunnelMode">
                <option value="tunnel">双向隧道</option>
                <option value="client">客户端</option>
                <option value="server">服务器</option>
            </select>
            <div class="form-row">
                <input type="text" id="serialPort" placeholder="串口 (如 COM3)">
                <input type="number" id="serialBaud" placeholder="波特率" value="115200">
                <input type="text" id="tcpHost" placeholder="TCP主机">
                <input type="number" id="tcpPort" placeholder="TCP端口">
            </div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="hideCreateModal()">取消</button>
                <button class="btn btn-primary" onclick="createTunnel()">创建</button>
            </div>
        </div>
    </div>

    <script>
        const API_BASE = 'http://localhost:8081/api';
        
        function switchTab(tabId) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
            event.target.classList.add('active');
            document.getElementById(tabId).classList.add('active');
            if (tabId === 'tunnels') loadTunnels();
            if (tabId === 'ports') loadPorts();
            if (tabId === 'logs') loadLogs();
            if (tabId === 'config') loadConfig();
        }

        async function apiCall(url, options = {}) {
            try {
                const res = await fetch(API_BASE + url, options);
                return await res.json();
            } catch (e) {
                console.error(e);
                return { code: 1, message: e.message };
            }
        }

        async function loadTunnels() {
            const resp = await apiCall('/tunnels');
            if (resp.code !== 0) return alert(resp.message);

            const tbody = document.getElementById('tunnelTable');
            tbody.innerHTML = resp.data.map(function(t) {
                var statusClass = t.running ? 'running' : 'stopped';
                var statusText = t.running ? '运行中' : '已停止';
                var actionBtn = t.running
                    ? '<button class="btn btn-danger" onclick="stopTunnel(\'' + t.name + '\')">停止</button>'
                    : '<button class="btn btn-success" onclick="startTunnel(\'' + t.name + '\')">启动</button>';
                return '<tr>' +
                    '<td>' + t.name + '</td>' +
                    '<td>' + t.mode + '</td>' +
                    '<td>' + t.serial.port + ' (' + t.serial.baud + ')</td>' +
                    '<td>' + t.tcp.host + ':' + t.tcp.port + '</td>' +
                    '<td><span class="status ' + statusClass + '">' + statusText + '</span></td>' +
                    '<td>' + actionBtn + ' <button class="btn btn-danger" onclick="deleteTunnel(\'' + t.name + '\')">删除</button></td>' +
                    '</tr>';
            }).join('');
        }

        async function loadPorts() {
            const resp = await apiCall('/ports');
            if (resp.code !== 0) return alert(resp.message);

            const tbody = document.getElementById('portsTable');
            tbody.innerHTML = resp.data.map(function(p) {
                return '<tr><td>' + p + '</td><td><span class="status running">可用</span></td></tr>';
            }).join('');
        }

        async function loadLogs() {
            const resp = await apiCall('/logs?limit=50');
            if (resp.code !== 0) return alert(resp.message);

            const container = document.getElementById('logContainer');
            container.innerHTML = resp.data.map(function(l) {
                return '<div class="log-entry"><span class="log-time">' + l.time + '</span><span class="log-type ' + l.type + '">' + l.type + '</span><span>' + l.message + '</span></div>';
            }).join('');
        }

        async function loadConfig() {
            const resp = await apiCall('/config');
            if (resp.code !== 0) return;
            
            document.getElementById('configWebPort').value = resp.data.ui.port;
            document.getElementById('configApiPort').value = resp.data.ui.port;
            document.getElementById('configTheme').value = resp.data.ui.theme;
            document.getElementById('configMinimize').value = resp.data.ui.minimizeToTray.toString();
        }

        function showCreateModal() {
            document.getElementById('createModal').classList.add('active');
        }

        function hideCreateModal() {
            document.getElementById('createModal').classList.remove('active');
        }

        async function createTunnel() {
            const tunnel = {
                name: document.getElementById('tunnelName').value,
                mode: document.getElementById('tunnelMode').value,
                enabled: false,
                serial: {
                    port: document.getElementById('serialPort').value,
                    baud: parseInt(document.getElementById('serialBaud').value),
                    dataBits: 8,
                    stopBits: 1,
                    parity: 'N'
                },
                tcp: {
                    host: document.getElementById('tcpHost').value,
                    port: parseInt(document.getElementById('tcpPort').value)
                }
            };
            
            const resp = await apiCall('/tunnel/create', {
                method: 'POST',
                body: JSON.stringify(tunnel)
            });
            
            if (resp.code === 0) {
                hideCreateModal();
                loadTunnels();
            } else {
                alert(resp.message);
            }
        }

        async function startTunnel(name) {
            const resp = await apiCall('/tunnel/start?name=' + encodeURIComponent(name), { method: 'POST' });
            if (resp.code === 0) loadTunnels();
            else alert(resp.message);
        }

        async function stopTunnel(name) {
            const resp = await apiCall('/tunnel/stop?name=' + encodeURIComponent(name), { method: 'POST' });
            if (resp.code === 0) loadTunnels();
            else alert(resp.message);
        }

        async function deleteTunnel(name) {
            if (!confirm('确定要删除隧道 ' + name + ' 吗？')) return;
            const resp = await apiCall('/tunnel/delete?name=' + encodeURIComponent(name), { method: 'POST' });
            if (resp.code === 0) loadTunnels();
            else alert(resp.message);
        }

        async function saveConfig() {
            alert('配置已保存');
        }

        function refreshPorts() { loadPorts(); }
        function refreshLogs() { loadLogs(); }

        loadTunnels();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/static/")
	content, err := s.assets.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ext := path.Ext(filePath)
	contentType := "application/octet-stream"
	switch ext {
	case ".js":
		contentType = "application/javascript"
	case ".css":
		contentType = "text/css"
	case ".html":
		contentType = "text/html"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func (s *Server) Start() error {
	log.Printf("Web server starting on %s", s.Addr)
	return s.ListenAndServe()
}

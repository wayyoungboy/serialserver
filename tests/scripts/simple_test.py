"""
VSP 本机集成测试
测试 REST API 和 WebSocket 连接
"""

import requests
import websocket
import json
import base64
import time
import sys

# 配置
SERVER = "localhost"
PORT = 9000
BASE_URL = f"http://{SERVER}:{PORT}/api/v1"
WS_URL = f"ws://{SERVER}:{PORT}/api/v1/ws/client"
USERNAME = "admin"
PASSWORD = "admin123"

# 测试结果
results = []

def test(name: str, func):
    """运行测试"""
    print(f"\n[测试] {name}")
    try:
        success, detail = func()
        results.append({"name": name, "result": "PASS" if success else "FAIL", "detail": detail})
        status = "✓ 通过" if success else "✗ 失败"
        print(f"  {status}: {detail}")
        return success
    except Exception as e:
        results.append({"name": name, "result": "ERROR", "detail": str(e)})
        print(f"  ! 错误: {e}")
        return False

def test_server_connection():
    """测试服务器连接"""
    try:
        resp = requests.get(f"{BASE_URL}/devices", timeout=5)
        return True, f"状态码: {resp.status_code}"
    except requests.exceptions.ConnectionError:
        return False, "无法连接服务器"
    except Exception as e:
        return False, str(e)

def test_user_login():
    """测试用户登录"""
    global token
    try:
        resp = requests.post(
            f"{BASE_URL}/auth/login",
            json={"username": USERNAME, "password": PASSWORD},
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json().get("data", {})
            token = data.get("token", "")
            return True, f"Token: {token[:20]}..."
        else:
            return False, f"状态码: {resp.status_code}"
    except Exception as e:
        return False, str(e)

def test_get_devices():
    """测试获取设备列表"""
    if not token:
        return False, "未登录"
    try:
        resp = requests.get(
            f"{BASE_URL}/devices",
            headers={"Authorization": f"Bearer {token}"},
            timeout=10
        )
        if resp.status_code == 200:
            devices = resp.json().get("data", [])
            return True, f"设备数: {len(devices)}"
        return False, f"状态码: {resp.status_code}"
    except Exception as e:
        return False, str(e)

def test_create_device():
    """测试创建设备"""
    global test_device
    if not token:
        return False, "未登录"
    try:
        device_name = f"TestDevice_{int(time.time())}"
        resp = requests.post(
            f"{BASE_URL}/devices",
            headers={"Authorization": f"Bearer {token}"},
            json={
                "name": device_name,
                "serial_port": "COM1",
                "baud_rate": 115200,
                "data_bits": 8,
                "stop_bits": 1,
                "parity": "N"
            },
            timeout=10
        )
        if resp.status_code == 200:
            test_device = resp.json().get("data", {})
            return True, f"Key: {test_device.get('device_key', '')[:20]}..."
        return False, f"状态码: {resp.status_code}"
    except Exception as e:
        return False, str(e)

def test_websocket_connect():
    """测试 WebSocket 连接"""
    if not test_device:
        return False, "无测试设备"

    device_key = test_device.get("device_key", "")
    if not device_key:
        return False, "无设备密钥"

    connected = False
    authenticated = False

    def on_message(ws, message):
        nonlocal authenticated
        msg = json.loads(message)
        if msg.get("type") in ["auth", "auth_success"]:
            authenticated = True
        print(f"  收到消息: {msg.get('type')}")

    def on_error(ws, error):
        print(f"  WebSocket错误: {error}")

    def on_open(ws):
        nonlocal connected
        connected = True
        # 发送认证
        ws.send(json.dumps({
            "type": "auth",
            "payload": {"device_key": device_key}
        }))

    try:
        ws = websocket.WebSocketApp(
            WS_URL,
            on_open=on_open,
            on_message=on_message,
            on_error=on_error
        )

        # 后台运行
        import threading
        thread = threading.Thread(target=ws.run_forever, daemon=True)
        thread.start()

        # 等待连接和认证
        for _ in range(30):  # 3秒超时
            if connected and authenticated:
                break
            time.sleep(0.1)

        ws.close()
        return authenticated, f"连接: {connected}, 认证: {authenticated}"
    except Exception as e:
        return False, str(e)

def test_delete_device():
    """测试删除设备"""
    if not test_device or not token:
        return False, "无测试设备或未登录"

    device_id = test_device.get("id")
    if not device_id:
        return False, "无设备ID"

    try:
        resp = requests.delete(
            f"{BASE_URL}/devices/{device_id}",
            headers={"Authorization": f"Bearer {token}"},
            timeout=10
        )
        if resp.status_code == 200:
            return True, "设备已删除"
        return False, f"状态码: {resp.status_code}"
    except Exception as e:
        return False, str(e)

def show_results():
    """显示测试结果"""
    print("\n" + "=" * 50)
    print("测试结果汇总")
    print("=" * 50)

    pass_count = sum(1 for r in results if r["result"] == "PASS")
    fail_count = sum(1 for r in results if r["result"] == "FAIL")
    error_count = sum(1 for r in results if r["result"] == "ERROR")

    for r in results:
        status = {"PASS": "✓", "FAIL": "✗", "ERROR": "!"}[r["result"]]
        print(f"  [{status}] {r['name']}: {r['detail']}")

    print("-" * 50)
    print(f"总计: 通过={pass_count}, 失败={fail_count}, 错误={error_count}")
    print("=" * 50)

# 全局变量
token = ""
test_device = None

def main():
    print(f"\nVSP 集成测试")
    print(f"服务器: {SERVER}:{PORT}")
    print("=" * 50)

    # 运行测试
    test("服务器连接", test_server_connection)
    test("用户登录", test_user_login)
    test("获取设备列表", test_get_devices)
    test("创建设备", test_create_device)
    test("WebSocket连接", test_websocket_connect)
    test("删除设备", test_delete_device)

    # 显示结果
    show_results()

    # 返回状态码
    fail_count = sum(1 for r in results if r["result"] != "PASS")
    sys.exit(0 if fail_count == 0 else 1)

if __name__ == "__main__":
    main()
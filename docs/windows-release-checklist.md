# Windows 发版检查清单

本清单用于 VSPManager Windows 正式安装包发版前验收。当前主线提供本地 TCP 网关，不依赖 com0com。

## 1. 图标和应用元数据

- 运行 `cd vsp-windows && go run tools/gen_windows_assets.go`。
- 确认 `vsp-windows/build/appicon.png` 存在。
- 确认 `vsp-windows/build/windows/icon.ico` 存在。
- 确认 `vsp-windows/wails.json` 的应用名称、公司名、产品名和图标配置符合发版版本。

## 2. Wails 构建

```powershell
cd vsp-windows
wails build -clean
```

验收:

- `vsp-windows/build/bin/VSPManager.exe` 存在。
- 双击启动后界面正常显示。
- 中文和英文切换正常。
- 登录、设备列表、在线映射刷新、本地 TCP 端口输入、启动和停止网关按钮状态正常。
- 桌面端只要求用户登录信息，不要求输入 DeviceKey。

## 3. NSIS 标准安装包

```powershell
cd vsp-windows
makensis /DAPP_VERSION=0.0.3 packaging/windows/VSPManager.nsi
```

验收:

- `vsp-windows/build/installer/VSPManager-*-Setup.exe` 存在。
- 安装包使用 VSP 图标。
- 安装包写入当前用户开始菜单快捷方式。
- 安装包写入当前用户桌面快捷方式。
- Windows “应用和功能”或卸载列表中能看到 VSPManager。

## 4. 安装后功能验证

在干净 Windows 环境中执行:

- 运行安装器，使用默认安装路径完成安装。
- 从开始菜单启动 VSPManager。
- 从桌面快捷方式启动 VSPManager。
- 配置服务端地址并登录测试账号。
- 选择在线设备映射。
- 启动本地 TCP 网关，例如 `127.0.0.1:7000`。
- 用串口调试工具或 TCP 调试工具连接本地端口，确认连接状态和字节计数变化。
- 停止网关后，本地端口应释放，界面状态回到可启动。

## 5. 卸载验证

- 通过开始菜单或 Windows “应用和功能”卸载 VSPManager。
- 确认开始菜单快捷方式被删除。
- 确认桌面快捷方式被删除。
- 确认安装目录被删除或只剩用户主动创建的运行日志。
- 重新安装一次，确认安装器不会因为旧快捷方式或卸载项残留失败。

## 6. 发版前命令

```bash
cd vsp-server && go test ./...
cd vsp-client && go test ./...
cd vsp-windows && go test ./...
cd vsp-windows/frontend && npm run build
cd tests/e2e && go test ./...
```

或直接运行:

```bash
make test
```

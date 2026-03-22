package ui

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	
	"vsp-client/internal/config"
	"vsp-client/internal/serial"
	"vsp-client/internal/tcp"
)

// App UI应用
type App struct {
	window        fyne.Window
	configMgr    *config.Manager
	serialMgr    *serial.PortManager
	tunnelMgr    *tcp.Manager
	
	tunnelList   *widget.List
	statusLabel  *widget.Label
	logView      *widget.TextGrid
	
	tunnels      []TunnelItem
}

// TunnelItem 隧道列表项
type TunnelItem struct {
	Name    string
	Mode    string
	Status  string
	Enabled bool
}

// New 创建UI应用
func New(serialMgr *serial.PortManager, tunnelMgr *tcp.Manager, configMgr *config.Manager) *App {
	return &App{
		serialMgr:  serialMgr,
		tunnelMgr:  tunnelMgr,
		configMgr:  configMgr,
		tunnels:    []TunnelItem{},
	}
}

// Run 运行UI
func (a *App) Run(window fyne.Window) {
	a.window = window
	window.SetTitle("VSP Manager - 虚拟串口管理器")
	window.Resize(fyne.NewSize(800, 600)

	// 创建主界面
	content := a.createMainContent()
	window.SetContent(content)
	
	// 加载配置中的隧道
	a.loadTunnels()
}

// createMainContent 创建主界面
func (a *App) createMainContent() fyne.CanvasObject {
	// 顶部标题栏
	title := canvas.NewText("VSP Manager", &fyne.Theme{})
	title.TextSize = 24
	title.Alignment = fyne.TextAlignCenter
	
	// 隧道列表
	a.tunnelList = widget.NewList(
		func() int { return len(a.tunnels) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Name"),
				widget.NewLabel("Mode"),
				widget.NewLabel("Status"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			container := item.(*fyne.Container)
			if id < len(a.tunnels) {
				tunnel := a.tunnels[id]
				container.Objects[0].(*widget.Label).SetText(tunnel.Name)
				container.Objects[1].(*widget.Label).SetText(tunnel.Mode)
				container.Objects[2].(*widget.Label).SetText(tunnel.Status)
			}
		},
	)
	
	// 按钮栏
	btnAdd := widget.NewButton("添加隧道", a.onAddTunnel)
	btnEdit := widget.NewButton("编辑隧道", a.onEditTunnel)
	btnDelete := widget.NewButton("删除隧道", a.onDeleteTunnel)
	btnStart := widget.NewButton("启动", a.onStartTunnel)
	btnStop := widget.NewButton("停止", a.onStopTunnel)
	
	buttonBar := container.NewHBox(
		btnAdd, btnEdit, btnDelete,
		layout.NewSpacer(),
		btnStart, btnStop,
	)
	
	// 日志区域
	logLabel := canvas.NewText("日志", &fyne.Theme{})
	logLabel.TextSize = 14
	
	a.logView = widget.NewTextGrid()
	a.logView.SetText("系统就绪\n")
	
	logContainer := container.NewVBox(
		logLabel,
		a.logView,
	)
	logContainer.Resize(fyne.NewSize(780, 200))
	
	// 状态栏
	a.statusLabel = widget.NewLabel("就绪")
	statusBar := container.NewHBox(
		a.statusLabel,
		layout.NewSpacer(),
		widget.NewLabel("v1.0.0"),
	)
	
	// 布局
	mainContainer := container.NewVBox(
		canvas.NewText("虚拟串口管理器", &fyne.Theme{}),
		widget.NewSeparator(),
		a.tunnelList,
		buttonBar,
		widget.NewSeparator(),
		logContainer,
		widget.NewSeparator(),
		statusBar,
	)
	
	return container.NewPadded(mainContainer)
}

// onAddTunnel 添加隧道
func (a *App) onAddTunnel() {
	log.Println("添加隧道")
	a.log("打开添加隧道对话框")
	
	// 创建添加对话框
	dialog.ShowCustomConfirm("添加隧道", "确定", "取消",
		a.createTunnelForm(nil), a.window)
}

// onEditTunnel 编辑隧道
func (a *App) onEditTunnel() {
	selected := a.tunnelList.SelectedIndex()
	if selected < 0 || selected >= len(a.tunnels) {
		dialog.ShowInformation("提示", "请先选择要编辑的隧道", a.window)
		return
	}
	
	tunnel := a.tunnels[selected]
	a.log(fmt.Sprintf("编辑隧道: %s", tunnel.Name))
	
	dialog.ShowCustomConfirm("编辑隧道", "确定", "取消",
		a.createTunnelForm(&tunnel), a.window)
}

// onDeleteTunnel 删除隧道
func (a *App) onDeleteTunnel() {
	selected := a.tunnelList.SelectedIndex()
	if selected < 0 || selected >= len(a.tunnels) {
		dialog.ShowInformation("提示", "请先选择要删除的隧道", a.window)
		return
	}
	
	tunnel := a.tunnels[selected]
	
	dialog.ShowConfirm("确认", fmt.Sprintf("确定要删除隧道 %s 吗?", tunnel.Name),
		func(confirmed bool) {
			if confirmed {
				a.deleteTunnel(selected)
			}
		}, a.window)
}

// createTunnelForm 创建隧道表单
func (a *App) createTunnelForm(tunnel *TunnelItem) fyne.CanvasObject {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("隧道名称")
	
	serialEntry := widget.NewEntry()
	serialEntry.SetPlaceHolder("COM1")
	
	baudSelect := widget.NewSelect([]string{"9600", "19200", "38400", "57600", "115200", "230400", "460800", "921600"}, func(s string) {})
	baudSelect.SetSelected("115200")
	
	modeSelect := widget.NewSelect([]string{"client", "server", "tunnel"}, func(s string) {})
	modeSelect.SetSelected("tunnel")
	
	tcpHostEntry := widget.NewEntry()
	tcpHostEntry.SetPlaceHolder("127.0.0.1")
	
	tcpPortEntry := widget.NewEntry()
	tcpPortEntry.SetPlaceHolder("9000")
	
	if tunnel != nil {
		nameEntry.SetText(tunnel.Name)
		modeSelect.SetSelected(tunnel.Mode)
	}
	
	form := container.NewVBox(
		widget.NewLabel("隧道名称"),
		nameEntry,
		widget.NewLabel("模式"),
		modeSelect,
		widget.NewLabel("串口"),
		serialEntry,
		widget.NewLabel("波特率"),
		baudSelect,
		widget.NewLabel("TCP地址"),
		tcpHostEntry,
		widget.NewLabel("TCP端口"),
		tcpPortEntry,
	)
	
	return form
}

// onStartTunnel 启动隧道
func (a *App) onStartTunnel() {
	selected := a.tunnelList.SelectedIndex()
	if selected < 0 || selected >= len(a.tunnels) {
		dialog.ShowInformation("提示", "请先选择要启动的隧道", a.window)
		return
	}
	
	tunnel := a.tunnels[selected]
	a.log(fmt.Sprintf("启动隧道: %s", tunnel.Name))
	
	// 启动隧道逻辑
	a.statusLabel.SetText(fmt.Sprintf("隧道 %s 已启动", tunnel.Name))
}

// onStopTunnel 停止隧道
func (a *App) onStopTunnel() {
	selected := a.tunnelList.SelectedIndex()
	if selected < 0 || selected >= len(a.tunnels) {
		dialog.ShowInformation("提示", "请先选择要停止的隧道", a.window)
		return
	}
	
	tunnel := a.tunnels[selected]
	a.log(fmt.Sprintf("停止隧道: %s", tunnel.Name))
	
	a.statusLabel.SetText(fmt.Sprintf("隧道 %s 已停止", tunnel.Name))
}

// loadTunnels 加载隧道列表
func (a *App) loadTunnels() {
	cfg := a.configMgr.Get()
	a.tunnels = make([]TunnelItem, 0, len(cfg.Tunnels))
	
	for _, t := range cfg.Tunnels {
		status := "已停止"
		if t.Enabled {
			status = "运行中"
		}
		
		a.tunnels = append(a.tunnels, TunnelItem{
			Name:    t.Name,
			Mode:    t.Mode,
			Status:  status,
			Enabled: t.Enabled,
		})
	}
	
	a.tunnelList.Refresh()
	a.log(fmt.Sprintf("加载了 %d 个隧道配置", len(a.tunnels)))
}

// deleteTunnel 删除隧道
func (a *App) deleteTunnel(index int) {
	if index >= len(a.tunnels) {
		return
	}
	
	tunnel := a.tunnels[index]
	
	// 停止隧道
	a.tunnelMgr.Stop(tunnel.Name)
	
	// 从配置中删除
	a.configMgr.RemoveTunnel(tunnel.Name)
	
	// 从列表中删除
	a.tunnels = append(a.tunnels[:index], a.tunnels[index+1:]...)
	a.tunnelList.Refresh()
	
	a.log(fmt.Sprintf("删除了隧道: %s", tunnel.Name))
	a.statusLabel.SetText("隧道已删除")
}

// log 添加日志
func (a *App) log(message string) {
	text := a.logView.Text()
	a.logView.SetText(text + message + "\n")
	
	// 自动滚动到底部
	rows := a.logView.RowLines()
	if rows > 0 {
		a.logView.ScrollToRow(rows - 1)
	}
}

// ShowAbout 显示关于对话框
func (a *App) ShowAbout() {
	dialog.ShowInformation("关于",
		"VSP Manager v1.0.0\n\n虚拟串口管理器\n支持串口转TCP、TCP转串口、串口共享", a.window)
}

// RefreshList 刷新列表
func (a *App) RefreshList() {
	a.loadTunnels()
}

using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Windows;
using System.IO;
using System.IO.Ports;
using VSPManager.Core.Configuration;
using VSPManager.Core.Driver;
using VSPManager.Core.Network;
using VSPManager.Models;

namespace VSPManager.ViewModels;

public partial class MainViewModel : ObservableObject
{
    private static readonly string LogFile = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "vspmanager.log");

    private void Log(string message)
    {
        var logLine = $"[{DateTime.Now:HH:mm:ss.fff}] {message}";
        try { File.AppendAllText(LogFile, logLine + "\n"); } catch { }
    }

    private readonly ConfigManager _configManager;
    private readonly Com0ComManager _com0comManager;
    private readonly ComPortClient _portClient;
    private readonly VspWsClient _wsClient;
    private CancellationTokenSource? _forwardCts;
    private readonly HttpClient _httpClient;

    [ObservableProperty]
    private ConnectionStatus _connectionStatus = new();

    [ObservableProperty]
    private long _bytesReceived;

    [ObservableProperty]
    private long _bytesSent;

    [ObservableProperty]
    private string _serverHost = "localhost";

    [ObservableProperty]
    private int _serverPort = 9000;

    [ObservableProperty]
    private string _username = "";

    [ObservableProperty]
    private string _password = "";

    [ObservableProperty]
    private string _token = "";

    [ObservableProperty]
    private string _currentUser = "";

    [ObservableProperty]
    private bool _isLoggedIn = false;

    [ObservableProperty]
    private string _portName = "VSP1";

    [ObservableProperty]
    private List<DeviceInfo> _devices = new();

    [ObservableProperty]
    private DeviceInfo? _selectedDevice;

    [ObservableProperty]
    private int _baudRate = 115200;

    [ObservableProperty]
    private int _dataBits = 8;

    [ObservableProperty]
    private string _stopBits = "1";

    [ObservableProperty]
    private string _parity = "None";

    [ObservableProperty]
    private string _deviceSerialPort = "COM3";

    [ObservableProperty]
    private bool _isConnecting;

    [ObservableProperty]
    private string _statusMessage = "请先登录";

    public bool CanConnect => !IsConnecting && ConnectionStatus.State != ConnectionState.Connected && SelectedDevice != null && IsLoggedIn;
    public bool CanDisconnect => ConnectionStatus.State == ConnectionState.Connected;

    public MainViewModel()
    {
        _configManager = new ConfigManager();
        _com0comManager = new Com0ComManager();
        _portClient = new ComPortClient();
        _wsClient = new VspWsClient();
        _httpClient = new HttpClient();

        LoadConfig();

        _wsClient.Connected += OnWsConnected;
        _wsClient.Disconnected += OnWsDisconnected;
        _wsClient.DataReceived += OnWsDataReceived;
        _wsClient.Error += OnWsError;
    }

    private void LoadConfig()
    {
        var config = _configManager.Config;
        ServerHost = config.ServerHost;
        ServerPort = config.ServerPort;
        PortName = config.PortName;
        DeviceSerialPort = config.DeviceSerialPort ?? "COM3";
        BaudRate = config.BaudRate;
        DataBits = config.DataBits;
        StopBits = config.StopBits;
        Parity = config.Parity;
        Token = config.Token ?? "";
        CurrentUser = config.Username ?? "";
    }

    private void SaveConfig()
    {
        _configManager.Config.ServerHost = ServerHost;
        _configManager.Config.ServerPort = ServerPort;
        _configManager.Config.PortName = PortName;
        _configManager.Config.DeviceSerialPort = DeviceSerialPort;
        _configManager.Config.BaudRate = BaudRate;
        _configManager.Config.DataBits = DataBits;
        _configManager.Config.StopBits = StopBits;
        _configManager.Config.Parity = Parity;
        _configManager.Config.Token = Token;
        _configManager.Config.Username = CurrentUser;
        _configManager.Save();
    }

    [RelayCommand]
    private async Task LoginAsync()
    {
        if (string.IsNullOrEmpty(Username) || string.IsNullOrEmpty(Password))
        {
            MessageBox.Show("请输入用户名和密码", "提示", MessageBoxButton.OK, MessageBoxImage.Warning);
            return;
        }

        try
        {
            StatusMessage = "正在登录...";
            var apiUrl = $"http://{ServerHost}:{ServerPort}/api/v1/auth/login";
            var loginData = new { username = Username, password = Password };
            var json = JsonSerializer.Serialize(loginData);
            var content = new StringContent(json, Encoding.UTF8, "application/json");

            var response = await _httpClient.PostAsync(apiUrl, content);
            var responseBody = await response.Content.ReadAsStringAsync();

            if (response.IsSuccessStatusCode)
            {
                var result = JsonSerializer.Deserialize<LoginResponse>(responseBody, new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
                if (result?.Data != null)
                {
                    Token = result.Data.Token;
                    CurrentUser = result.Data.User?.Username ?? Username;
                    IsLoggedIn = true;
                    SaveConfig();

                    StatusMessage = "加载设备中...";
                    await LoadDevicesAsync();
                    StatusMessage = "已登录";
                }
            }
            else
            {
                var errorResult = JsonSerializer.Deserialize<ErrorResponse>(responseBody, new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
                MessageBox.Show($"登录失败: {errorResult?.Error ?? response.StatusCode.ToString()}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
                StatusMessage = "登录失败";
            }
        }
        catch (Exception ex)
        {
            MessageBox.Show($"登录失败: {ex.Message}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
            StatusMessage = "登录失败";
        }
    }

    [RelayCommand]
    private void Logout()
    {
        // 断开连接
        if (ConnectionStatus.State == ConnectionState.Connected)
        {
            Disconnect();
        }

        // 清除登录状态
        Token = "";
        CurrentUser = "";
        IsLoggedIn = false;
        Devices = new List<DeviceInfo>();
        SelectedDevice = null;
        StatusMessage = "请先登录";
        SaveConfig();
    }

    private async Task LoadDevicesAsync()
    {
        try
        {
            var apiUrl = $"http://{ServerHost}:{ServerPort}/api/v1/devices";
            _httpClient.DefaultRequestHeaders.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", Token);

            var response = await _httpClient.GetAsync(apiUrl);
            var responseBody = await response.Content.ReadAsStringAsync();

            if (response.IsSuccessStatusCode)
            {
                var result = JsonSerializer.Deserialize<DevicesResponse>(responseBody, new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
                if (result?.Data != null)
                {
                    Devices = result.Data;
                    if (Devices.Count > 0)
                    {
                        SelectedDevice = Devices[0];
                        UpdateDeviceConfig();
                    }
                }
            }
        }
        catch (Exception ex)
        {
            MessageBox.Show($"加载设备失败: {ex.Message}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
        }
    }

    partial void OnSelectedDeviceChanged(DeviceInfo? value)
    {
        if (value != null)
        {
            UpdateDeviceConfig();
            OnPropertyChanged(nameof(CanConnect));
        }
    }

    private void UpdateDeviceConfig()
    {
        if (SelectedDevice != null)
        {
            BaudRate = SelectedDevice.BaudRate;
            DataBits = SelectedDevice.DataBits;
            StopBits = SelectedDevice.StopBits == 1 ? "1" : SelectedDevice.StopBits == 2 ? "2" : "1.5";
            Parity = SelectedDevice.Parity switch
            {
                "N" => "None",
                "O" => "Odd",
                "E" => "Even",
                _ => "None"
            };
            DeviceSerialPort = SelectedDevice.SerialPort ?? "COM3";
        }
    }

    [RelayCommand]
    private async Task SaveConfigAsync()
    {
        if (SelectedDevice == null)
        {
            MessageBox.Show("请先选择设备", "提示", MessageBoxButton.OK, MessageBoxImage.Warning);
            return;
        }

        try
        {
            SaveConfig();

            var apiUrl = $"http://{ServerHost}:{ServerPort}/api/v1/devices/{SelectedDevice.Id}/config";
            var configData = new
            {
                serial_port = DeviceSerialPort,
                baud_rate = BaudRate,
                data_bits = DataBits,
                stop_bits = StopBits == "1.5" ? 1.5 : int.Parse(StopBits),
                parity = Parity == "None" ? "N" : Parity == "Odd" ? "O" : "E"
            };

            var json = JsonSerializer.Serialize(configData);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            _httpClient.DefaultRequestHeaders.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", Token);
            var response = await _httpClient.PutAsync(apiUrl, content);

            if (response.IsSuccessStatusCode)
            {
                MessageBox.Show("配置已保存到服务器", "成功", MessageBoxButton.OK, MessageBoxImage.Information);
            }
            else
            {
                MessageBox.Show($"保存失败: {response.StatusCode}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
            }
        }
        catch (Exception ex)
        {
            MessageBox.Show($"保存配置失败: {ex.Message}", "错误", MessageBoxButton.OK, MessageBoxImage.Error);
        }
    }

    [RelayCommand]
    private async Task ConnectAsync()
    {
        Log($"ConnectAsync called: IsConnecting={IsConnecting}, SelectedDevice={SelectedDevice?.Name ?? "null"}");
        if (IsConnecting || SelectedDevice == null) return;

        IsConnecting = true;
        StatusMessage = "正在连接...";
        ConnectionStatus.State = ConnectionState.Connecting;

        try
        {
            SaveConfig();

            bool portOpened = false;
            string userPortName = "";
            string hiddenPort = "";

            // 检查 com0com 是否已安装
            if (_com0comManager.IsInstalled())
            {
                Log("com0com detected, creating/finding VSPManager port pair...");

                // 创建或获取 VSPManager 专用端口对
                var portPair = _com0comManager.EnsurePortPair(PortName.StartsWith("COM") ? PortName : null);

                if (portPair != null)
                {
                    userPortName = portPair.Value.VisiblePort;
                    hiddenPort = portPair.Value.HiddenPort;
                    Log($"Port pair: {userPortName} (visible) <-> {hiddenPort} (hidden)");

                    // 打开隐藏端口用于数据转发
                    portOpened = _portClient.OpenDirect(hiddenPort, BaudRate);
                    Log($"Hidden port opened: {portOpened}");
                }
                else
                {
                    Log("Failed to create port pair");
                }
            }
            else
            {
                Log("com0com not installed");
            }

            if (!portOpened)
            {
                StatusMessage = "虚拟串口不可用，仅连接服务器...";
            }

            // Connect to server via WebSocket
            Log($"Connecting WebSocket: {ServerHost}:{ServerPort}, DeviceKey={SelectedDevice.DeviceKey}");
            await _wsClient.ConnectAsync(ServerHost, ServerPort, SelectedDevice.DeviceKey);
            Log("WebSocket connected successfully");

            // Start data forwarding
            StartDataForwarding();

            ConnectionStatus.State = ConnectionState.Connected;
            ConnectionStatus.ConnectedTime = DateTime.Now;
            ConnectionStatus.ServerAddress = $"{ServerHost}:{ServerPort}";
            ConnectionStatus.PortName = portOpened ? userPortName : "(无虚拟串口)";
            StatusMessage = portOpened ? $"已连接 ({userPortName})" : "已连接 (无虚拟串口)";
        }
        catch (Exception ex)
        {
            Log($"ConnectAsync error: {ex.Message}\n{ex.StackTrace}");
            ConnectionStatus.State = ConnectionState.Error;
            ConnectionStatus.ErrorMessage = ex.Message;
            StatusMessage = $"连接失败: {ex.Message}";
            _portClient.Close();
        }
        finally
        {
            IsConnecting = false;
            OnPropertyChanged(nameof(CanConnect));
            OnPropertyChanged(nameof(CanDisconnect));
        }
    }

    [RelayCommand]
    private void Disconnect()
    {
        StopDataForwarding();
        _wsClient.Disconnect();
        _portClient.Close();

        // 删除虚拟串口对
        _com0comManager.RemoveCurrentPortPair();

        ConnectionStatus.State = ConnectionState.Disconnected;
        ConnectionStatus.ConnectedTime = null;
        StatusMessage = "已断开";
        BytesReceived = 0;
        BytesSent = 0;

        OnPropertyChanged(nameof(CanConnect));
        OnPropertyChanged(nameof(CanDisconnect));
    }

    private void StartDataForwarding()
    {
        _forwardCts = new CancellationTokenSource();
        var token = _forwardCts.Token;

        Log("StartDataForwarding: starting...");

        // Port -> WebSocket
        _ = Task.Run(async () =>
        {
            var buffer = new byte[8192];
            Log($"Forwarding task started: IsOpen={_portClient.IsOpen}, IsConnected={_wsClient.IsConnected}");
            while (!token.IsCancellationRequested && _portClient.IsOpen && _wsClient.IsConnected)
            {
                try
                {
                    var read = _portClient.Read(buffer, 0, buffer.Length);
                    if (read > 0)
                    {
                        Log($"Read {read} bytes from port, sending to WebSocket...");
                        await _wsClient.SendAsync(buffer.AsMemory(0, read).ToArray(), token);
                        BytesSent += read;
                        Log($"Sent {read} bytes, total sent: {BytesSent}");
                    }
                }
                catch (TimeoutException)
                {
                    // 超时是正常的，继续等待数据
                    continue;
                }
                catch (OperationCanceledException) { Log("Forwarding cancelled"); break; }
                catch (Exception ex)
                {
                    Log($"Forwarding error: {ex.Message}");
                    // 不要退出，继续尝试
                    await Task.Delay(100, token);
                }
            }
            Log("Forwarding task ended");
        }, token);
    }

    private void StopDataForwarding()
    {
        _forwardCts?.Cancel();
        _forwardCts?.Dispose();
        _forwardCts = null;
    }

    private void OnWsConnected(object? sender, EventArgs e)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            StatusMessage = "服务器连接成功";
        });
    }

    private void OnWsDisconnected(object? sender, EventArgs e)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            Disconnect();
        });
    }

    private async void OnWsDataReceived(object? sender, byte[] data)
    {
        Log($"OnWsDataReceived: {data.Length} bytes, IsOpen={_portClient.IsOpen}");
        if (_portClient.IsOpen)
        {
            try
            {
                _portClient.Write(data, 0, data.Length);
                BytesReceived += data.Length;
                Log($"Written {data.Length} bytes to port, total received: {BytesReceived}");
            }
            catch (Exception ex)
            {
                Log($"Write to port failed: {ex.Message}");
            }
        }
    }

    private void OnWsError(object? sender, Exception ex)
    {
        Application.Current.Dispatcher.Invoke(() =>
        {
            StatusMessage = $"网络错误: {ex.Message}";
        });
    }
}

// Response models
public class LoginResponse
{
    public LoginData? Data { get; set; }
}

public class LoginData
{
    public string Token { get; set; } = "";
    public UserInfo? User { get; set; }
}

public class UserInfo
{
    public string Username { get; set; } = "";
}

public class ErrorResponse
{
    public string Error { get; set; } = "";
}

public class DevicesResponse
{
    public List<DeviceInfo> Data { get; set; } = new();
}

public class DeviceInfo
{
    [System.Text.Json.Serialization.JsonPropertyName("id")]
    public int Id { get; set; }

    [System.Text.Json.Serialization.JsonPropertyName("name")]
    public string Name { get; set; } = "";

    [System.Text.Json.Serialization.JsonPropertyName("device_key")]
    public string DeviceKey { get; set; } = "";

    [System.Text.Json.Serialization.JsonPropertyName("serial_port")]
    public string? SerialPort { get; set; }

    [System.Text.Json.Serialization.JsonPropertyName("baud_rate")]
    public int BaudRate { get; set; } = 115200;

    [System.Text.Json.Serialization.JsonPropertyName("data_bits")]
    public int DataBits { get; set; } = 8;

    [System.Text.Json.Serialization.JsonPropertyName("stop_bits")]
    public int StopBits { get; set; } = 1;

    [System.Text.Json.Serialization.JsonPropertyName("parity")]
    public string Parity { get; set; } = "N";

    [System.Text.Json.Serialization.JsonPropertyName("status")]
    public string Status { get; set; } = "offline";
}
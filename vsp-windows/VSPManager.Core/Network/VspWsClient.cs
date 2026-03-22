using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using System.IO;

namespace VSPManager.Core.Network;

/// <summary>
/// WebSocket client for VSP network communication
/// </summary>
public class VspWsClient : IDisposable
{
    private static readonly string LogFile = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "vspmanager.log");
    private ClientWebSocket? _webSocket;
    private CancellationTokenSource? _cts;
    private bool _disposed;
    private bool _disconnecting;
    private string _deviceKey = "";

    private void Log(string message)
    {
        var logLine = $"[{DateTime.Now:HH:mm:ss.fff}] {message}";
        Console.WriteLine(logLine);
        try { File.AppendAllText(LogFile, logLine + "\n"); } catch { }
    }

    public event EventHandler<byte[]>? DataReceived;
    public event EventHandler? Connected;
    public event EventHandler? Disconnected;
    public event EventHandler<Exception>? Error;

    public bool IsConnected => _webSocket?.State == WebSocketState.Open;

    /// <summary>
    /// Connect to remote server via WebSocket
    /// </summary>
    public async Task ConnectAsync(string host, int port, string deviceKey, CancellationToken cancellationToken = default)
    {
        _deviceKey = deviceKey;
        _webSocket = new ClientWebSocket();

        var uri = new Uri($"ws://{host}:{port}/api/v1/ws/client");
        Log($"[VspWsClient] Connecting to: {uri}");
        await _webSocket.ConnectAsync(uri, cancellationToken);
        Log($"[VspWsClient] WebSocket connected, state: {_webSocket.State}");

        _cts = new CancellationTokenSource();

        // Send auth message
        var authMsg = new
        {
            type = "auth",
            payload = new { device_key = deviceKey }
        };
        var authJson = JsonSerializer.Serialize(authMsg);
        Log($"[VspWsClient] Sending auth: {authJson}");
        var authBytes = Encoding.UTF8.GetBytes(authJson);
        await _webSocket.SendAsync(new ArraySegment<byte>(authBytes), WebSocketMessageType.Text, true, cancellationToken);

        // Wait for auth response
        var buffer = new byte[4096];
        var result = await _webSocket.ReceiveAsync(new ArraySegment<byte>(buffer), cancellationToken);
        Log($"[VspWsClient] Received response, type: {result.MessageType}, count: {result.Count}");

        if (result.MessageType == WebSocketMessageType.Text)
        {
            var response = Encoding.UTF8.GetString(buffer, 0, result.Count);
            Log($"[VspWsClient] Response: {response}");
            var msg = JsonSerializer.Deserialize<WsMessage>(response);
            if (msg?.Type == "error")
            {
                throw new Exception($"认证失败: {msg.Payload}");
            }
        }

        Log("[VspWsClient] Auth successful, invoking Connected event");
        Connected?.Invoke(this, EventArgs.Empty);

        // Start receive loop
        _ = Task.Run(() => ReceiveLoop(_cts.Token), _cts.Token);
    }

    /// <summary>
    /// Disconnect from server
    /// </summary>
    public void Disconnect()
    {
        if (_disconnecting)
        {
            Log("[VspWsClient] Disconnect already in progress, skipping");
            return;
        }
        _disconnecting = true;
        Log("[VspWsClient] Disconnecting...");

        _cts?.Cancel();

        if (_webSocket?.State == WebSocketState.Open)
        {
            try
            {
                _webSocket.CloseAsync(WebSocketCloseStatus.NormalClosure, "Closing", CancellationToken.None).Wait();
            }
            catch (Exception ex)
            {
                Log($"[VspWsClient] Close error: {ex.Message}");
            }
        }

        _webSocket?.Dispose();
        _webSocket = null;

        Log("[VspWsClient] Firing Disconnected event");
        // Fire event while still in disconnecting state to prevent re-entry
        Disconnected?.Invoke(this, EventArgs.Empty);

        _disconnecting = false;
        Log("[VspWsClient] Disconnect complete");
    }

    /// <summary>
    /// Send data to server
    /// </summary>
    public async Task SendAsync(byte[] data, CancellationToken cancellationToken = default)
    {
        if (_webSocket == null || _webSocket.State != WebSocketState.Open)
            throw new InvalidOperationException("Not connected");

        var msg = new
        {
            type = "data",
            payload = new { data = data }
        };
        var json = JsonSerializer.Serialize(msg);
        var bytes = Encoding.UTF8.GetBytes(json);
        await _webSocket.SendAsync(new ArraySegment<byte>(bytes), WebSocketMessageType.Text, true, cancellationToken);
    }

    private async Task ReceiveLoop(CancellationToken cancellationToken)
    {
        var buffer = new byte[8192];
        Log("[VspWsClient] ReceiveLoop started");

        try
        {
            while (!cancellationToken.IsCancellationRequested && _webSocket?.State == WebSocketState.Open)
            {
                Log($"[VspWsClient] Waiting for message... State: {_webSocket?.State}");
                var result = await _webSocket.ReceiveAsync(new ArraySegment<byte>(buffer), cancellationToken);
                Log($"[VspWsClient] Received: type={result.MessageType}, count={result.Count}");

                if (result.MessageType == WebSocketMessageType.Close)
                {
                    Log("[VspWsClient] Server sent close message");
                    break;
                }

                if (result.MessageType == WebSocketMessageType.Text)
                {
                    var json = Encoding.UTF8.GetString(buffer, 0, result.Count);
                    Log($"[VspWsClient] Text message: {json}");

                    try
                    {
                        using var doc = JsonDocument.Parse(json);
                        var root = doc.RootElement;

                        if (root.TryGetProperty("type", out var typeEl) && typeEl.GetString() == "data")
                        {
                            if (root.TryGetProperty("payload", out var payloadEl) &&
                                payloadEl.TryGetProperty("data", out var dataEl))
                            {
                                var base64Data = dataEl.GetString();
                                if (!string.IsNullOrEmpty(base64Data))
                                {
                                    var data = Convert.FromBase64String(base64Data);
                                    Log($"[VspWsClient] Decoded base64 data: {data.Length} bytes");
                                    DataReceived?.Invoke(this, data);
                                }
                            }
                        }
                    }
                    catch (Exception ex)
                    {
                        Log($"[VspWsClient] JSON parse error: {ex.Message}");
                    }
                }
            }
        }
        catch (OperationCanceledException)
        {
            Log("[VspWsClient] ReceiveLoop cancelled");
        }
        catch (Exception ex)
        {
            Log($"[VspWsClient] ReceiveLoop error: {ex.Message}");
            Error?.Invoke(this, ex);
        }

        Log($"[VspWsClient] ReceiveLoop ended, _disconnecting={_disconnecting}");
        if (!_disconnecting)
        {
            Log("[VspWsClient] Firing Disconnected from ReceiveLoop");
            Disconnected?.Invoke(this, EventArgs.Empty);
        }
    }

    public void Dispose()
    {
        if (_disposed)
            return;

        Disconnect();
        _disposed = true;
        GC.SuppressFinalize(this);
    }

    private class WsMessage
    {
        public string Type { get; set; } = "";
        public object? Payload { get; set; }
    }

    private class DataPayload
    {
        public string? Data { get; set; }
    }
}
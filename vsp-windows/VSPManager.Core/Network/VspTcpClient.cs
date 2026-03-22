using System.Net.Sockets;

namespace VSPManager.Core.Network;

/// <summary>
/// TCP client for VSP network communication
/// </summary>
public class VspTcpClient : IDisposable
{
    private TcpClient? _tcpClient;
    private NetworkStream? _stream;
    private CancellationTokenSource? _cts;
    private bool _disposed;

    public event EventHandler<byte[]>? DataReceived;
    public event EventHandler? Connected;
    public event EventHandler? Disconnected;
    public event EventHandler<Exception>? Error;

    public bool IsConnected => _tcpClient?.Connected ?? false;

    /// <summary>
    /// Connect to remote server
    /// </summary>
    public async Task ConnectAsync(string host, int port, CancellationToken cancellationToken = default)
    {
        _tcpClient = new TcpClient();
        await _tcpClient.ConnectAsync(host, port, cancellationToken);
        _stream = _tcpClient.GetStream();
        _cts = new CancellationTokenSource();

        // Send handshake
        var handshake = System.Text.Encoding.ASCII.GetBytes("WINDOWS\n");
        await _stream.WriteAsync(handshake, cancellationToken);

        // Wait for OK response
        var buffer = new byte[64];
        var read = await _stream.ReadAsync(buffer, cancellationToken);
        var response = System.Text.Encoding.ASCII.GetString(buffer, 0, read).Trim();
        if (response != "OK")
        {
            throw new Exception($"Server rejected connection: {response}");
        }

        Connected?.Invoke(this, EventArgs.Empty);

        // Start receive loop
        _ = Task.Run(() => ReceiveLoop(_cts.Token), _cts.Token);
    }

    /// <summary>
    /// Disconnect from server
    /// </summary>
    public void Disconnect()
    {
        _cts?.Cancel();
        _stream?.Close();
        _tcpClient?.Close();
        _stream = null;
        _tcpClient = null;

        Disconnected?.Invoke(this, EventArgs.Empty);
    }

    /// <summary>
    /// Send data to server
    /// </summary>
    public async Task SendAsync(byte[] data, CancellationToken cancellationToken = default)
    {
        if (_stream == null)
            throw new InvalidOperationException("Not connected");

        await _stream.WriteAsync(data, cancellationToken);
    }

    private async Task ReceiveLoop(CancellationToken cancellationToken)
    {
        var buffer = new byte[8192];

        try
        {
            while (!cancellationToken.IsCancellationRequested && _stream != null)
            {
                var read = await _stream.ReadAsync(buffer, cancellationToken);
                if (read == 0)
                {
                    // Server closed connection
                    break;
                }

                var data = new byte[read];
                Buffer.BlockCopy(buffer, 0, data, 0, read);
                DataReceived?.Invoke(this, data);
            }
        }
        catch (OperationCanceledException)
        {
            // Normal cancellation
        }
        catch (Exception ex)
        {
            Error?.Invoke(this, ex);
        }

        Disconnect();
    }

    public void Dispose()
    {
        if (_disposed)
            return;

        Disconnect();
        _disposed = true;
        GC.SuppressFinalize(this);
    }
}
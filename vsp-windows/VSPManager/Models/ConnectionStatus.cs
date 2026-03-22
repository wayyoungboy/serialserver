namespace VSPManager.Models;

/// <summary>
/// Connection status model
/// </summary>
public class ConnectionStatus
{
    public ConnectionState State { get; set; } = ConnectionState.Disconnected;
    public string ServerAddress { get; set; } = "";
    public string PortName { get; set; } = "";
    public DateTime? ConnectedTime { get; set; }
    public string? ErrorMessage { get; set; }
}

public enum ConnectionState
{
    Disconnected,
    Connecting,
    Connected,
    Disconnecting,
    Error
}

/// <summary>
/// Data statistics model
/// </summary>
public class DataStatistics
{
    public long BytesReceived { get; set; }
    public long BytesSent { get; set; }
    public double ReceiveRate { get; set; } // bytes per second
    public double SendRate { get; set; } // bytes per second

    public void Reset()
    {
        BytesReceived = 0;
        BytesSent = 0;
        ReceiveRate = 0;
        SendRate = 0;
    }
}
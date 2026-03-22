namespace VSPManager.Models;

/// <summary>
/// VSP configuration model
/// </summary>
public class VspConfig
{
    public string PortName { get; set; } = "VSP1";
    public string ServerHost { get; set; } = "192.168.1.100";
    public int ServerPort { get; set; } = 9000;
    public bool AutoConnect { get; set; }
    public bool AutoStart { get; set; }
    public bool MinimizeToTray { get; set; } = true;
}

/// <summary>
/// Serial port configuration
/// </summary>
public class SerialConfig
{
    public int BaudRate { get; set; } = 115200;
    public int DataBits { get; set; } = 8;
    public StopBitsOption StopBits { get; set; } = StopBitsOption.One;
    public ParityOption Parity { get; set; } = ParityOption.None;
}

public enum StopBitsOption
{
    One = 1,
    OnePointFive = 3,
    Two = 2
}

public enum ParityOption
{
    None = 0,
    Odd = 1,
    Even = 2,
    Mark = 3,
    Space = 4
}
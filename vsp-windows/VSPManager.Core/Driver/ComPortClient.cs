using System.IO.Ports;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace VSPManager.Core.Driver;

/// <summary>
/// 串口客户端 - 使用标准 SerialPort 或直接文件操作
/// </summary>
public class ComPortClient : IDisposable
{
    private static readonly string LogFile = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        "vspmanager.log");

    private void Log(string message)
    {
        var logLine = $"[{DateTime.Now:HH:mm:ss.fff}] {message}";
        try { File.AppendAllText(LogFile, logLine + "\n"); } catch { }
    }

    private SerialPort? _serialPort;
    private SafeFileHandle? _handle;
    private FileStream? _stream;
    private bool _disposed;

    /// <summary>
    /// 检查串口是否已打开
    /// </summary>
    public bool IsOpen => _serialPort?.IsOpen == true || (_handle != null && !_handle.IsInvalid);

    /// <summary>
    /// 打开串口 (使用 SerialPort 类)
    /// </summary>
    public bool Open(string portName, int baudRate = 115200, int dataBits = 8, StopBits stopBits = StopBits.One, Parity parity = Parity.None)
    {
        if (IsOpen)
            return true;

        try
        {
            _serialPort = new SerialPort(portName, baudRate, parity, dataBits, stopBits)
            {
                ReadTimeout = 500,
                WriteTimeout = 500,
                ReadBufferSize = 8192,
                WriteBufferSize = 8192
            };

            _serialPort.Open();
            Log($"SerialPort opened: {portName} @ {baudRate}");
            return true;
        }
        catch (Exception ex)
        {
            Log($"SerialPort open failed: {ex.Message}");
            _serialPort?.Dispose();
            _serialPort = null;
            return false;
        }
    }

    /// <summary>
    /// 打开串口 (使用 CreateFile，支持 CNCA 等特殊端口)
    /// </summary>
    public bool OpenDirect(string portName, int baudRate = 115200)
    {
        if (IsOpen)
            return true;

        // 首先尝试使用 SerialPort（更可靠）
        try
        {
            _serialPort = new SerialPort(portName, baudRate)
            {
                ReadTimeout = 1000,  // 1秒超时
                WriteTimeout = 1000,
                ReadBufferSize = 8192,
                WriteBufferSize = 8192
            };

            _serialPort.Open();
            Log($"SerialPort opened directly: {portName} @ {baudRate}");
            return true;
        }
        catch (Exception ex)
        {
            Log($"SerialPort open failed: {ex.Message}, trying CreateFile...");
            _serialPort?.Dispose();
            _serialPort = null;
        }

        // 如果 SerialPort 失败，尝试 CreateFile
        try
        {
            var deviceName = portName.StartsWith("\\\\?\\") || portName.StartsWith("\\\\.\\")
                ? portName
                : $@"\\.\{portName}";

            Log($"Opening port with CreateFile: {deviceName}");

            _handle = CreateFile(
                deviceName,
                0xC0000000, // GENERIC_READ | GENERIC_WRITE
                3,          // FILE_SHARE_READ | FILE_SHARE_WRITE
                IntPtr.Zero,
                3,          // OPEN_EXISTING
                0x40000000, // FILE_FLAG_OVERLAPPED for async
                IntPtr.Zero);

            if (_handle.IsInvalid)
            {
                var error = Marshal.GetLastWin32Error();
                Log($"CreateFile failed: error={error}");
                _handle.Dispose();
                _handle = null;
                return false;
            }

            _stream = new FileStream(_handle, FileAccess.ReadWrite, 4096, true);
            Log($"Port opened with CreateFile: {portName}");
            return true;
        }
        catch (Exception ex)
        {
            Log($"OpenDirect failed: {ex.Message}");
            _stream?.Dispose();
            _handle?.Dispose();
            _stream = null;
            _handle = null;
            return false;
        }
    }

    /// <summary>
    /// 读取数据
    /// </summary>
    public int Read(byte[] buffer, int offset, int count)
    {
        if (_serialPort?.IsOpen == true)
        {
            return _serialPort.BaseStream.Read(buffer, offset, count);
        }

        if (_stream != null)
        {
            return _stream.Read(buffer, offset, count);
        }

        throw new InvalidOperationException("Port not opened");
    }

    /// <summary>
    /// 写入数据
    /// </summary>
    public void Write(byte[] data, int offset, int count)
    {
        if (_serialPort?.IsOpen == true)
        {
            _serialPort.BaseStream.Write(data, offset, count);
            return;
        }

        if (_stream != null)
        {
            _stream.Write(data, offset, count);
            _stream.Flush();
            return;
        }

        throw new InvalidOperationException("Port not opened");
    }

    /// <summary>
    /// 异步读取
    /// </summary>
    public async Task<int> ReadAsync(byte[] buffer, CancellationToken cancellationToken = default)
    {
        if (_serialPort?.IsOpen == true)
        {
            return await _serialPort.BaseStream.ReadAsync(buffer, cancellationToken);
        }

        if (_stream != null)
        {
            return await _stream.ReadAsync(buffer, cancellationToken);
        }

        throw new InvalidOperationException("Port not opened");
    }

    /// <summary>
    /// 异步写入
    /// </summary>
    public async Task WriteAsync(byte[] data, int offset, int count, CancellationToken cancellationToken = default)
    {
        if (_serialPort?.IsOpen == true)
        {
            await _serialPort.BaseStream.WriteAsync(data, offset, count, cancellationToken);
            return;
        }

        if (_stream != null)
        {
            await _stream.WriteAsync(data, offset, count, cancellationToken);
            await _stream.FlushAsync(cancellationToken);
            return;
        }

        throw new InvalidOperationException("Port not opened");
    }

    /// <summary>
    /// 关闭串口
    /// </summary>
    public void Close()
    {
        try
        {
            _stream?.Dispose();
            _handle?.Dispose();
            _serialPort?.Close();
            _serialPort?.Dispose();
        }
        catch { }

        _stream = null;
        _handle = null;
        _serialPort = null;
        Log("Port closed");
    }

    public void Dispose()
    {
        if (_disposed) return;
        Close();
        _disposed = true;
        GC.SuppressFinalize(this);
    }

    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    private static extern SafeFileHandle CreateFile(
        string lpFileName,
        uint dwDesiredAccess,
        uint dwShareMode,
        IntPtr lpSecurityAttributes,
        uint dwCreationDisposition,
        uint dwFlagsAndAttributes,
        IntPtr hTemplateFile);
}
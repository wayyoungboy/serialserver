using System.Diagnostics;
using System.IO.Ports;
using System.Management;
using System.Text;

namespace VSPManager.Core.Driver;

/// <summary>
/// com0com 虚拟串口管理器
/// 用于创建虚拟串口对：一个可见的 COM 端口 + 一个隐藏的 CNCA 端口
/// </summary>
public class Com0ComManager : IDisposable
{
    private static readonly string LogFile = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        "vspmanager.log");

    private void Log(string message)
    {
        var logLine = $"[{DateTime.Now:HH:mm:ss.fff}] {message}";
        try { File.AppendAllText(LogFile, logLine + "\n"); } catch { }
    }

    // com0com 安装路径
    private readonly string _com0comPath;
    private readonly string _setupcPath;

    // 已创建的端口对
    private string? _visiblePort;   // 用户可见的端口 (如 COM5)
    private string? _hiddenPort;    // 隐藏端口 (如 CNCA0)

    public string? VisiblePort => _visiblePort;
    public string? HiddenPort => _hiddenPort;

    public Com0ComManager()
    {
        // 尝试多个路径
        var possiblePaths = new List<string>
        {
            // 应用程序目录下的 com0com
            Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "com0com"),
            // Program Files (x86)
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86), "com0com"),
            // Program Files
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "com0com")
        };

        foreach (var path in possiblePaths)
        {
            var setupcPath = Path.Combine(path, "setupc.exe");
            if (File.Exists(setupcPath))
            {
                _com0comPath = path;
                _setupcPath = setupcPath;
                Log($"com0com found at: {path}");
                return;
            }
        }

        // 默认使用应用程序目录
        _com0comPath = possiblePaths[0];
        _setupcPath = Path.Combine(_com0comPath, "setupc.exe");
        Log($"com0com not found, default path: {_com0comPath}");
    }

    /// <summary>
    /// 检查 com0com 是否已安装
    /// </summary>
    public bool IsInstalled()
    {
        var installed = File.Exists(_setupcPath);
        Log($"com0com installed check: {installed} (path: {_setupcPath})");
        return installed;
    }

    /// <summary>
    /// 获取已存在的 com0com 端口对
    /// </summary>
    public List<(string PortA, string PortB)> GetExistingPairs()
    {
        var pairs = new List<(string, string)>();

        try
        {
            // 使用 WMI 查询 com0com 设备
            using var searcher = new ManagementObjectSearcher(
                "SELECT * FROM Win32_PnPEntity WHERE Name LIKE '%com0com%' OR Name LIKE '%CNC%'");

            var ports = new List<string>();
            foreach (ManagementObject obj in searcher.Get())
            {
                var name = obj["Name"]?.ToString();
                if (!string.IsNullOrEmpty(name))
                {
                    // 提取端口号
                    var match = System.Text.RegularExpressions.Regex.Match(name, @"(COM\d+|CNC[AB]\d+)");
                    if (match.Success)
                    {
                        ports.Add(match.Groups[1].Value);
                    }
                }
            }

            // 配对
            for (int i = 0; i < ports.Count; i += 2)
            {
                if (i + 1 < ports.Count)
                {
                    pairs.Add((ports[i], ports[i + 1]));
                }
            }
        }
        catch (Exception ex)
        {
            Log($"GetExistingPairs error: {ex.Message}");
        }

        return pairs;
    }

    /// <summary>
    /// 创建虚拟串口对，系统自动分配可见端口号
    /// </summary>
    public bool CreatePortPair()
    {
        if (!IsInstalled())
        {
            Log("com0com not installed");
            return false;
        }

        try
        {
            // 使用 setupc.exe 创建端口对
            // PortName=COM# 让系统自动分配 COM 端口号并显示在设备管理器中
            // PortName=- 表示隐藏端口（不在设备管理器显示）
            var args = "install PortName=COM# PortName=-";
            Log($"Running: setupc.exe {args}");

            var psi = new ProcessStartInfo
            {
                FileName = _setupcPath,
                Arguments = args,
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true,
                WorkingDirectory = _com0comPath
            };

            using var process = Process.Start(psi);
            if (process == null)
            {
                Log("Failed to start setupc.exe");
                return false;
            }

            var output = process.StandardOutput.ReadToEnd();
            var error = process.StandardError.ReadToEnd();
            process.WaitForExit(30000);

            if (process.ExitCode == 0)
            {
                // 从输出中解析实际端口名
                // 输出格式: CNCA0 PortName=COM#,RealPortName=COM5
                var lines = output.Split('\n');
                foreach (var line in lines)
                {
                    if (line.Contains("RealPortName="))
                    {
                        var match = System.Text.RegularExpressions.Regex.Match(line, @"RealPortName=(COM\d+)");
                        if (match.Success)
                        {
                            _visiblePort = match.Groups[1].Value;
                            // 获取对应的隐藏端口名 (CNCA0 或类似的)
                            var portMatch = System.Text.RegularExpressions.Regex.Match(line, @"^(CNCA\d+|CNCB\d+)");
                            if (portMatch.Success)
                            {
                                _hiddenPort = portMatch.Groups[1].Value;
                            }
                            Log($"Port pair created: {_visiblePort} (visible) <-> {_hiddenPort} (hidden)");
                            return true;
                        }
                    }
                }

                // 如果无法解析，使用 list 命令获取
                var listOutput = RunSetupcCommandWithOutput("list");
                Log($"List output: {listOutput}");
                var listMatch = System.Text.RegularExpressions.Regex.Match(listOutput, @"RealPortName=(COM\d+)");
                if (listMatch.Success)
                {
                    _visiblePort = listMatch.Groups[1].Value;
                    Log($"Got visible port from list: {_visiblePort}");
                    return true;
                }

                Log($"Failed to parse port name from output: {output}");
                return false;
            }
            else
            {
                Log($"setupc.exe failed: exit={process.ExitCode}, error={error}");
                return false;
            }
        }
        catch (Exception ex)
        {
            Log($"CreatePortPair error: {ex.Message}");
            return false;
        }
    }

    /// <summary>
    /// 删除虚拟串口对
    /// </summary>
    public bool RemovePortPair(string portName)
    {
        if (!IsInstalled())
            return false;

        try
        {
            var args = $"remove {portName}";
            var psi = new ProcessStartInfo
            {
                FileName = _setupcPath,
                Arguments = args,
                UseShellExecute = false,
                RedirectStandardOutput = true,
                CreateNoWindow = true,
                WorkingDirectory = _com0comPath
            };

            using var process = Process.Start(psi);
            process?.WaitForExit(10000);

            Log($"Port pair removed: {portName}");
            return true;
        }
        catch (Exception ex)
        {
            Log($"RemovePortPair error: {ex.Message}");
            return false;
        }
    }

    /// <summary>
    /// 查找可用的 COM 端口号
    /// </summary>
    public string FindAvailableComPort(int startFrom = 5)
    {
        var existingPorts = SerialPort.GetPortNames();
        for (int i = startFrom; i < 100; i++)
        {
            var portName = $"COM{i}";
            if (!existingPorts.Contains(portName))
            {
                return portName;
            }
        }
        return $"COM{startFrom}";
    }

    /// <summary>
    /// 检查端口是否存在
    /// </summary>
    public bool PortExists(string portName)
    {
        return SerialPort.GetPortNames().Contains(portName);
    }

    /// <summary>
    /// 自动创建虚拟串口对（启动时创建，关闭时删除）
    /// </summary>
    public (string VisiblePort, string HiddenPort)? EnsurePortPair(string? preferredVisiblePort = null)
    {
        // 直接创建新的端口对，系统自动分配端口号
        if (CreatePortPair())
        {
            return (_visiblePort!, _hiddenPort!);
        }

        return null;
    }

    /// <summary>
    /// 删除当前端口对
    /// </summary>
    public void RemoveCurrentPortPair()
    {
        if (!string.IsNullOrEmpty(_visiblePort))
        {
            RemovePortPair(_visiblePort);
            _visiblePort = null;
            _hiddenPort = null;
        }
    }

    /// <summary>
    /// 执行 setupc 命令
    /// </summary>
    private bool RunSetupcCommand(string args)
    {
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = _setupcPath,
                Arguments = args,
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true,
                WorkingDirectory = _com0comPath
            };

            using var process = Process.Start(psi);
            if (process == null) return false;

            process.WaitForExit(10000);
            return process.ExitCode == 0;
        }
        catch (Exception ex)
        {
            Log($"RunSetupcCommand error: {ex.Message}");
            return false;
        }
    }

    /// <summary>
    /// 执行 setupc 命令并返回输出
    /// </summary>
    private string RunSetupcCommandWithOutput(string args)
    {
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = _setupcPath,
                Arguments = args,
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true,
                WorkingDirectory = _com0comPath
            };

            using var process = Process.Start(psi);
            if (process == null) return "";

            var output = process.StandardOutput.ReadToEnd();
            process.WaitForExit(10000);
            return output;
        }
        catch (Exception ex)
        {
            Log($"RunSetupcCommandWithOutput error: {ex.Message}");
            return "";
        }
    }

    /// <summary>
    /// 查找可用的 CNC 端口名
    /// </summary>
    public string FindAvailableCncPort()
    {
        var existingPorts = SerialPort.GetPortNames();
        for (int i = 0; i < 100; i++)
        {
            var portName = $"CNCA{i}";
            if (!existingPorts.Contains(portName))
            {
                return portName;
            }
            portName = $"CNCB{i}";
            if (!existingPorts.Contains(portName))
            {
                return portName;
            }
        }
        return "CNCA0";
    }

    public void Dispose()
    {
        // 关闭时删除端口对
        RemoveCurrentPortPair();
    }
}
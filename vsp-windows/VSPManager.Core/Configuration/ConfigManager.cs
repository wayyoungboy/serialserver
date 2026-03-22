using System.Text.Json;

namespace VSPManager.Core.Configuration;

/// <summary>
/// Configuration manager for VSP settings
/// </summary>
public class ConfigManager
{
    private static readonly string ConfigPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "VSPManager",
        "config.json");

    public VspConfig Config { get; private set; }

    public ConfigManager()
    {
        Config = Load();
    }

    /// <summary>
    /// Load configuration from file
    /// </summary>
    public VspConfig Load()
    {
        if (!File.Exists(ConfigPath))
        {
            return new VspConfig();
        }

        try
        {
            var json = File.ReadAllText(ConfigPath);
            return JsonSerializer.Deserialize<VspConfig>(json) ?? new VspConfig();
        }
        catch
        {
            return new VspConfig();
        }
    }

    /// <summary>
    /// Save configuration to file
    /// </summary>
    public void Save()
    {
        var directory = Path.GetDirectoryName(ConfigPath);
        if (!string.IsNullOrEmpty(directory) && !Directory.Exists(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var json = JsonSerializer.Serialize(Config, new JsonSerializerOptions
        {
            WriteIndented = true
        });

        File.WriteAllText(ConfigPath, json);
    }

    /// <summary>
    /// Reset configuration to defaults
    /// </summary>
    public void Reset()
    {
        Config = new VspConfig();
        Save();
    }
}

/// <summary>
/// VSP configuration model
/// </summary>
public class VspConfig
{
    public string PortName { get; set; } = "VSP1";
    public string ServerHost { get; set; } = "localhost";
    public int ServerPort { get; set; } = 9000;
    public bool AutoConnect { get; set; }
    public bool AutoStart { get; set; }
    public bool MinimizeToTray { get; set; } = true;

    // User session
    public string? Token { get; set; }
    public string? Username { get; set; }

    // Device configuration
    public string DeviceKey { get; set; } = "";
    public string DeviceSerialPort { get; set; } = "COM3";
    public int BaudRate { get; set; } = 115200;
    public int DataBits { get; set; } = 8;
    public string StopBits { get; set; } = "1";
    public string Parity { get; set; } = "None";
}
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace VSPManager.Native;

/// <summary>
/// Windows Kernel32 API P/Invoke declarations
/// </summary>
public static class Kernel32
{
    // Access rights
    public const uint GENERIC_READ = 0x80000000;
    public const uint GENERIC_WRITE = 0x40000000;
    public const uint GENERIC_READ_WRITE = GENERIC_READ | GENERIC_WRITE;

    // Creation disposition
    public const uint OPEN_EXISTING = 3;

    // IOCTL codes (calculated from CTL_CODE macro)
    // CTL_CODE(FILE_DEVICE_SERIAL_PORT, 0x800, METHOD_BUFFERED, FILE_ANY_ACCESS)
    // FILE_DEVICE_SERIAL_PORT = 0x1B
    public const uint FILE_DEVICE_SERIAL_PORT = 0x1B;
    public const uint IOCTL_VSP_REGISTER_NET_CLIENT = 0x1B0800;
    public const uint IOCTL_VSP_UNREGISTER_NET_CLIENT = 0x1B0804;

    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    public static extern SafeFileHandle CreateFile(
        string lpFileName,
        uint dwDesiredAccess,
        uint dwShareMode,
        IntPtr lpSecurityAttributes,
        uint dwCreationDisposition,
        uint dwFlagsAndAttributes,
        IntPtr hTemplateFile);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool DeviceIoControl(
        SafeFileHandle hDevice,
        uint dwIoControlCode,
        byte[]? lpInBuffer,
        uint nInBufferSize,
        byte[]? lpOutBuffer,
        uint nOutBufferSize,
        out uint lpBytesReturned,
        IntPtr lpOverlapped);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool DeviceIoControl(
        SafeFileHandle hDevice,
        uint dwIoControlCode,
        IntPtr lpInBuffer,
        uint nInBufferSize,
        IntPtr lpOutBuffer,
        uint nOutBufferSize,
        out uint lpBytesReturned,
        IntPtr lpOverlapped);

    [DllImport("kernel32.dll")]
    public static extern uint GetLastError();
}
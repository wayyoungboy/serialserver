using System.Windows;

namespace VSPManager;

public partial class App : Application
{
    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        // Set up MVVM and dependency injection if needed
        // For simplicity, we create ViewModel directly in MainWindow
    }
}
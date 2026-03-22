using System.Windows;
using System.Windows.Controls;

namespace VSPManager;

public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
        var vm = new ViewModels.MainViewModel();
        DataContext = vm;

        // Password binding workaround
        PasswordBox.PasswordChanged += (s, e) =>
        {
            vm.Password = PasswordBox.Password;
        };

        // Handle page switching
        vm.PropertyChanged += (s, e) =>
        {
            if (e.PropertyName == nameof(ViewModels.MainViewModel.IsLoggedIn))
            {
                Dispatcher.Invoke(() =>
                {
                    LoginGrid.Visibility = vm.IsLoggedIn ? Visibility.Collapsed : Visibility.Visible;
                    MainGrid.Visibility = vm.IsLoggedIn ? Visibility.Visible : Visibility.Collapsed;
                });
            }
        };
    }

    private void OnExitClick(object sender, RoutedEventArgs e)
    {
        Application.Current.Shutdown();
    }
}
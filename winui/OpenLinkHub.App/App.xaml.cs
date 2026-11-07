using Microsoft.UI.Xaml;
using OpenLinkHub.App.Services;

namespace OpenLinkHub.App;

public partial class App : Application
{
    public static OlhClient Api { get; } = new();
    public static MainWindow? Window { get; private set; }

    public App()
    {
        InitializeComponent();
        UnhandledException += (_, e) =>
        {
            Log(e.Exception);
            e.Handled = true;
            Window?.ShowToast(e.Exception.Message, false);
        };
    }

    public static string LogPath { get; } = System.IO.Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "OpenLinkHub.App", "app.log");

    public static void Log(Exception ex)
    {
        try
        {
            Directory.CreateDirectory(System.IO.Path.GetDirectoryName(LogPath)!);
            File.AppendAllText(LogPath, $"{DateTime.Now:O} {ex}{Environment.NewLine}");
        }
        catch
        {
        }
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        Window = new MainWindow();
        Window.Activate();
    }
}

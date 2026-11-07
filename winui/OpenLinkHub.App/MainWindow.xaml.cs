using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Animation;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Views;

namespace OpenLinkHub.App;

public sealed partial class MainWindow : Window
{
    private readonly List<NavigationViewItem> _deviceItems = new();
    private bool _suppressSelection;

    public MainWindow()
    {
        InitializeComponent();
        ExtendsContentIntoTitleBar = true;
        SetTitleBar(AppTitleBar);
        AppWindow.SetIcon("Assets/app.ico");
        SizeToDisplay();
        AppWindow.TitleBar.PreferredHeightOption = Microsoft.UI.Windowing.TitleBarHeightOption.Tall;

        ContentFrame.Navigated += ContentFrame_Navigated;
        Nav.SelectedItem = Nav.MenuItems[0];
        _ = LoadDevicesAsync();
    }

    private void SizeToDisplay()
    {
        var area = Microsoft.UI.Windowing.DisplayArea.GetFromWindowId(AppWindow.Id, Microsoft.UI.Windowing.DisplayAreaFallback.Primary).WorkArea;
        var width = Math.Min(area.Width, Math.Max(1100 * area.Width / 1920, (int)(area.Width * 0.7)));
        var height = Math.Min(area.Height, (int)(area.Height * 0.8));
        AppWindow.MoveAndResize(new Windows.Graphics.RectInt32(
            area.X + (area.Width - width) / 2, area.Y + (area.Height - height) / 2, width, height));
    }

    public void OpenDevice(DeviceSummary device)
    {
        var item = _deviceItems.FirstOrDefault(i => (i.Tag as DeviceSummary)?.Serial == device.Serial);
        if (item != null)
        {
            Nav.SelectedItem = item;
        }
        else
        {
            Navigate(typeof(DevicePage), device);
        }
    }

    private readonly DispatcherTimer _toastTimer = new() { Interval = TimeSpan.FromSeconds(4) };

    public void ShowToast(string message, bool success = true)
    {
        Toast.Severity = success ? InfoBarSeverity.Success : InfoBarSeverity.Error;
        Toast.Message = message;
        Toast.IsOpen = true;
        _toastTimer.Stop();
        _toastTimer.Tick -= HideToast;
        _toastTimer.Tick += HideToast;
        _toastTimer.Start();
    }

    private void HideToast(object? sender, object e)
    {
        _toastTimer.Stop();
        Toast.IsOpen = false;
    }

    public void SetConnection(bool online, string text)
    {
        ConnectionText.Text = text;
        ConnectionDot.Fill = (Brush)Application.Current.Resources[online
            ? "SystemFillColorSuccessBrush"
            : "SystemFillColorCriticalBrush"];
    }

    private async Task LoadDevicesAsync()
    {
        try
        {
            var devices = await App.Api.GetDevicesAsync();
            SetConnection(true, $"Connected · {devices.Count} devices");

            foreach (var old in _deviceItems)
            {
                Nav.MenuItems.Remove(old);
            }
            _deviceItems.Clear();

            foreach (var device in devices.Where(d => !d.Hidden).OrderBy(d => d.Category).ThenBy(d => d.Product))
            {
                var item = new NavigationViewItem
                {
                    Content = device.DisplayName,
                    Tag = device,
                    Icon = new FontIcon { Glyph = device.Glyph },
                };
                ToolTipService.SetToolTip(item, device.Product);
                _deviceItems.Add(item);
                Nav.MenuItems.Add(item);
            }
        }
        catch (Exception ex)
        {
            SetConnection(false, "OpenLinkHub not reachable");
            System.Diagnostics.Debug.WriteLine(ex);
        }
    }

    private void Nav_SelectionChanged(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (_suppressSelection)
        {
            return;
        }

        if (args.IsSettingsSelected)
        {
            Navigate(typeof(SettingsPage), null);
            return;
        }

        if (args.SelectedItem is not NavigationViewItem item)
        {
            return;
        }

        switch (item.Tag)
        {
            case "home":
                Navigate(typeof(HomePage), null);
                break;
            case "dashboard":
                Navigate(typeof(DashboardPage), null);
                break;
            case "profiles":
                Navigate(typeof(ProfilesPage), null);
                break;
            case "curves":
                Navigate(typeof(FanCurvesPage), null);
                break;
            case "macros":
                Navigate(typeof(MacrosPage), null);
                break;
            case DeviceSummary device:
                Navigate(typeof(DevicePage), device);
                break;
        }
    }

    private void Navigate(Type page, object? parameter)
    {
        if (ContentFrame.CurrentSourcePageType == page && Equals(ContentFrame.Tag, parameter))
        {
            return;
        }
        ContentFrame.Tag = parameter;
        ContentFrame.Navigate(page, parameter, new EntranceNavigationTransitionInfo());
    }

    private void Nav_BackRequested(NavigationView sender, NavigationViewBackRequestedEventArgs args)
    {
        if (ContentFrame.CanGoBack)
        {
            ContentFrame.GoBack();
        }
    }

    private void ContentFrame_Navigated(object sender, Microsoft.UI.Xaml.Navigation.NavigationEventArgs e)
    {
        _suppressSelection = true;
        try
        {
            ContentFrame.Tag = e.Parameter;
            Nav.SelectedItem = e.SourcePageType switch
            {
                var t when t == typeof(SettingsPage) => Nav.SettingsItem,
                var t when t == typeof(HomePage) => Nav.MenuItems[0],
                var t when t == typeof(DashboardPage) => Nav.MenuItems[1],
                var t when t == typeof(ProfilesPage) => Nav.MenuItems[2],
                var t when t == typeof(FanCurvesPage) => Nav.MenuItems[3],
                var t when t == typeof(MacrosPage) => Nav.MenuItems[4],
                var t when t == typeof(DevicePage) && e.Parameter is DeviceSummary d =>
                    _deviceItems.FirstOrDefault(i => (i.Tag as DeviceSummary)?.Serial == d.Serial),
                _ => Nav.SelectedItem,
            };
        }
        finally
        {
            _suppressSelection = false;
        }
    }
}

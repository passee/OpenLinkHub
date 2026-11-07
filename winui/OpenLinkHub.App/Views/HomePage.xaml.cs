using System.Collections.ObjectModel;
using System.Text.Json.Nodes;
using CommunityToolkit.Mvvm.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services;

namespace OpenLinkHub.App.Views;

public sealed partial class DeviceTile : ObservableObject
{
    public required DeviceSummary Device { get; init; }

    private string _status = "";
    private UIElement? _preview;

    public UIElement? Preview
    {
        get => _preview;
        set
        {
            if (SetProperty(ref _preview, value))
            {
                OnPropertyChanged(nameof(IconVisibility));
            }
        }
    }

    public Visibility IconVisibility => _preview == null ? Visibility.Visible : Visibility.Collapsed;

    public string Status
    {
        get => _status;
        set => SetProperty(ref _status, value);
    }

    public static string DescribeStatus(JsonObject? detail)
    {
        if (detail == null)
        {
            return "";
        }

        if (detail["Connected"] is JsonNode connected && !connected.GetValue<bool>())
        {
            return "Disconnected";
        }

        if (detail["BatteryLevel"] is JsonNode battery && battery.GetValue<int>() > 0)
        {
            return $"{battery.GetValue<int>()} % battery";
        }

        if (detail["devices"] is JsonObject channels)
        {
            var temp = channels
                .Select(c => c.Value?["temperatureString"]?.GetValue<string>())
                .FirstOrDefault(t => !string.IsNullOrEmpty(t));
            if (temp != null)
            {
                return temp;
            }

            var fans = channels.Count(c => c.Value?["HasSpeed"]?.GetValue<bool>() == true);
            if (fans > 0)
            {
                return $"{fans} fans";
            }
        }

        return "";
    }
}

public sealed class ProfileTile
{
    public required ProfileInfo Info { get; init; }
    public string Name => Info.Name;
    public string DisplayName => Info.DisplayName;
    public string Subtitle => Info.ActiveCount > 0 ? "Active" : $"{Info.DeviceCount} devices";
    public Visibility ActiveVisibility => Info.ActiveCount > 0 ? Visibility.Visible : Visibility.Collapsed;
    public Style ButtonStyle => (Style)Application.Current.Resources[Info.ActiveCount > 0 ? "AccentButtonStyle" : "DefaultButtonStyle"];
}

public sealed partial class HomePage : Page
{
    private readonly ObservableCollection<DeviceTile> _tiles = new();
    private IReadOnlyDictionary<string, JsonObject> _details = new Dictionary<string, JsonObject>();
    private readonly DispatcherTimer _refresh = new() { Interval = TimeSpan.FromSeconds(5) };

    public HomePage()
    {
        InitializeComponent();
        DevicesRepeater.ItemsSource = _tiles;
        _refresh.Tick += async (_, _) => await RefreshStatusAsync();
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        await LoadAsync();
        _refresh.Start();
    }

    protected override void OnNavigatedFrom(NavigationEventArgs e) => _refresh.Stop();

    private async Task LoadAsync()
    {
        LoadingRing.IsActive = true;
        ErrorBar.IsOpen = false;
        try
        {
            var devices = await App.Api.GetDevicesAsync();
            var visible = devices.Where(d => !d.Hidden).ToList();

            _tiles.Clear();
            foreach (var device in visible.Where(d => d.Category != DeviceCategory.Receiver)
                         .OrderBy(d => d.Category).ThenBy(d => d.Product))
            {
                _tiles.Add(new DeviceTile { Device = device });
            }

            var receivers = visible.Where(d => d.Category == DeviceCategory.Receiver).ToList();
            ReceiversRepeater.ItemsSource = receivers;
            ReceiversExpander.Visibility = receivers.Count > 0 ? Visibility.Visible : Visibility.Collapsed;

            var (profiles, details) = await ProfileService.LoadAsync(visible);
            _details = details;
            foreach (var tile in _tiles)
            {
                tile.Preview = await Controls.DevicePreviews.CreateAsync(tile.Device, details.GetValueOrDefault(tile.Device.Serial));
            }
            ProfilesRepeater.ItemsSource = profiles.Select(p => new ProfileTile { Info = p }).ToList();
            ApplyStatus();
        }
        catch
        {
            ErrorBar.IsOpen = true;
        }
        finally
        {
            LoadingRing.IsActive = false;
            LoadingRing.Visibility = Visibility.Collapsed;
        }
    }

    private async Task RefreshStatusAsync()
    {
        var updated = new Dictionary<string, JsonObject>(_details);
        await Task.WhenAll(_tiles.Select(async tile =>
        {
            try
            {
                var detail = await App.Api.GetDeviceAsync(tile.Device.Serial);
                if (detail != null)
                {
                    lock (updated)
                    {
                        updated[tile.Device.Serial] = detail;
                    }
                }
            }
            catch
            {
            }
        }));
        _details = updated;
        ApplyStatus();
    }

    private void ApplyStatus()
    {
        foreach (var tile in _tiles)
        {
            tile.Status = DeviceTile.DescribeStatus(_details.GetValueOrDefault(tile.Device.Serial));
        }
    }

    private async void Profile_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: string name })
        {
            return;
        }

        var (ok, failed) = await ProfileService.ActivateAsync(name, _details);
        App.Window?.ShowToast(failed == 0
            ? $"Profile \"{(name == "default" ? "Default" : name)}\" applied to {ok} devices"
            : $"Profile applied to {ok} devices, {failed} failed", failed == 0);
        await LoadAsync();
    }

    private void Device_Click(object sender, RoutedEventArgs e)
    {
        if (sender is Button { Tag: DeviceSummary device })
        {
            App.Window?.OpenDevice(device);
        }
    }

    private void ManageProfiles_Click(object sender, RoutedEventArgs e) => Frame.Navigate(typeof(ProfilesPage));

    private async void Retry_Click(object sender, RoutedEventArgs e) => await LoadAsync();
}

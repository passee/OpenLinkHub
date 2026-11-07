using System.Collections.ObjectModel;
using System.Globalization;
using System.Text.Json.Nodes;
using CommunityToolkit.Mvvm.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Views;

public sealed partial class SensorTile : ObservableObject
{
    public required string Key { get; init; }
    public required string Title { get; init; }
    public required string Glyph { get; init; }
    public string Source { get; init; } = "";

    private string _value = "–";

    public string Value
    {
        get => _value;
        set => SetProperty(ref _value, value);
    }
}

public sealed partial class DashboardPage : Page
{
    private readonly ObservableCollection<SensorTile> _tiles = new();
    private readonly DispatcherTimer _refresh = new() { Interval = TimeSpan.FromSeconds(3) };
    private IReadOnlyList<DeviceSummary> _devices = Array.Empty<DeviceSummary>();

    public DashboardPage()
    {
        InitializeComponent();
        Tiles.ItemsSource = _tiles;
        _refresh.Tick += async (_, _) => await RefreshAsync();
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        try
        {
            _devices = (await App.Api.GetDevicesAsync()).Where(d => !d.Hidden && d.Category != DeviceCategory.Receiver).ToList();
        }
        catch
        {
        }
        await RefreshAsync();
        _refresh.Start();
    }

    protected override void OnNavigatedFrom(NavigationEventArgs e) => _refresh.Stop();

    private async Task RefreshAsync()
    {
        var cpu = Safe(App.Api.GetCpuTempAsync());
        var gpu = Safe(App.Api.GetGpuTempAsync());
        var details = _devices.Select(async d => (d, detail: await Safe(App.Api.GetDeviceAsync(d.Serial)))).ToList();

        Set("cpu", "CPU temperature", "\uE950", "Processor", Temp(await cpu));
        Set("gpu", "GPU temperature", "\uE7F4", "Graphics card", Temp(await gpu));

        foreach (var (device, detail) in await Task.WhenAll(details))
        {
            if (detail == null)
            {
                continue;
            }

            if (detail["BatteryLevel"] is JsonNode battery && battery.GetValue<int>() > 0)
            {
                Set($"{device.Serial}/battery", "Battery", "\uE83F", device.DisplayName, $"{battery.GetValue<int>()} %");
            }

            if (detail["devices"] is not JsonObject channels)
            {
                continue;
            }

            foreach (var (id, node) in channels)
            {
                if (node is not JsonObject channel)
                {
                    continue;
                }
                var name = channel["name"]?.GetValue<string>() ?? $"Channel {id}";
                if (channel["HasTemps"]?.GetValue<bool>() == true && channel["temperature"]?.GetValue<double>() is > 0 and var t)
                {
                    Set($"{device.Serial}/{id}/temp", $"{name}", "\uE9CA", device.DisplayName, Temp(t));
                }
                if (channel["HasSpeed"]?.GetValue<bool>() == true)
                {
                    Set($"{device.Serial}/{id}/rpm", name, "\uE9CA", device.DisplayName, $"{channel["rpm"]?.GetValue<int>() ?? 0} RPM");
                }
            }
        }
    }

    private void Set(string key, string title, string glyph, string source, string value)
    {
        var tile = _tiles.FirstOrDefault(t => t.Key == key);
        if (tile == null)
        {
            tile = new SensorTile { Key = key, Title = title, Glyph = glyph, Source = source };
            _tiles.Add(tile);
        }
        tile.Value = value;
    }

    private static string Temp(double? value) =>
        value is > 0 ? $"{value.Value.ToString("0.#", CultureInfo.CurrentCulture)} °C" : "–";

    private static async Task<T?> Safe<T>(Task<T> task)
    {
        try
        {
            return await task;
        }
        catch
        {
            return default;
        }
    }
}

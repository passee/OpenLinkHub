using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using OpenLinkHub.App.Models;
using Windows.Foundation;

namespace OpenLinkHub.App.Views;

public sealed partial class FanCurvesPage : Page
{
    private static readonly string[] BuiltIn = { "Quiet", "Normal", "Performance" };

    private static readonly Dictionary<int, string> SensorNames = new()
    {
        [0] = "CPU", [1] = "GPU", [2] = "Liquid", [3] = "Storage", [4] = "Probe", [5] = "CPU + GPU",
        [6] = "HwMon", [7] = "Executable", [8] = "GPU", [9] = "Device sensor", [10] = "PSU", [11] = "GPUs",
    };

    private JsonObject _profiles = new();
    private string? _current;
    private List<Point> _original = new();
    private List<Point> _beyondRange = new();
    private bool _loading;
    private readonly DispatcherTimer _marker = new() { Interval = TimeSpan.FromSeconds(3) };

    public FanCurvesPage()
    {
        InitializeComponent();
        Editor.PointsChanged += async (_, _) => await SaveAsync(Editor.Points);
        _marker.Tick += async (_, _) => await UpdateMarkerAsync();
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        await LoadAsync(e.Parameter as string);
        _marker.Start();
    }

    protected override void OnNavigatedFrom(NavigationEventArgs e) => _marker.Stop();

    private int Target => int.Parse(TargetBar.SelectedItem?.Tag as string ?? "1");
    private JsonObject? Profile => _current == null ? null : _profiles[_current] as JsonObject;
    private int Sensor => Profile?["sensor"]?.GetValue<int>() ?? 0;
    private bool IsReadOnly => _current != null && (BuiltIn.Contains(_current) || Profile?["linear"]?.GetValue<bool>() == true);

    private async Task LoadAsync(string? select = null)
    {
        _loading = true;
        try
        {
            _profiles = (await App.Api.GetAsync("api/temperatures/"))?["data"] as JsonObject ?? new JsonObject();
            var names = _profiles
                .Where(p => p.Value?["Hidden"]?.GetValue<bool>() != true)
                .Select(p => p.Key)
                .OrderBy(n => Array.IndexOf(BuiltIn, n) is var i && i >= 0 ? i : 99)
                .ThenBy(n => n, StringComparer.OrdinalIgnoreCase)
                .ToList();
            CurveCombo.ItemsSource = names;
            CurveCombo.SelectedItem = names.FirstOrDefault(n => n == (select ?? _current)) ?? names.FirstOrDefault();
        }
        catch (Exception ex)
        {
            App.Window?.ShowToast($"Could not load curves: {ex.Message}", false);
        }
        finally
        {
            _loading = false;
        }
        ShowCurve();
        await LoadUsedByAsync();
    }

    private void ShowCurve()
    {
        _current = CurveCombo.SelectedItem as string;
        var profile = Profile;
        if (profile == null)
        {
            return;
        }

        ReadOnlyBar.IsOpen = IsReadOnly;
        DeleteButton.IsEnabled = !BuiltIn.Contains(_current);
        Editor.IsReadOnly = IsReadOnly;
        Editor.MaxX = Sensor == 2 ? 60 : 100;

        var flags = new List<string> { $"Sensor: {SensorNames.GetValueOrDefault(Sensor, "Unknown")}" };
        if (profile["zeroRpm"]?.GetValue<bool>() == true)
        {
            flags.Add("fans may stop");
        }
        SensorText.Text = string.Join(" · ", flags);

        var all = (profile["points"]?[Target.ToString()] as JsonArray ?? new JsonArray())
            .Select(p => new Point(p?["x"]?.GetValue<double>() ?? 0, p?["y"]?.GetValue<double>() ?? 0))
            .OrderBy(p => p.X)
            .ToList();
        _beyondRange = all.Where(p => p.X > Editor.MaxX).ToList();
        _original = all.Where(p => p.X <= Editor.MaxX).ToList();
        Editor.SetPoints(_original);
        ResetButton.IsEnabled = false;
        _ = UpdateMarkerAsync();
    }

    private async Task SaveAsync(IEnumerable<Point> points)
    {
        if (_current == null || IsReadOnly)
        {
            return;
        }
        var payload = points.Concat(_beyondRange)
            .OrderBy(p => p.X)
            .Select(p => new { x = p.X, y = p.Y })
            .ToList();
        var result = await App.Api.PutAsync("api/temperatures/updateGraph", new { profile = _current, updateType = Target, points = payload });
        if (!result.Ok)
        {
            App.Window?.ShowToast(result.Message, false);
            return;
        }
        if (Profile?["points"] is JsonObject stored)
        {
            stored[Target.ToString()] = new JsonArray(payload.Select(p => (JsonNode)new JsonObject { ["x"] = p.x, ["y"] = p.y }).ToArray());
        }
        ResetButton.IsEnabled = true;
    }

    private async Task UpdateMarkerAsync()
    {
        try
        {
            double? value = Sensor switch
            {
                0 => await App.Api.GetCpuTempAsync(),
                1 or 8 => await App.Api.GetGpuTempAsync(),
                5 => Math.Max(await App.Api.GetCpuTempAsync() ?? 0, await App.Api.GetGpuTempAsync() ?? 0),
                _ => null,
            };
            Editor.SetMarker(value ?? double.NaN);
        }
        catch
        {
        }
    }

    private async Task LoadUsedByAsync()
    {
        UsedBy.Children.Clear();
        if (_current == null)
        {
            return;
        }
        try
        {
            var devices = (await App.Api.GetDevicesAsync()).Where(d => d.Category is DeviceCategory.Cooling or DeviceCategory.PowerSupply);
            foreach (var device in devices)
            {
                var detail = await App.Api.GetDeviceAsync(device.Serial);
                if (detail?["devices"] is not JsonObject channels)
                {
                    continue;
                }
                foreach (var (_, channel) in channels)
                {
                    if (channel?["HasSpeed"]?.GetValue<bool>() == true && channel["profile"]?.GetValue<string>() == _current)
                    {
                        UsedBy.Children.Add(new SettingsCard
                        {
                            Header = channel["name"]?.GetValue<string>() ?? "",
                            Description = device.DisplayName,
                            HeaderIcon = new FontIcon { Glyph = "\uE9CA" },
                        });
                    }
                }
            }
            if (UsedBy.Children.Count == 0)
            {
                UsedBy.Children.Add(new TextBlock
                {
                    Text = "No fan uses this curve yet.",
                    Foreground = (Microsoft.UI.Xaml.Media.Brush)Application.Current.Resources["TextFillColorTertiaryBrush"],
                });
            }
        }
        catch
        {
        }
    }

    private async void CurveCombo_SelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (_loading)
        {
            return;
        }
        ShowCurve();
        await LoadUsedByAsync();
    }

    private void TargetBar_SelectionChanged(SelectorBar sender, SelectorBarSelectionChangedEventArgs args)
    {
        if (!_loading && IsLoaded)
        {
            ShowCurve();
        }
    }

    private async void Reset_Click(object sender, RoutedEventArgs e)
    {
        await SaveAsync(_original);
        Editor.SetPoints(_original);
        ResetButton.IsEnabled = false;
    }

    private async void New_Click(object sender, RoutedEventArgs e)
    {
        var name = new TextBox { Header = "Name", PlaceholderText = "Letters and numbers only" };
        var sensor = new ComboBox
        {
            Header = "Follow temperature of",
            ItemsSource = new[] { new Choice("0", "CPU"), new Choice("1", "GPU"), new Choice("2", "Liquid (AIO)"), new Choice("5", "CPU + GPU, whichever is hotter") },
            SelectedIndex = 0,
            HorizontalAlignment = HorizontalAlignment.Stretch,
        };
        var zeroRpm = new CheckBox { Content = "Let fans stop at low temperatures" };
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = "New fan curve",
            PrimaryButtonText = "Create",
            CloseButtonText = "Cancel",
            DefaultButton = ContentDialogButton.Primary,
            Content = new StackPanel { Spacing = 12, MinWidth = 360, Children = { name, sensor, zeroRpm } },
        };
        if (await dialog.ShowAsync() != ContentDialogResult.Primary)
        {
            return;
        }

        var result = await App.Api.PostAsync("api/temperatures/new", new
        {
            profile = name.Text.Trim(),
            @static = false,
            zeroRpm = zeroRpm.IsChecked == true,
            linear = false,
            sensor = int.Parse(((Choice)sensor.SelectedItem).Value),
            hwmonDeviceId = "",
            deviceId = "",
            channelId = 0,
            temperatureInputId = "",
            hwmonDevice = "",
            externalExecutable = "",
            gpuIndex = 0,
        });
        App.Window?.ShowToast(result.Message, result.Ok);
        if (result.Ok)
        {
            await LoadAsync(name.Text.Trim());
        }
    }

    private async void Delete_Click(object sender, RoutedEventArgs e)
    {
        if (_current == null)
        {
            return;
        }
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = $"Delete \"{_current}\"?",
            Content = "Fans using this curve go back to Normal.",
            PrimaryButtonText = "Delete",
            CloseButtonText = "Cancel",
        };
        if (await dialog.ShowAsync() != ContentDialogResult.Primary)
        {
            return;
        }
        var result = await App.Api.DeleteAsync("api/temperatures/delete", new { profile = _current });
        App.Window?.ShowToast(result.Message, result.Ok);
        _current = null;
        await LoadAsync();
    }
}

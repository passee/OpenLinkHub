using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using OpenLinkHub.App.Controls;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services.Icue;
using Windows.Foundation;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private const double MarkerRadius = 34;

    /// Button centres on the controller's mappings render (front view left, back view right),
    /// keyed by the service's button names. iCUE's manifest has no positions for most of them.
    private static readonly Dictionary<string, Point> ControllerMarkers = new(StringComparer.OrdinalIgnoreCase)
    {
        ["DPAD Up"] = new(516, 272), ["DPAD Down"] = new(516, 422), ["DPAD Left"] = new(442, 346), ["DPAD Right"] = new(590, 346),
        ["Y"] = new(1314, 256), ["X"] = new(1224, 344), ["B"] = new(1406, 344), ["A"] = new(1316, 432),
        ["LS (Press)"] = new(700, 524), ["RS (Press)"] = new(1130, 524),
        ["LOCK"] = new(660, 248), ["MENU"] = new(1170, 246), ["Power"] = new(916, 530), ["Profile"] = new(916, 626),
        ["G1"] = new(666, 724), ["G2"] = new(792, 724), ["G3"] = new(916, 724), ["G4"] = new(1042, 724), ["G5"] = new(1166, 724),
        ["RB"] = new(1660, 128), ["LB"] = new(2410, 128), ["RT"] = new(1640, 205), ["LT"] = new(2430, 205),
        ["P1"] = new(1790, 600), ["P4"] = new(2280, 600), ["P2"] = new(1665, 750), ["P3"] = new(2415, 750),
    };

    private static readonly Rect ControllerClip = new(220, 80, 2530, 960);

    /// The service keys controller buttons by bitmask; the render needs small ids.
    private readonly Dictionary<int, long> _buttonMasks = new();

    private bool IsController => _device?.Category == DeviceCategory.Controller;

    private async Task<bool> BuildControllerSurfaceAsync()
    {
        _buttonMasks.Clear();
        if (!IsController || _detail?["KeyAssignment"] is not JsonObject buttons)
        {
            return false;
        }
        var bundle = await IcueAssets.ForDeviceAsync(_device!, _detail);
        if (bundle?.BodyImage(null, "mappings") is not { } image)
        {
            return false;
        }

        var keys = new List<KeyInfo>();
        var outlines = new Dictionary<int, IcueItem>();
        var id = 1;
        foreach (var (mask, node) in buttons.OrderBy(b => long.Parse(b.Key)))
        {
            if (node is not JsonObject button)
            {
                continue;
            }
            var name = Str(button, "name") ?? mask;
            _buttonMasks[id] = long.Parse(mask);
            keys.Add(new KeyInfo(id, name, button));
            if (ControllerMarkers.TryGetValue(name, out var centre))
            {
                outlines[id] = new IcueItem(name, "Key", Circle(centre));
            }
            id++;
        }

        var view = new IcueKeyboardView(image, ControllerClip, outlines, keys) { MaxHeight = 360, ShowMarkers = true };
        _keyboard = view;
        _render = view;
        _bundle = bundle;
        _views = new[] { image };
        PreviewHost.Child = view;
        PreviewHost.Visibility = Visibility.Visible;
        return true;
    }

    private static List<Point> Circle(Point centre) => Enumerable.Range(0, 24)
        .Select(i => new Point(centre.X + MarkerRadius * Math.Cos(i * Math.PI / 12), centre.Y + MarkerRadius * Math.Sin(i * Math.PI / 12)))
        .ToList();

    private long KeyIndexFor(int id) => IsController ? _buttonMasks[id] : id;

    private string KeyEndpoint => IsController ? "api/controller/updateKeyAssignment" : "api/keyboard/updateKeyAssignment";

    // ---------- Thumbsticks ----------

    private IEnumerable<UIElement> SticksSection()
    {
        var profile = (JsonObject)_detail!["DeviceProfile"]!;
        var modes = (_detail["ThumbStickModes"] as JsonObject ?? new JsonObject())
            .Select(m => new Choice(m.Key, m.Value?.GetValue<string>() ?? m.Key))
            .ToList();
        foreach (var (stick, prefix, title) in new[] { (0, "Left", "Left stick"), (1, "Right", "Right stick") })
        {
            yield return Header(title);
            var mode = new ComboBox { ItemsSource = modes, MinWidth = 200 };
            mode.SelectedItem = modes.FirstOrDefault(m => m.Value == profile[$"{prefix}ThumbStickMode"]?.ToString());
            var x = new Slider { Minimum = 5, Maximum = 50, Width = 220, Value = profile[$"{prefix}ThumbStickSensitivityX"]?.GetValue<int>() ?? 5 };
            var y = new Slider { Minimum = 5, Maximum = 50, Width = 220, Value = profile[$"{prefix}ThumbStickSensitivityY"]?.GetValue<int>() ?? 5 };
            var invert = new ToggleSwitch { IsOn = profile[$"{prefix}ThumbStickInvertY"]?.GetValue<bool>() == true };

            var debounce = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(400) };
            var stickIndex = stick;
            debounce.Tick += async (_, _) =>
            {
                debounce.Stop();
                Report(await App.Api.PostAsync("api/controller/emulation", new
                {
                    deviceId = _device!.Serial,
                    emulationDevice = stickIndex,
                    emulationMode = int.Parse((mode.SelectedItem as Choice)?.Value ?? "2"),
                    sensitivityX = (int)x.Value,
                    sensitivityY = (int)y.Value,
                    invertYAxis = invert.IsOn,
                }));
            };
            void Changed()
            {
                debounce.Stop();
                debounce.Start();
            }
            mode.SelectionChanged += (_, _) => Changed();
            x.ValueChanged += (_, _) => Changed();
            y.ValueChanged += (_, _) => Changed();
            invert.Toggled += (_, _) => Changed();

            yield return new SettingsCard { Header = "Acts as", Description = "Thumbstick, mouse movement, or nothing", HeaderIcon = new FontIcon { Glyph = "\uE7FC" }, Content = mode };
            yield return new SettingsCard { Header = "Horizontal sensitivity", Description = "Used when the stick moves the mouse", Content = x };
            yield return new SettingsCard { Header = "Vertical sensitivity", Content = y };
            yield return new SettingsCard { Header = "Invert vertical axis", Content = invert };
        }
    }

    // ---------- Vibration ----------

    private IEnumerable<UIElement> VibrationSection()
    {
        var profile = (JsonObject)_detail!["DeviceProfile"]!;
        yield return Header("Rumble strength");
        foreach (var (module, key, title) in new[] { (0, "LeftVibrationValue", "Left motor"), (1, "RightVibrationValue", "Right motor") })
        {
            var motor = module;
            yield return SliderCard(title, module == 0 ? "The heavy rumble in the left grip" : "The light rumble in the right grip", "\uE877",
                0, 100, 5, profile[key]?.GetValue<int>() ?? 100, " %",
                async v => Report(await App.Api.PostAsync("api/controller/vibration", new { deviceId = _device!.Serial, vibrationModule = motor, vibrationValue = (int)v })));
        }
    }

    private UIElement? OfflineNotice() => _detail?["Connected"] is JsonValue c && c.TryGetValue<bool>(out var connected) && !connected
        ? new InfoBar
        {
            Severity = InfoBarSeverity.Warning,
            IsOpen = true,
            IsClosable = false,
            Title = "Switched off",
            Message = $"Turn the {_device!.CategoryName.ToLowerInvariant()} on to change its settings.",
            Margin = new Thickness(0, 0, 0, 8),
        }
        : null;
}

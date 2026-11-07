using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using OpenLinkHub.App.Controls;
using OpenLinkHub.App.Models;
using Windows.UI;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private IKeySurface? _keyboard;
    private DeviceDrawing? _drawing;
    private ComboBox? _effectCombo;
    private readonly Dictionary<int, ColorButton> _zoneButtons = new();

    /// The RGB profile that makes the device show its own per-key or per-zone colours.
    private string? ZoneProfileName => _device?.Category switch
    {
        DeviceCategory.Keyboard => "keyboard",
        DeviceCategory.Mouse => "mouse",
        DeviceCategory.Mousepad => "mousepad",
        DeviceCategory.Headset => "headset",
        _ => null,
    };

    private string ZoneProfileLabel => _device?.Category == DeviceCategory.Keyboard ? "Per-key colours" : "Custom zone colours";

    private KeyboardSurfaces.Parts? _keyboardParts;

    private async Task BuildPreviewAsync()
    {
        _keyboard = null;
        _drawing = null;
        _render = null;
        _bundle = null;
        PreviewHost.Child = null;
        PreviewHost.Visibility = Visibility.Collapsed;

        if (await BuildControllerSurfaceAsync())
        {
            return;
        }

        _keyboardParts = await KeyboardSurfaces.LoadAsync(_device!, _detail);
        if (_keyboardParts != null)
        {
            var (view, surface) = KeyboardSurfaces.Create(_keyboardParts, 340);
            _keyboard = surface;
            PreviewHost.Child = view;
            PreviewHost.Visibility = Visibility.Visible;
            return;
        }

        if (await BuildRenderAsync())
        {
            return;
        }

        if (DeviceDrawing.Supports(_device!.Category))
        {
            _drawing = new DeviceDrawing(_device.Category) { MaxHeight = 220 };
            PreviewHost.Child = _drawing;
            PreviewHost.Visibility = Visibility.Visible;
            _ = PaintDrawingAsync();
        }
    }

    private async Task PaintDrawingAsync()
    {
        foreach (var (zone, color) in await ZoneColorsAsync())
        {
            _drawing?.SetZoneColor(zone, color.Color);
        }
    }

    private void OnSectionChanging(string? section)
    {
        _ = ShowControllerViewAsync(section);
        if (_keyboard == null)
        {
            return;
        }
        _keyboard.ResetSelectionListeners();
        _keyboard.ClearSelection();
        _keyboard.AssignableOnly = section == "Keys";
        _keyboard.ShowColors = section != "Keys" && !IsController;
        _keyboard.SetHighlighted(section == "Keys"
            ? _keyboard.Keys.Values.Where(k => !k.OnlyColor && k.Data["default"]?.GetValue<bool>() == false).Select(k => k.Id)
            : Array.Empty<int>());
        _keyboard.SelectionMode = section switch
        {
            "Lighting" => IsController ? KeySelectionMode.None : KeySelectionMode.Multiple,
            "Keys" => KeySelectionMode.Single,
            _ => KeySelectionMode.None,
        };
    }

    // ---------- Zone and per-key colours ----------

    private sealed record ZoneColor(string Name, Color Color);

    private async Task<Dictionary<int, ZoneColor>> ZoneColorsAsync()
    {
        var result = new Dictionary<int, ZoneColor>();
        if (_device!.Category == DeviceCategory.Mousepad)
        {
            var rows = (await App.Api.GetAsync($"api/color/zone/{_device.Serial}"))?["data"]?["row"] as JsonObject
                       ?? (_detail?["DeviceProfile"]?["Stand"]?["row"] as JsonObject);
            foreach (var (_, row) in rows ?? new JsonObject())
            {
                foreach (var (id, zone) in row?["zones"] as JsonObject ?? new JsonObject())
                {
                    var name = Str(zone as JsonObject, "name");
                    result[int.Parse(id)] = new ZoneColor(string.IsNullOrEmpty(name) ? $"Edge {id}" : Capitalize(name.ToLowerInvariant()),
                        ColorButton.FromJson(zone?["color"]));
                }
            }
            return result;
        }

        foreach (var (id, zone) in _detail?["DeviceProfile"]?["ZoneColors"] as JsonObject ?? new JsonObject())
        {
            result[int.Parse(id)] = new ZoneColor(Str(zone as JsonObject, "Name") ?? $"Zone {id}", ColorButton.FromJson(zone?["Color"]));
        }
        return result;
    }

    private IEnumerable<UIElement> ZoneLightingCards()
    {
        if (_keyboard != null && !IsController)
        {
            yield return Header("Per-key colours");
            yield return PerKeyCard();
        }
        else if (_drawing != null || _detail?["DeviceProfile"]?["ZoneColors"] is JsonObject || _device!.Category == DeviceCategory.Mousepad)
        {
            yield return Header("Zone colours");
            var host = new StackPanel { Spacing = 4 };
            _ = FillZoneCardsAsync(host);
            yield return host;
        }
    }

    private async Task<bool> EnsureZoneProfileAsync()
    {
        var profile = _detail?["DeviceProfile"] as JsonObject;
        if (ZoneProfileName == null || Str(profile, "RGBProfile") == ZoneProfileName)
        {
            return true;
        }
        var result = await App.Api.SetRgbProfileAsync(_device!.Serial, 0, ZoneProfileName);
        if (!result.Ok)
        {
            Report(result);
            return false;
        }
        if (profile != null)
        {
            profile["RGBProfile"] = ZoneProfileName;
        }
        if (_effectCombo?.ItemsSource is IEnumerable<Choice> choices)
        {
            _syncingEffect = true;
            _effectCombo.SelectedItem = choices.FirstOrDefault(c => c.Value == ZoneProfileName);
            _syncingEffect = false;
        }
        App.Window?.ShowToast($"Effect switched to {ZoneProfileLabel}");
        return true;
    }

    private bool _syncingEffect;

    private UIElement PerKeyCard()
    {
        var serial = _device!.Serial;
        var color = new ColorButton(Colors.White);
        ToolTipService.SetToolTip(color, "Colour to paint");
        var selection = new TextBlock { VerticalAlignment = VerticalAlignment.Center, Foreground = (Microsoft.UI.Xaml.Media.Brush)Application.Current.Resources["TextFillColorSecondaryBrush"] };
        var paint = new Button { Content = "Paint selected", Style = (Style)Application.Current.Resources["AccentButtonStyle"], IsEnabled = false };
        var paintAll = new Button { Content = "Paint all keys" };
        var selectAll = new Button { Content = "Select all" };
        var clear = new Button { Content = "Clear selection" };

        void UpdateSelection()
        {
            var count = _keyboard!.SelectedKeys.Count;
            selection.Text = count == 0 ? "Click keys above to select them" : $"{count} selected";
            paint.IsEnabled = count > 0;
        }
        UpdateSelection();
        _keyboard!.SelectionChanged += (_, _) => UpdateSelection();
        selectAll.Click += (_, _) => _keyboard.SelectAll();
        clear.Click += (_, _) => _keyboard.ClearSelection();

        async Task PaintAsync(bool all)
        {
            if (!await EnsureZoneProfileAsync())
            {
                return;
            }
            var keys = all ? _keyboard.Keys.Where(k => !k.Value.NoColor).Select(k => k.Key).ToList() : _keyboard.SelectedKeys.ToList();
            var result = await App.Api.PostAsync("api/keyboard/color", new
            {
                deviceId = serial,
                keyId = keys.FirstOrDefault(1),
                keyOption = all ? 2 : 3,
                keys,
                color = ColorButton.ToJson(color.Color),
            });
            if (!result.Ok)
            {
                Report(result);
                return;
            }
            _keyboard.SetColor(keys, color.Color);
            var saved = await App.Api.PostAsync("api/keyboard/profile/save", new { deviceId = serial, keyboardProfileName = "0", @new = false });
            App.Window?.ShowToast(saved.Ok ? $"Painted {keys.Count} keys" : saved.Message, saved.Ok);
        }
        paint.Click += async (_, _) => await PaintAsync(false);
        paintAll.Click += async (_, _) => await PaintAsync(true);

        return new SettingsCard
        {
            Header = "Paint keys",
            Description = selection,
            HeaderIcon = new FontIcon { Glyph = "\uE790" },
            Content = new StackPanel
            {
                Orientation = Orientation.Horizontal,
                Spacing = 8,
                Children = { color, paint, paintAll, selectAll, clear },
            },
        };
    }

    private async Task FillZoneCardsAsync(StackPanel host)
    {
        var serial = _device!.Serial;
        var zones = await ZoneColorsAsync();
        _zoneButtons.Clear();

        foreach (var (id, zone) in zones.OrderBy(z => z.Key))
        {
            var button = new ColorButton(zone.Color);
            _zoneButtons[id] = button;
            var zoneId = id;
            button.ColorCommitted += async (_, c) => await SaveZoneAsync(zoneId, c);
            host.Children.Add(new SettingsCard
            {
                Header = zone.Name,
                HeaderIcon = new FontIcon { Glyph = "\uE781" },
                Content = button,
            });
        }

        if (_device.Category == DeviceCategory.Mouse && _detail?["DeviceProfile"]?["DPIColor"] is JsonObject dpi)
        {
            var dpiButton = new ColorButton(ColorButton.FromJson(dpi));
            dpiButton.ColorCommitted += async (_, c) =>
            {
                dpi["red"] = c.R;
                dpi["green"] = c.G;
                dpi["blue"] = c.B;
                await SaveZoneAsync(null, c);
            };
            host.Children.Add(new SettingsCard
            {
                Header = "DPI indicator",
                Description = "Colour that flashes when you change DPI",
                HeaderIcon = new FontIcon { Glyph = "\uE962" },
                Content = dpiButton,
            });
        }

        if (zones.Count > 1)
        {
            var all = new ColorButton(zones.First().Value.Color);
            all.ColorCommitted += async (_, c) =>
            {
                foreach (var b in _zoneButtons.Values)
                {
                    b.SetColor(c);
                }
                await SaveZoneAsync(-1, c);
            };
            host.Children.Insert(0, new SettingsCard
            {
                Header = "All zones",
                Description = "Set every zone to one colour",
                HeaderIcon = new FontIcon { Glyph = "\uE790" },
                Content = all,
            });
        }
    }

    /// zone -1 means every zone; null means only the DPI colour changed.
    private async Task SaveZoneAsync(int? zone, Color color)
    {
        if (!await EnsureZoneProfileAsync())
        {
            return;
        }
        var serial = _device!.Serial;
        Services.ApiResult result;

        if (_device.Category == DeviceCategory.Mousepad)
        {
            result = await App.Api.PostAsync("api/misc/color", new
            {
                deviceId = serial,
                areaId = zone is null or -1 ? _zoneButtons.Keys.First() : zone.Value,
                areaOption = zone == -1 ? 2 : 0,
                color = ColorButton.ToJson(color),
            });
            if (result.Ok)
            {
                await App.Api.PostAsync("api/keyboard/profile/save", new { deviceId = serial, keyboardProfileName = "0", @new = false });
            }
        }
        else
        {
            var zones = _zoneButtons.ToDictionary(z => z.Key.ToString(), z => ColorButton.ToJson(z.Value.Color));
            if (_device.Category == DeviceCategory.Mouse)
            {
                result = await App.Api.PostAsync("api/mouse/zoneColors", new
                {
                    deviceId = serial,
                    colorZones = zones,
                    colorDpi = ColorButton.ToJson(ColorButton.FromJson(_detail?["DeviceProfile"]?["DPIColor"])),
                });
            }
            else
            {
                result = await App.Api.PostAsync(IsController ? "api/controller/zoneColors" : "api/headset/zoneColors", new { deviceId = serial, colorZones = zones });
            }
        }

        Report(result);
        if (result.Ok)
        {
            foreach (var (id, button) in _zoneButtons)
            {
                _drawing?.SetZoneColor(id, button.Color);
            }
            _ = RefreshGlowsAsync();
        }
    }
}

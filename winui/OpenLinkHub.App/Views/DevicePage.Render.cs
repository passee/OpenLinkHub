using System.Text.Json.Nodes;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;
using OpenLinkHub.App.Controls;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services.Icue;
using Windows.Foundation;
using Windows.UI;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private const double RenderMargin = 24;

    /// The SCUF bundle has one render per page, named after it.
    private static readonly Dictionary<string, string> ControllerViewForSection = new()
    {
        ["Keys"] = "mappings", ["Lighting"] = "lightings", ["Thumbsticks"] = "thumbsticks", ["Vibration"] = "vibration",
    };

    private IcueBundle? _bundle;
    private IcueKeyboardView? _render;
    private IReadOnlyList<string> _views = Array.Empty<string>();
    private int _view;
    private readonly List<Button> _viewButtons = new();

    /// A render of a device that has no keys to click: mouse, headset, mousepad, hubs.
    private async Task<bool> BuildRenderAsync()
    {
        _bundle = await IcueAssets.ForDeviceAsync(_device!, _detail);
        _zoneCache = null;
        _views = _bundle?.Views() ?? Array.Empty<string>();
        if (_bundle == null || _views.Count == 0)
        {
            return false;
        }

        _view = 0;
        _render = new IcueKeyboardView(_views[0], Padded(_bundle.Bounds(_views[0])), new Dictionary<int, IcueItem>(), Array.Empty<KeyInfo>())
        {
            MaxHeight = 300,
        };
        var layout = new Grid { RowSpacing = 12 };
        layout.RowDefinitions.Add(new RowDefinition());
        layout.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        layout.Children.Add(_render);
        if (_views.Count > 1)
        {
            var switcher = ViewSwitcher();
            Grid.SetRow(switcher, 1);
            layout.Children.Add(switcher);
        }
        PreviewHost.Child = layout;
        PreviewHost.Visibility = Visibility.Visible;
        await RefreshGlowsAsync();
        return true;
    }

    private FrameworkElement ViewSwitcher()
    {
        _viewButtons.Clear();
        var row = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8, HorizontalAlignment = HorizontalAlignment.Center };
        for (var i = 0; i < _views.Count; i++)
        {
            var index = i;
            var button = new Button
            {
                Padding = new Thickness(6),
                Content = new Image
                {
                    Source = new BitmapImage(new Uri(_views[i])) { DecodePixelHeight = 120 },
                    Height = 44,
                    Width = 88,
                    Stretch = Stretch.Uniform,
                },
            };
            ToolTipService.SetToolTip(button, $"View {i + 1}");
            Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(button, $"View {i + 1}");
            button.Click += async (_, _) => await ShowViewAsync(index);
            _viewButtons.Add(button);
            row.Children.Add(button);
        }
        MarkView();
        return row;
    }

    private async Task ShowViewAsync(int index)
    {
        if (_render == null || _bundle == null)
        {
            return;
        }
        _view = index;
        _render.SetImage(_views[index], Padded(_bundle.Bounds(_views[index])));
        MarkView();
        await RefreshGlowsAsync();
    }

    private void MarkView()
    {
        for (var i = 0; i < _viewButtons.Count; i++)
        {
            _viewButtons[i].BorderBrush = i == _view
                ? (Brush)Application.Current.Resources["AccentFillColorDefaultBrush"]
                : (Brush)Application.Current.Resources["ControlStrokeColorDefaultBrush"];
            _viewButtons[i].BorderThickness = new Thickness(i == _view ? 2 : 1);
        }
    }

    /// The controller shows the render that belongs to the open tab; buttons are clickable on the mappings one.
    private async Task ShowControllerViewAsync(string? section)
    {
        if (!IsController || _render == null || _bundle == null)
        {
            return;
        }
        var purpose = section != null && ControllerViewForSection.TryGetValue(section, out var p) ? p : "mappings";
        var image = _bundle.BodyImage(null, purpose);
        if (image == null)
        {
            return;
        }
        _view = 0;
        var mappings = purpose == "mappings";
        _render.SetImage(image, mappings ? ControllerClip : Padded(_bundle.Bounds(image)));
        _render.ShowHotspots = mappings;
        _views = new[] { image };
        await RefreshGlowsAsync();
    }

    private static Rect? Padded(Rect? bounds) => bounds is { } b
        ? new Rect(b.X - RenderMargin, b.Y - RenderMargin, b.Width + 2 * RenderMargin, b.Height + 2 * RenderMargin)
        : null;

    /// Paints each zone's current colour onto the render through iCUE's glow masks.
    private async Task RefreshGlowsAsync()
    {
        if (_render == null || _bundle == null)
        {
            return;
        }
        var leds = _bundle.GlowLeds();
        if (leds.Count == 0)
        {
            return;
        }
        // The controller's glow masks belong to its lighting render only.
        var glowView = IsController ? (_views.FirstOrDefault()?.Contains("lightings") == true ? 0 : -1) : _view;

        _zoneCache ??= await ZoneColorsAsync();
        var glows = new List<(string, Color)>();
        foreach (var (id, zone) in _zoneCache)
        {
            var color = _liveZones?.TryGetValue(id, out var live) == true ? live
                : _zoneButtons.TryGetValue(id, out var button) ? button.Color : zone.Color;
            if (LedForZone(id, zone.Name, leds) is { } led && _bundle.GlowMask(led, glowView) is { } mask)
            {
                glows.Add((mask, color));
            }
        }
        if (_device!.Category == DeviceCategory.Mouse && _detail?["DeviceProfile"]?["DPIColor"] is JsonObject dpi)
        {
            foreach (var led in leds.Where(l => l.Contains("Dpi", StringComparison.OrdinalIgnoreCase)))
            {
                if (_bundle.GlowMask(led, glowView) is { } mask)
                {
                    glows.Add((mask, _liveDpi ?? ColorButton.FromJson(dpi)));
                }
            }
        }
        // An unlit LED shows the render as it is rather than a black patch.
        _render.SetGlows(glows.Select(g => g.Item2 is { R: 0, G: 0, B: 0 } ? (g.Item1, Color.FromArgb(0, 0, 0, 0)) : g));
    }

    private Dictionary<int, ZoneColor>? _zoneCache;

    /// Service zones are numbered or named ("Logo", "Scroll"); iCUE names LEDs like "MouseLogoLed" or "MouseMat_Zone1".
    private static string? LedForZone(int id, string name, IReadOnlyList<string> leds)
    {
        var numbered = leds.FirstOrDefault(l => l.EndsWith($"Zone{id}", StringComparison.OrdinalIgnoreCase));
        if (numbered != null)
        {
            return numbered;
        }
        var word = new string(name.TakeWhile(char.IsLetter).ToArray());
        if (word.Length >= 3 && leds.FirstOrDefault(l => l.Contains(word, StringComparison.OrdinalIgnoreCase)) is { } named)
        {
            return named;
        }
        return leds.Count == 1 && id == 0 ? leds[0] : null;
    }
}

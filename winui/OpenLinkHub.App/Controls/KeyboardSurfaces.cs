using System.Text.Json.Nodes;
using Microsoft.UI.Xaml;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services.Icue;
using Windows.Foundation;

namespace OpenLinkHub.App.Controls;

public static class KeyboardSurfaces
{
    public sealed record Parts(string? Image, Rect? Clip, Dictionary<int, IcueItem> Outlines, JsonObject Layout)
    {
        public bool HasRender => Image != null;
    }

    public static JsonObject? Layout(JsonObject? detail)
    {
        if (detail?["DeviceProfile"]?["Keyboards"] is not JsonObject keyboards)
        {
            return null;
        }
        var name = detail["DeviceProfile"]?["Profile"] is JsonValue v && v.TryGetValue<string>(out var s) ? s : "default";
        return keyboards[name] as JsonObject ?? keyboards.FirstOrDefault().Value as JsonObject;
    }

    public static async Task<Parts?> LoadAsync(DeviceSummary device, JsonObject? detail)
    {
        if (Layout(detail) is not { } layout)
        {
            return null;
        }
        var bundle = await IcueAssets.ForDeviceAsync(device, detail);
        var layoutCode = detail?["DeviceProfile"]?["Layout"]?.GetValue<string>();
        return await Task.Run(() =>
        {
            if (bundle?.BodyImage(layoutCode) is not { } image)
            {
                return new Parts(null, null, new Dictionary<int, IcueItem>(), layout);
            }
            var placed = KeyboardView.Place(layout);
            var outlines = KeyMatcher.Match(placed, bundle.Items(layoutCode), bundle.DeviceLedNames());
            return outlines.Count < placed.Count / 2
                ? new Parts(null, null, new Dictionary<int, IcueItem>(), layout)
                : new Parts(image, Covering(bundle.ClipRect, outlines.Values), outlines, layout);
        });
    }

    /// iCUE's crop leaves out the side and top light strips; grow it to fit every outline.
    private static Rect? Covering(Rect? clip, IEnumerable<IcueItem> outlines)
    {
        if (clip is not { } area)
        {
            return null;
        }
        const double margin = 6;
        foreach (var outline in outlines)
        {
            area.Union(outline.Bounds);
        }
        return new Rect(area.X - margin, area.Y - margin, area.Width + 2 * margin, area.Height + 2 * margin);
    }

    /// A fresh view each time: the same element can't sit in two places of the visual tree.
    public static (UIElement View, IKeySurface Surface) Create(Parts parts, double maxHeight)
    {
        var keys = KeyboardView.Place((JsonObject)parts.Layout.DeepClone()).Select(p => p.Info).ToList();
        if (parts.HasRender)
        {
            var view = new IcueKeyboardView(parts.Image!, parts.Clip, parts.Outlines, keys) { MaxHeight = maxHeight };
            return (view, view);
        }
        var drawn = new KeyboardView { SelectionMode = KeySelectionMode.None, MaxHeight = maxHeight };
        drawn.Load(parts.Layout);
        return (drawn, drawn);
    }

    /// iCUE's name for a key: its outline id, or the name its LED index carries.
    public static string? KeyName(Parts parts, KeyInfo key) =>
        parts.Outlines.TryGetValue(key.Id, out var outline) ? outline.Id
        : KeyMatcher.LedIndex(key) is { } led ? KeyMatcher.NameForLed(led) : null;
}

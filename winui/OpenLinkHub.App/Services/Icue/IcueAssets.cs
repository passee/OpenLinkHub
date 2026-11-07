using System.Text.Json.Nodes;
using OpenLinkHub.App.Models;
using Windows.Foundation;

namespace OpenLinkHub.App.Services.Icue;

public sealed record IcueItem(string Id, string Type, IReadOnlyList<Point> Polygon)
{
    public Rect Bounds
    {
        get
        {
            double x0 = Polygon.Min(p => p.X), y0 = Polygon.Min(p => p.Y);
            return new Rect(x0, y0, Polygon.Max(p => p.X) - x0, Polygon.Max(p => p.Y) - y0);
        }
    }

    public Point Center => new(Bounds.X + Bounds.Width / 2, Bounds.Y + Bounds.Height / 2);
}

/// Device renders, thumbnails and key outlines in Assets/Devices, copied from iCUE with
/// tools/extract-icue-assets.py. The app never reads an iCUE installation.
public static class IcueAssets
{
    private const int CorsairVendorId = 6940;

    private static readonly string DevicesFolder = Path.Combine(AppContext.BaseDirectory, "Assets", "Devices");

    public static readonly string CacheFolder = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "OpenLinkHub.App");

    /// Hubs, PSUs and memory don't report a USB product id through the service.
    private static readonly Dictionary<int, int> ProductTypeToPid = new()
    {
        [1] = 3100, [4] = 3098, [6] = 3088, [8] = 1027, [203] = 2667, [501] = 7174, [606] = 14857, [998] = 7078,
    };

    private static Task<Dictionary<int, List<IcueBundle>>>? _index;

    public static bool Available => Directory.Exists(DevicesFolder);

    public static async Task<IcueBundle?> ForDeviceAsync(DeviceSummary device, JsonObject? detail)
    {
        if (!Available)
        {
            return null;
        }
        var index = await (_index ??= Task.Run(BuildIndex));
        var pid = detail?["ProductId"]?.GetValue<int>() is > 0 and var p ? p : ProductTypeToPid.GetValueOrDefault(device.ProductType);
        if (!index.TryGetValue(pid, out var bundles))
        {
            return null;
        }
        var hasLcd = detail?["HasLCD"]?.GetValue<bool>() == true;
        return bundles.FirstOrDefault(b => b.Name.Contains("screen") == hasLcd) ?? bundles[0];
    }

    private static Dictionary<int, List<IcueBundle>> BuildIndex()
    {
        var index = new Dictionary<int, List<IcueBundle>>();
        foreach (var folder in Directory.EnumerateDirectories(DevicesFolder))
        {
            try
            {
                var bundle = new IcueBundle(folder);
                foreach (var usb in bundle.Common["UsbIds"] as JsonArray ?? new JsonArray())
                {
                    if (usb?["vid"]?.GetValue<int>() == CorsairVendorId && usb["pid"]?.GetValue<int>() is int pid)
                    {
                        (index.TryGetValue(pid, out var list) ? list : index[pid] = new List<IcueBundle>()).Add(bundle);
                    }
                }
            }
            catch (Exception ex)
            {
                App.Log(ex);
            }
        }
        return index;
    }
}

public sealed class IcueBundle
{
    private readonly string _folder;
    private readonly Dictionary<string, string> _images;

    public IcueBundle(string folder)
    {
        _folder = folder;
        Name = Path.GetFileName(folder);
        Common = JsonNode.Parse(File.ReadAllText(Path.Combine(folder, "manifest.json")))?["common"] as JsonObject ?? new JsonObject();
        _images = (Common["Images"] as JsonObject ?? new JsonObject())
            .ToDictionary(kv => kv.Key, kv => Path.Combine(folder, kv.Value!.GetValue<string>()), StringComparer.OrdinalIgnoreCase);
    }

    public string Name { get; }

    public JsonObject Common { get; }

    /// A receiver's folder may also carry its paired device's picture; that one is preferred.
    public string? Thumbnail
    {
        get
        {
            var folder = Path.Combine(_folder, "thumbnails");
            if (!Directory.Exists(folder))
            {
                return null;
            }
            var thumbnails = Directory.EnumerateFiles(folder, "*.png").OrderBy(f => f).ToList();
            return thumbnails.FirstOrDefault(f => !Path.GetFileName(f).Contains("dongle", StringComparison.OrdinalIgnoreCase))
                   ?? thumbnails.FirstOrDefault();
        }
    }

    /// The part of the render the device occupies, in render pixels.
    public Rect? ClipRect
    {
        get
        {
            var clip = Common["CanvasParameters"]?["regularParameters"] is JsonArray { Count: > 0 } parameters
                ? parameters[0]?["clipRect"]
                : null;
            return clip == null ? null : new Rect(
                clip["x"]!.GetValue<double>(), clip["y"]!.GetValue<double>(),
                clip["width"]!.GetValue<double>(), clip["height"]!.GetValue<double>());
        }
    }

    /// Keyboards have one render per printed layout; the service's layout code (DE, UK, ...) picks it.
    /// Other devices may have one per page (mappings, lightings, ...); purpose picks among those.
    public string? BodyImage(string? layoutCode, string? purpose = null)
    {
        if (LayoutFor(layoutCode) is { } layout && layoutCode != null
            && layout["layoutImages"]?[layoutCode.ToUpperInvariant()]?["Black"]?[0]?.GetValue<string>() is { } qtPath
            && _images.TryGetValue(qtPath, out var keyboardImage))
        {
            return keyboardImage;
        }
        var bodies = _images.OrderBy(kv => kv.Key).Select(kv => kv.Value).ToList();
        return bodies.FirstOrDefault(f => purpose != null && f.Contains(purpose, StringComparison.OrdinalIgnoreCase))
               ?? bodies.FirstOrDefault(f => f.Contains("mappings") || f.Contains("00_") || f.Contains("body_1"))
               ?? bodies.FirstOrDefault();
    }

    public IReadOnlyList<IcueItem> Items(string? layoutCode)
    {
        var geometry = Common["StaticGeometryMap"]?["RegularStatic"] as JsonObject;
        var layout = LayoutFor(layoutCode);
        var items = new List<IcueItem>();
        if (geometry == null || layout?["keyItems"] is not JsonArray keyItems)
        {
            return items;
        }

        foreach (var item in keyItems.OfType<JsonObject>())
        {
            if (item["viewIndex"]?.GetValue<int>() is > 0)
            {
                continue;
            }
            var reference = item["geometryIndex"]?["Black"]?.GetValue<string>()?.Split(',')[0].Trim();
            if (reference == null || reference.StartsWith('-') || geometry[reference] is not JsonArray points || points.Count == 0)
            {
                continue;
            }
            var polygon = points.Select(p => new Point(Number(p?["x"]), Number(p?["y"]))).ToList();
            items.Add(new IcueItem(item["id"]?.GetValue<string>() ?? "", item["type"]?.GetValue<string>() ?? "", polygon));
        }
        return items;
    }

    /// Every body render in view order (top view first, then side views or angles).
    public IReadOnlyList<string> Views() => _images
        .Where(kv => kv.Key.Contains("/body/"))
        .OrderBy(kv => kv.Key, StringComparer.Ordinal)
        .Select(kv => kv.Value)
        .ToList();

    /// The visible part of a render, in render pixels.
    public Rect? Bounds(string imagePath)
    {
        var relative = Path.GetRelativePath(_folder, imagePath).Replace(Path.DirectorySeparatorChar, '/');
        var b = Common["ImageBounds"]?[relative];
        return b == null ? null : new Rect(Number(b["x"]), Number(b["y"]), Number(b["width"]), Number(b["height"]));
    }

    /// LEDs iCUE can paint onto the renders.
    public IReadOnlyList<string> GlowLeds() => LedItems()
        .Where(item => item.Geometry.Any(g => Common["LedGlowMasks"]?[g] != null))
        .Select(item => item.Id)
        .ToList();

    /// The glow shape of an LED on one view, or null when the LED isn't visible from that side.
    public string? GlowMask(string ledId, int view)
    {
        var geometry = LedItems().FirstOrDefault(i => i.Id == ledId).Geometry;
        if (geometry == null || view < 0 || view >= geometry.Length)
        {
            return null;
        }
        var qtPath = Common["LedGlowMasks"]?[geometry[view]]?.GetValue<string>();
        return qtPath != null && _images.TryGetValue(qtPath, out var mask) ? mask : null;
    }

    private IEnumerable<(string Id, string[] Geometry)> LedItems() =>
        (LayoutFor(null)?["keyItems"] as JsonArray ?? new JsonArray())
        .OfType<JsonObject>()
        .Where(i => i["type"]?.GetValue<string>() == "Led")
        .Select(i => (i["id"]?.GetValue<string>() ?? "",
            (i["geometryIndex"]?["Black"]?.GetValue<string>() ?? "").Split(',').Select(g => g.Trim()).ToArray()));

    /// iCUE's LED names for the device's own LEDs, by hardware LED index.
    public IReadOnlyDictionary<int, string> DeviceLedNames()
    {
        var difference = Common["DeviceSpecificIndexToLedMap"]?["difference"] as JsonObject ?? new JsonObject();
        return difference
            .Where(kv => int.TryParse(kv.Key, out _) && kv.Value?.GetValue<string>() is { } v && v != "None")
            .ToDictionary(kv => int.Parse(kv.Key), kv => kv.Value!.GetValue<string>());
    }

    private JsonObject? LayoutFor(string? layoutCode)
    {
        if (Common["Layouts"] is not JsonObject layouts)
        {
            return null;
        }
        var code = layoutCode?.ToUpperInvariant();
        return (code != null ? layouts.FirstOrDefault(l => l.Value?["layoutImages"]?[code] != null).Value : null) as JsonObject
               ?? (code != null ? layouts[code] : null) as JsonObject
               ?? layouts["US"] as JsonObject
               ?? layouts.FirstOrDefault().Value as JsonObject;
    }

    private static double Number(JsonNode? node) => node is JsonValue v && v.TryGetValue<double>(out var d)
        ? d
        : double.TryParse(node?.ToString(), System.Globalization.CultureInfo.InvariantCulture, out var parsed) ? parsed : 0;
}

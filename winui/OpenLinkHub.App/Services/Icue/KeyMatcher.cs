using OpenLinkHub.App.Controls;
using Windows.Foundation;

namespace OpenLinkHub.App.Services.Icue;

/// Pairs the service's keys with iCUE's key outlines. A key's colour packet index divided by
/// three is its hardware LED index, which iCUE names either in the device manifest or after the
/// key's HID usage. Keys without such a name are placed by position, using the named keys to
/// fit the service's layout onto the render.
public static class KeyMatcher
{
    private const int BytesPerLed = 3;

    /// Corsair keyboards number these LEDs outside the HID range; the service's own K100 data shows this table.
    private static readonly Dictionary<int, string> CorsairLeds = new()
    {
        [102] = "Mute", [105] = "LeftCtrl", [106] = "LeftShift", [107] = "LeftAlt", [108] = "LeftGui",
        [109] = "RightCtrl", [110] = "RightShift", [111] = "RightAlt", [112] = "RightGui", [114] = "WinLock",
        [122] = "Fn", [123] = "Stop", [124] = "PlayPause", [125] = "ScanNextTrack", [126] = "ScanPreviousTrack",
        [128] = "Key_Profile", [131] = "G1", [132] = "G2", [133] = "G3", [134] = "G4", [135] = "G5", [136] = "G6",
    };

    /// Keys without an LED, named the same way in every service layout.
    private static readonly Dictionary<string, string> UnlitKeys = new(StringComparer.OrdinalIgnoreCase)
    {
        ["WH +"] = "VolumeUp", ["WH -"] = "VolumeDown",
    };

    public static Dictionary<int, IcueItem> Match(IReadOnlyList<KeyboardView.PlacedKey> keys, IReadOnlyList<IcueItem> items,
        IReadOnlyDictionary<int, string> deviceLedNames)
    {
        var byId = items.GroupBy(i => i.Id).ToDictionary(g => g.Key, g => g.First());
        var pressable = items.Where(i => i.Type == "Key").Select(i => i.Id).ToHashSet();
        var hidNames = KeyCodes.HidUsage.GroupBy(kv => kv.Value).ToDictionary(g => g.Key, g => g.First().Key);
        var result = new Dictionary<int, IcueItem>();
        var used = new HashSet<string>();

        foreach (var key in keys)
        {
            string? name;
            if (LedIndex(key.Info) is { } led)
            {
                name = deviceLedNames.GetValueOrDefault(led) ?? hidNames.GetValueOrDefault(led) ?? CorsairLeds.GetValueOrDefault(led);
            }
            else
            {
                name = UnlitKeys.GetValueOrDefault(key.Info.Name.Trim());
            }
            if (name != null && byId.TryGetValue(name, out var item) && used.Add(name))
            {
                result[key.Info.Id] = WithExtraLeds(key.Info, item, byId, deviceLedNames, used);
            }
        }

        var anchors = keys
            .Where(k => k.Rect != null && result.ContainsKey(k.Info.Id) && !k.Info.OnlyColor)
            .Select(k => (From: Center(k.Rect!.Value), To: result[k.Info.Id].Center))
            .ToList();
        if (anchors.Count < 4)
        {
            return result;
        }
        var (ax, bx) = Fit(anchors.Select(a => (a.From.X, a.To.X)).ToList());
        var (ay, by) = Fit(anchors.Select(a => (a.From.Y, a.To.Y)).ToList());
        var tolerance = result.Values.Average(i => Math.Min(i.Bounds.Width, i.Bounds.Height)) * 1.2;

        var candidates = new List<(double Distance, int KeyId, IcueItem Item)>();
        foreach (var key in keys.Where(k => k.Rect != null && !result.ContainsKey(k.Info.Id)))
        {
            var c = Center(key.Rect!.Value);
            var predicted = new Point(ax * c.X + bx, ay * c.Y + by);
            foreach (var item in byId.Values.Where(i => !used.Contains(i.Id) && (key.Info.OnlyColor || pressable.Contains(i.Id))))
            {
                var d = Distance(predicted, item.Center);
                if (d < tolerance)
                {
                    candidates.Add((d, key.Info.Id, item));
                }
            }
        }
        foreach (var (_, keyId, item) in candidates.OrderBy(c => c.Distance))
        {
            if (!result.ContainsKey(keyId) && used.Add(item.Id))
            {
                result[keyId] = item;
            }
        }
        return result;
    }

    /// A key lit by several LEDs (the K100 logo has three) covers all their outlines.
    private static IcueItem WithExtraLeds(KeyInfo key, IcueItem first, IReadOnlyDictionary<string, IcueItem> byId,
        IReadOnlyDictionary<int, string> deviceLedNames, HashSet<string> used)
    {
        if (key.Data["packetIndex"] is not System.Text.Json.Nodes.JsonArray { Count: > 1 } packets)
        {
            return first;
        }
        var bounds = first.Bounds;
        foreach (var packet in packets.Skip(1))
        {
            if (deviceLedNames.GetValueOrDefault(packet!.GetValue<int>() / BytesPerLed) is { } name
                && byId.TryGetValue(name, out var extra) && used.Add(name))
            {
                bounds.Union(extra.Bounds);
            }
        }
        return first with
        {
            Polygon = new[]
            {
                new Point(bounds.Left, bounds.Top), new Point(bounds.Right, bounds.Top),
                new Point(bounds.Right, bounds.Bottom), new Point(bounds.Left, bounds.Bottom),
            },
        };
    }

    /// iCUE's name for a key, from its LED index alone.
    public static string? NameForLed(int led) =>
        KeyCodes.HidUsage.FirstOrDefault(kv => kv.Value == led).Key ?? CorsairLeds.GetValueOrDefault(led);

    public static int? LedIndex(KeyInfo key) =>
        key.Data["packetIndex"] is System.Text.Json.Nodes.JsonArray { Count: > 0 } packets
            ? packets[0]!.GetValue<int>() / BytesPerLed
            : null;

    private static Point Center(Rect r) => new(r.X + r.Width / 2, r.Y + r.Height / 2);

    private static double Distance(Point a, Point b) => Math.Sqrt((a.X - b.X) * (a.X - b.X) + (a.Y - b.Y) * (a.Y - b.Y));

    /// Least-squares fit of to = a * from + b.
    private static (double A, double B) Fit(IReadOnlyList<(double From, double To)> pairs)
    {
        var n = pairs.Count;
        double sx = pairs.Sum(p => p.From), sy = pairs.Sum(p => p.To);
        double sxx = pairs.Sum(p => p.From * p.From), sxy = pairs.Sum(p => p.From * p.To);
        var denominator = n * sxx - sx * sx;
        if (Math.Abs(denominator) < 1e-9)
        {
            return (1, (sy - sx) / n);
        }
        var a = (n * sxy - sx * sy) / denominator;
        return (a, (sy - a * sx) / n);
    }
}

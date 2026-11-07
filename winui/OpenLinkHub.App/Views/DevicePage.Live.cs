using System.Text.Json.Nodes;
using Microsoft.UI.Xaml;
using OpenLinkHub.App.Models;
using Windows.UI;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private readonly DispatcherTimer _liveTimer = new() { Interval = TimeSpan.FromMilliseconds(150) };
    private byte[]? _lastFrame;
    private bool _liveBusy;
    private Dictionary<int, Color>? _liveZones;
    private Color? _liveDpi;

    private void StartLiveColors()
    {
        _lastFrame = null;
        _liveZones = null;
        _liveTimer.Tick -= LiveTimer_Tick;
        _liveTimer.Tick += LiveTimer_Tick;
        _liveTimer.Start();
    }

    private void StopLiveColors() => _liveTimer.Stop();

    private async void LiveTimer_Tick(object? sender, object e)
    {
        if (_liveBusy || _device == null)
        {
            return;
        }
        _liveBusy = true;
        try
        {
            var frame = await App.Api.GetLiveFrameAsync(_device.Serial);
            if (frame == null || _lastFrame != null && frame.AsSpan().SequenceEqual(_lastFrame))
            {
                return;
            }
            _lastFrame = frame;
            await ApplyLiveFrameAsync(frame);
        }
        catch
        {
        }
        finally
        {
            _liveBusy = false;
        }
    }

    private async Task ApplyLiveFrameAsync(byte[] frame)
    {
        if (_keyboard != null && !IsController)
        {
            var colors = new Dictionary<int, Color>();
            foreach (var (id, key) in _keyboard.Keys)
            {
                // Keyboards keep red, green and blue next to each other at the key's packet index.
                if (key.Data["packetIndex"] is JsonArray { Count: > 0 } packets && packets[0]!.GetValue<int>() is var p && p + 2 < frame.Length)
                {
                    colors[id] = Color.FromArgb(255, frame[p], frame[p + 1], frame[p + 2]);
                }
            }
            _keyboard.SetLiveColors(colors);
            return;
        }

        _liveZones = new Dictionary<int, Color>();
        foreach (var (id, indices) in ZoneColorIndices())
        {
            if (indices.All(i => i < frame.Length))
            {
                _liveZones[id] = Color.FromArgb(255, frame[indices[0]], frame[indices[1]], frame[indices[2]]);
            }
        }
        _liveDpi = DpiLedColor(frame);
        await RefreshGlowsAsync();
    }

    /// Where each zone's red, green and blue bytes sit in the device's colour frame.
    private Dictionary<int, int[]> ZoneColorIndices()
    {
        var result = new Dictionary<int, int[]>();
        var profile = _detail?["DeviceProfile"] as JsonObject;
        if (_device!.Category == DeviceCategory.Mousepad)
        {
            // The mousepad stores each colour channel for all zones together: red, then green, then blue.
            foreach (var (_, row) in profile?["Stand"]?["row"] as JsonObject ?? new JsonObject())
            {
                foreach (var (id, zone) in row?["zones"] as JsonObject ?? new JsonObject())
                {
                    if (zone?["packetIndex"] is JsonArray { Count: > 0 } packets && packets[0]!.GetValue<int>() is var p)
                    {
                        result[int.Parse(id)] = new[] { p, p + 3, p + 6 };
                    }
                }
            }
            return result;
        }
        foreach (var (id, zone) in profile?["ZoneColors"] as JsonObject ?? new JsonObject())
        {
            if (zone?["ColorIndex"] is JsonArray { Count: >= 3 } index)
            {
                result[int.Parse(id)] = index.Take(3).Select(i => i!.GetValue<int>()).ToArray();
            }
        }
        return result;
    }

    /// The Ironclaw's three DPI LEDs follow its lighting zones in the frame, one channel block each.
    private Color? DpiLedColor(byte[] frame)
    {
        if (_device!.Category != DeviceCategory.Mouse || frame.Length % 3 != 0)
        {
            return null;
        }
        var leds = frame.Length / 3;
        var zones = ZoneColorIndices().Count;
        return leds > zones ? Color.FromArgb(255, frame[zones], frame[leds + zones], frame[2 * leds + zones]) : null;
    }
}

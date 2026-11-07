using System.Text.Json.Nodes;
using Microsoft.UI.Xaml;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Controls;

public static class DevicePreviews
{
    /// The device's iCUE thumbnail when iCUE is installed, otherwise a drawn stand-in.
    public static async Task<UIElement?> CreateAsync(DeviceSummary device, JsonObject? detail)
    {
        var bundle = await Services.Icue.IcueAssets.ForDeviceAsync(device, detail);
        if (bundle != null && await Task.Run(() => bundle.Thumbnail) is { } thumbnail)
        {
            return new Microsoft.UI.Xaml.Controls.Image
            {
                Source = new Microsoft.UI.Xaml.Media.Imaging.BitmapImage(new Uri(thumbnail)) { DecodePixelHeight = 240 },
                Stretch = Microsoft.UI.Xaml.Media.Stretch.Uniform,
            };
        }
        return Create(device, detail);
    }

    /// A non-interactive picture of the device for cards, or null when there is nothing better than its icon.
    public static UIElement? Create(DeviceSummary device, JsonObject? detail)
    {
        var profile = detail?["DeviceProfile"] as JsonObject;
        if (profile?["Keyboards"] is JsonObject keyboards)
        {
            var name = profile["Profile"]?.GetValue<string>() ?? "default";
            if ((keyboards[name] ?? keyboards.FirstOrDefault().Value) is JsonObject layout)
            {
                var view = new KeyboardView { SelectionMode = KeySelectionMode.None, IsHitTestVisible = false };
                view.Load(layout);
                return view;
            }
        }

        if (!DeviceDrawing.Supports(device.Category))
        {
            return null;
        }

        var drawing = new DeviceDrawing(device.Category) { IsHitTestVisible = false };
        foreach (var (id, zone) in profile?["ZoneColors"] as JsonObject ?? new JsonObject())
        {
            drawing.SetZoneColor(int.Parse(id), ColorButton.FromJson(zone?["Color"]));
        }
        foreach (var (_, row) in profile?["Stand"]?["row"] as JsonObject ?? new JsonObject())
        {
            foreach (var (id, zone) in row?["zones"] as JsonObject ?? new JsonObject())
            {
                drawing.SetZoneColor(int.Parse(id), ColorButton.FromJson(zone?["color"]));
            }
        }
        return drawing;
    }
}

using System.Text.Json.Nodes;

namespace OpenLinkHub.App.Models;

public enum DeviceCategory
{
    Keyboard,
    Mouse,
    Headset,
    Controller,
    Mousepad,
    Cooling,
    Lighting,
    Memory,
    PowerSupply,
    Receiver,
    Other,
}

public sealed class DeviceSummary
{
    public required string Serial { get; init; }
    public required string Product { get; init; }
    public int ProductType { get; init; }
    public string Image { get; init; } = "";
    public string Firmware { get; init; } = "";
    public bool Hidden { get; init; }
    public string Label { get; init; } = "";

    public DeviceCategory Category => CategoryFor(ProductType, Image);

    public string DisplayName => Product.Length > 0 && Product == Product.ToUpperInvariant()
        ? System.Globalization.CultureInfo.CurrentCulture.TextInfo.ToTitleCase(Product.ToLowerInvariant())
            .Replace("Rgb", "RGB").Replace("Icue", "iCUE").Replace("Hs80", "HS80").Replace("Mm700", "MM700")
            .Replace("Hx850I", "HX850i").Replace("Scuf", "SCUF").Replace("Core", "CORE")
        : Product;

    public string Glyph => GlyphFor(Category);

    public string CategoryName => Category switch
    {
        DeviceCategory.Keyboard => "Keyboard",
        DeviceCategory.Mouse => "Mouse",
        DeviceCategory.Headset => "Headset",
        DeviceCategory.Controller => "Controller",
        DeviceCategory.Mousepad => "Mousepad",
        DeviceCategory.Cooling => "Cooling & lighting hub",
        DeviceCategory.Lighting => "Lighting controller",
        DeviceCategory.Memory => "Memory",
        DeviceCategory.PowerSupply => "Power supply",
        DeviceCategory.Receiver => "Wireless receiver",
        _ => "Device",
    };

    public static string GlyphFor(DeviceCategory category) => category switch
    {
        DeviceCategory.Keyboard => "\uE765",
        DeviceCategory.Mouse => "\uE962",
        DeviceCategory.Headset => "\uE7F6",
        DeviceCategory.Controller => "\uE7FC",
        DeviceCategory.Mousepad => "\uE81E",
        DeviceCategory.Cooling => "\uE9CA",
        DeviceCategory.Lighting => "\uE781",
        DeviceCategory.Memory => "\uE964",
        DeviceCategory.PowerSupply => "\uE945",
        DeviceCategory.Receiver => "\uE704",
        _ => "\uE772",
    };

    public static DeviceSummary FromJson(JsonObject json) => new()
    {
        Serial = json["Serial"]?.GetValue<string>() ?? "",
        Product = json["Product"]?.GetValue<string>() ?? "",
        ProductType = json["ProductType"]?.GetValue<int>() ?? -1,
        Image = json["Image"]?.GetValue<string>() ?? "",
        Firmware = json["Firmware"]?.GetValue<string>() ?? "",
        Hidden = json["Hidden"]?.GetValue<bool>() ?? false,
        Label = json["GetDevice"]?["DeviceProfile"]?["Label"]?.GetValue<string>() ?? "",
    };

    private static DeviceCategory CategoryFor(int productType, string image)
    {
        switch (image)
        {
            case "icon-dongle.svg": return DeviceCategory.Receiver;
            case "icon-keyboard.svg": return DeviceCategory.Keyboard;
            case "icon-mouse.svg": return DeviceCategory.Mouse;
            case "icon-headphone.svg": return DeviceCategory.Headset;
            case "icon-mousepad.svg": return DeviceCategory.Mousepad;
            case "icon-ram.svg": return DeviceCategory.Memory;
            case "icon-psu.svg": return DeviceCategory.PowerSupply;
        }

        return productType switch
        {
            4 or 5 => DeviceCategory.Lighting,
            8 => DeviceCategory.Memory,
            >= 0 and <= 13 => DeviceCategory.Cooling,
            >= 101 and <= 199 => DeviceCategory.Keyboard,
            >= 201 and <= 299 => DeviceCategory.Mouse,
            >= 300 and <= 399 => DeviceCategory.Headset,
            >= 401 and <= 499 => DeviceCategory.Mousepad,
            >= 501 and <= 599 => DeviceCategory.PowerSupply,
            >= 601 and <= 699 => DeviceCategory.Controller,
            997 or 998 => DeviceCategory.Receiver,
            _ => DeviceCategory.Other,
        };
    }

    public override bool Equals(object? obj) => obj is DeviceSummary other && other.Serial == Serial;
    public override int GetHashCode() => Serial.GetHashCode();
}

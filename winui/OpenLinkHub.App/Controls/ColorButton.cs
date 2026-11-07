using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;
using Microsoft.UI.Xaml.Media;
using Windows.UI;

namespace OpenLinkHub.App.Controls;

/// A swatch that opens a colour picker and reports the colour once the picker closes.
public sealed class ColorButton : Button
{
    private readonly Border _swatch = new()
    {
        Width = 32,
        Height = 20,
        CornerRadius = new CornerRadius(4),
        BorderThickness = new Thickness(1),
    };
    private readonly ColorPicker _picker = new()
    {
        IsAlphaEnabled = false,
        IsMoreButtonVisible = false,
        IsColorChannelTextInputVisible = true,
        IsHexInputVisible = true,
        ColorSpectrumShape = ColorSpectrumShape.Ring,
    };
    private Color _committed;

    public ColorButton(Color color)
    {
        Padding = new Thickness(6);
        Content = _swatch;
        _swatch.BorderBrush = (Brush)Application.Current.Resources["ControlStrokeColorDefaultBrush"];
        SetColor(color);

        var flyout = new Flyout { Content = _picker, Placement = FlyoutPlacementMode.Bottom };
        flyout.Closed += (_, _) =>
        {
            if (_picker.Color != _committed)
            {
                _committed = _picker.Color;
                _swatch.Background = new SolidColorBrush(_committed);
                ColorCommitted?.Invoke(this, _committed);
            }
        };
        Flyout = flyout;
    }

    public Color Color => _committed;

    public event EventHandler<Color>? ColorCommitted;

    public void SetColor(Color color)
    {
        _committed = color;
        _picker.Color = color;
        _swatch.Background = new SolidColorBrush(color);
    }

    public static Color Parse(string? hex)
    {
        if (string.IsNullOrEmpty(hex) || hex.Length < 7 || hex[0] != '#')
        {
            return Colors.White;
        }
        return Color.FromArgb(255,
            Convert.ToByte(hex.Substring(1, 2), 16),
            Convert.ToByte(hex.Substring(3, 2), 16),
            Convert.ToByte(hex.Substring(5, 2), 16));
    }

    public static Color FromJson(System.Text.Json.Nodes.JsonNode? node) => Color.FromArgb(255,
        (byte)(node?["red"]?.GetValue<double>() ?? 255),
        (byte)(node?["green"]?.GetValue<double>() ?? 255),
        (byte)(node?["blue"]?.GetValue<double>() ?? 255));

    public static object ToJson(Color c) => new { red = (int)c.R, green = (int)c.G, blue = (int)c.B, brightness = 1 };
}

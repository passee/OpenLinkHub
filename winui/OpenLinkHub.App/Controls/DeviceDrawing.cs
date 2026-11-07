using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Markup;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Shapes;
using OpenLinkHub.App.Models;
using Windows.UI;
using Path = Microsoft.UI.Xaml.Shapes.Path;

namespace OpenLinkHub.App.Controls;

/// Stylised top-down drawing of a device whose LED zones take their live colours.
/// OpenLinkHub ships no product images, so these stand in for iCUE's renders.
public sealed class DeviceDrawing : UserControl
{
    private readonly Canvas _canvas = new() { Width = 400, Height = 260 };
    private readonly Dictionary<int, List<Shape>> _zones = new();

    public DeviceDrawing(DeviceCategory category)
    {
        Content = new Viewbox { Child = _canvas, Stretch = Stretch.Uniform };
        switch (category)
        {
            case DeviceCategory.Mouse:
                DrawMouse();
                break;
            case DeviceCategory.Mousepad:
                DrawMousepad();
                break;
            case DeviceCategory.Headset:
                DrawHeadset();
                break;
        }
    }

    public static bool Supports(DeviceCategory category) =>
        category is DeviceCategory.Mouse or DeviceCategory.Mousepad or DeviceCategory.Headset;

    public void SetZoneColor(int zone, Color color)
    {
        if (!_zones.TryGetValue(zone, out var shapes))
        {
            return;
        }
        foreach (var shape in shapes)
        {
            if (shape.Tag as string == "glow")
            {
                shape.Fill = new SolidColorBrush(Color.FromArgb(70, color.R, color.G, color.B));
            }
            else if (shape.Fill != null)
            {
                shape.Fill = new SolidColorBrush(color);
            }
            else
            {
                shape.Stroke = new SolidColorBrush(color);
            }
        }
    }

    private static Brush Body => new LinearGradientBrush
    {
        StartPoint = new Windows.Foundation.Point(0, 0),
        EndPoint = new Windows.Foundation.Point(0, 1),
        GradientStops =
        {
            new GradientStop { Color = Color.FromArgb(255, 0x3A, 0x3A, 0x3E), Offset = 0 },
            new GradientStop { Color = Color.FromArgb(255, 0x1E, 0x1E, 0x21), Offset = 1 },
        },
    };

    private static Brush Edge => new SolidColorBrush(Color.FromArgb(255, 0x55, 0x55, 0x5A));

    private Path AddPath(string data, Brush? fill, Brush? stroke = null, double thickness = 0)
    {
        var path = new Path
        {
            Data = (Geometry)XamlBindingHelper.ConvertValue(typeof(Geometry), data),
            Fill = fill,
            Stroke = stroke,
            StrokeThickness = thickness,
            StrokeStartLineCap = PenLineCap.Round,
            StrokeEndLineCap = PenLineCap.Round,
        };
        _canvas.Children.Add(path);
        return path;
    }

    private void Zone(int zone, Shape shape)
    {
        if (!_zones.TryGetValue(zone, out var list))
        {
            _zones[zone] = list = new List<Shape>();
        }
        list.Add(shape);
    }

    private void Glow(int zone, string data)
    {
        var glow = AddPath(data, new SolidColorBrush(Color.FromArgb(70, 255, 255, 255)));
        glow.Tag = "glow";
        Zone(zone, glow);
    }

    private void DrawMouse()
    {
        // Zones: 0 logo, 1 scroll wheel, 2 front.
        Glow(2, "M135,38 C165,8 235,8 265,38 L255,50 C230,28 170,28 145,50 Z");
        AddPath("M200,18 C262,18 292,70 292,135 C292,205 255,248 200,248 C145,248 108,205 108,135 C108,70 138,18 200,18 Z", Body, Edge, 2);
        AddPath("M200,22 L200,105", null, new SolidColorBrush(Color.FromArgb(255, 0x12, 0x12, 0x14)), 3);
        AddPath("M112,118 C150,108 250,108 288,118", null, new SolidColorBrush(Color.FromArgb(255, 0x12, 0x12, 0x14)), 2);
        Zone(2, AddPath("M150,30 C180,16 220,16 250,30", null, new SolidColorBrush(Colors.White), 6));
        Zone(1, AddPath("M191,46 h18 a4,4 0 0 1 4,4 v34 a4,4 0 0 1 -4,4 h-18 a4,4 0 0 1 -4,-4 v-34 a4,4 0 0 1 4,-4 Z", new SolidColorBrush(Colors.White)));
        Glow(0, "M200,160 m-34,0 a34,34 0 1 0 68,0 a34,34 0 1 0 -68,0");
        Zone(0, AddPath("M200,160 m-20,0 a20,20 0 1 0 40,0 a20,20 0 1 0 -40,0", new SolidColorBrush(Colors.White)));
    }

    private void DrawMousepad()
    {
        // Zones follow the service's area ids: 1 left edge, 2 logo on the hub, 3 right edge.
        Glow(1, "M10,40 h16 v206 h-16 Z");
        Glow(3, "M374,40 h16 v206 h-16 Z");
        AddPath("M24,30 h352 a14,14 0 0 1 14,14 v190 a14,14 0 0 1 -14,14 h-352 a14,14 0 0 1 -14,-14 v-190 a14,14 0 0 1 14,-14 Z", Body, Edge, 2);
        Zone(1, AddPath("M14,48 v190", null, new SolidColorBrush(Colors.White), 5));
        Zone(3, AddPath("M386,48 v190", null, new SolidColorBrush(Colors.White), 5));
        AddPath("M270,12 h100 a10,10 0 0 1 10,10 v22 h-120 v-22 a10,10 0 0 1 10,-10 Z", new SolidColorBrush(Color.FromArgb(255, 0x26, 0x26, 0x29)), Edge, 2);
        Zone(2, AddPath("M300,26 h40 a3,3 0 0 1 0,6 h-40 a3,3 0 0 1 0,-6 Z", new SolidColorBrush(Colors.White)));
    }

    private void DrawHeadset()
    {
        // Zones: 0 logos on both cups, 1 microphone tip, 2 status LED.
        AddPath("M90,150 C90,40 310,40 310,150", null, new SolidColorBrush(Color.FromArgb(255, 0x2E, 0x2E, 0x32)), 18);
        AddPath("M98,150 C98,52 302,52 302,150", null, Edge, 2);
        foreach (var cx in new[] { 90.0, 310.0 })
        {
            AddPath($"M{cx},175 m-48,0 a48,58 0 1 0 96,0 a48,58 0 1 0 -96,0", Body, Edge, 2);
            Glow(0, $"M{cx},175 m-30,0 a30,30 0 1 0 60,0 a30,30 0 1 0 -60,0");
            Zone(0, AddPath($"M{cx},175 m-16,0 a16,16 0 1 0 32,0 a16,16 0 1 0 -32,0", new SolidColorBrush(Colors.White)));
        }
        AddPath("M60,200 C30,230 60,250 110,248", null, new SolidColorBrush(Color.FromArgb(255, 0x2E, 0x2E, 0x32)), 7);
        Zone(1, AddPath("M112,248 m-7,0 a7,7 0 1 0 14,0 a7,7 0 1 0 -14,0", new SolidColorBrush(Colors.White)));
        Zone(2, AddPath("M340,215 m-4,0 a4,4 0 1 0 8,0 a4,4 0 1 0 -8,0", new SolidColorBrush(Colors.White)));
    }
}

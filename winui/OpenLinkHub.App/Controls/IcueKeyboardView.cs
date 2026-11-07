using System.Text.Json.Nodes;
using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;
using Microsoft.UI.Xaml.Shapes;
using OpenLinkHub.App.Services.Icue;
using Windows.Foundation;
using Windows.UI;

namespace OpenLinkHub.App.Controls;

/// The device's iCUE render with a clickable outline over every key the service knows.
public sealed class IcueKeyboardView : UserControl, IKeySurface
{
    private readonly Canvas _canvas = new();
    // Image, glows and outlines share render-pixel coordinates; only this layer moves when the crop changes.
    private readonly Canvas _scene = new();
    private readonly Image _image = new() { Stretch = Stretch.Fill };
    private readonly Grid _glowHost = new() { IsHitTestVisible = false };
    private readonly Dictionary<int, Polygon> _shapes = new();
    private readonly Dictionary<int, KeyInfo> _keys = new();
    private readonly HashSet<int> _selected = new();
    private readonly HashSet<int> _highlighted = new();
    private int? _hovered;
    private bool _showColors;

    public IcueKeyboardView(string imagePath, Rect? clip, IReadOnlyDictionary<int, IcueItem> outlines, IEnumerable<KeyInfo> keys)
    {
        _canvas.Children.Add(_scene);
        _scene.Children.Add(_image);
        _scene.Children.Add(_glowHost);
        SetImage(imagePath, clip);

        foreach (var key in keys)
        {
            _keys[key.Id] = key;
            if (!outlines.TryGetValue(key.Id, out var outline))
            {
                continue;
            }
            var polygon = new Polygon
            {
                Points = new PointCollection(),
                StrokeLineJoin = PenLineJoin.Round,
                Tag = key.Id,
            };
            foreach (var p in outline.Polygon)
            {
                polygon.Points.Add(p);
            }
            ToolTipService.SetToolTip(polygon, string.IsNullOrWhiteSpace(key.Name) ? outline.Id : key.Name);
            polygon.PointerPressed += Shape_PointerPressed;
            polygon.PointerEntered += (_, _) => SetHover(key.Id);
            polygon.PointerExited += (_, _) => SetHover(null);
            _shapes[key.Id] = polygon;
            _scene.Children.Add(polygon);
        }

        Content = new Viewbox { Child = _canvas, Stretch = Stretch.Uniform };
        Repaint();
    }

    /// Show another render of the same device, cropped to the given area.
    public void SetImage(string imagePath, Rect? clip)
    {
        var (width, height) = ImageSize.Read(imagePath);
        var area = clip ?? new Rect(0, 0, width, height);
        _image.Source = new BitmapImage(new Uri(imagePath));
        _image.Width = _glowHost.Width = width;
        _image.Height = _glowHost.Height = height;
        _canvas.Width = area.Width;
        _canvas.Height = area.Height;
        _canvas.Clip = new RectangleGeometry { Rect = new Rect(0, 0, area.Width, area.Height) };
        Canvas.SetLeft(_scene, -area.X);
        Canvas.SetTop(_scene, -area.Y);
    }

    public bool ShowHotspots
    {
        get => _shapes.Values.FirstOrDefault()?.Visibility != Visibility.Collapsed;
        set
        {
            foreach (var shape in _shapes.Values)
            {
                shape.Visibility = value ? Visibility.Visible : Visibility.Collapsed;
            }
        }
    }

    /// Paint LED zones onto the render: each mask is a white shape the size of the render.
    public void SetGlows(IEnumerable<(string Mask, Color Color)> glows) => GlowLayer.Apply(_glowHost, glows);

    public KeySelectionMode SelectionMode { get; set; } = KeySelectionMode.None;

    public bool AssignableOnly { get; set; }

    public bool ShowColors
    {
        get => _showColors;
        set
        {
            _showColors = value;
            Repaint();
        }
    }

    public Func<KeyInfo, bool>? Selectable { get; set; }

    /// Draw every hotspot as a ring, for pictures whose buttons are small or hard to spot.
    public bool ShowMarkers { get; set; }

    public IReadOnlyCollection<int> SelectedKeys => _selected;

    public IReadOnlyDictionary<int, KeyInfo> Keys => _keys;

    public event EventHandler? SelectionChanged;

    public void ResetSelectionListeners() => SelectionChanged = null;

    public void ClearSelection()
    {
        _selected.Clear();
        Repaint();
        SelectionChanged?.Invoke(this, EventArgs.Empty);
    }

    public void SelectAll()
    {
        _selected.Clear();
        _selected.UnionWith(_shapes.Keys.Where(id => IsSelectable(_keys[id])));
        Repaint();
        SelectionChanged?.Invoke(this, EventArgs.Empty);
    }

    public void Select(int id, bool notify = true)
    {
        _selected.Clear();
        if (_shapes.ContainsKey(id))
        {
            _selected.Add(id);
        }
        Repaint();
        if (notify)
        {
            SelectionChanged?.Invoke(this, EventArgs.Empty);
        }
    }

    public void SetColor(IEnumerable<int> ids, Color color)
    {
        foreach (var id in ids)
        {
            if (_keys.TryGetValue(id, out var key) && !key.NoColor)
            {
                key.Data["color"] = new JsonObject { ["red"] = color.R, ["green"] = color.G, ["blue"] = color.B };
            }
        }
        Repaint();
    }

    private IReadOnlyDictionary<int, Color> _live = new Dictionary<int, Color>();

    public void SetLiveColors(IReadOnlyDictionary<int, Color> colors)
    {
        _live = colors;
        if (_showColors)
        {
            Repaint();
        }
    }

    public void SetHighlighted(IEnumerable<int> ids)
    {
        _highlighted.Clear();
        _highlighted.UnionWith(ids);
        Repaint();
    }

    private bool IsSelectable(KeyInfo key) =>
        Selectable?.Invoke(key) ?? (AssignableOnly ? !key.OnlyColor : !key.NoColor);

    private void SetHover(int? id)
    {
        _hovered = id;
        Repaint();
    }

    private void Shape_PointerPressed(object sender, PointerRoutedEventArgs e)
    {
        if (SelectionMode == KeySelectionMode.None || sender is not Polygon { Tag: int id } || !IsSelectable(_keys[id]))
        {
            return;
        }
        if (SelectionMode == KeySelectionMode.Single)
        {
            _selected.Clear();
            _selected.Add(id);
        }
        else if (!_selected.Remove(id))
        {
            _selected.Add(id);
        }
        Repaint();
        SelectionChanged?.Invoke(this, EventArgs.Empty);
        e.Handled = true;
    }

    private static SolidColorBrush Brush(byte a, Color c) => new(Color.FromArgb(a, c.R, c.G, c.B));

    private void Repaint()
    {
        var accent = (Color)Application.Current.Resources["SystemAccentColorLight1"];
        foreach (var (id, shape) in _shapes)
        {
            var key = _keys[id];
            var selectable = SelectionMode != KeySelectionMode.None && IsSelectable(key);
            Brush fill = new SolidColorBrush(Colors.Transparent);
            Brush? stroke = null;
            double thickness = 0;
            if (ShowMarkers)
            {
                fill = Brush(0x30, accent);
                stroke = Brush(0xD0, accent);
                thickness = 5;
            }

            if (_showColors && !key.NoColor && _live.TryGetValue(id, out var live))
            {
                fill = Brush(0x8C, live);
            }
            else if (_showColors && !key.NoColor && key.Data["color"] is JsonObject c)
            {
                fill = Brush(0x80, Color.FromArgb(255, (byte)(c["red"]?.GetValue<double>() ?? 0),
                    (byte)(c["green"]?.GetValue<double>() ?? 0), (byte)(c["blue"]?.GetValue<double>() ?? 0)));
            }
            if (_highlighted.Contains(id))
            {
                fill = Brush(0x8C, Colors.White);
                stroke = Brush(0xE6, Colors.White);
                thickness = 2;
            }
            if (selectable && _hovered == id)
            {
                fill = _highlighted.Contains(id) ? Brush(0xB0, Colors.White) : Brush(0x40, Colors.White);
            }
            if (_selected.Contains(id))
            {
                stroke = Brush(0xFF, accent);
                thickness = 6;
                if (!_showColors)
                {
                    fill = Brush(0x50, accent);
                }
            }
            shape.Fill = fill;
            shape.Stroke = stroke;
            shape.StrokeThickness = thickness;
            shape.Opacity = SelectionMode != KeySelectionMode.None && !selectable && !_showColors ? 0.6 : 1;
        }
    }
}

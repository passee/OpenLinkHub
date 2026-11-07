using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Shapes;
using Windows.Foundation;

namespace OpenLinkHub.App.Controls;

/// Drag-the-points editor for a temperature → speed curve.
public sealed class CurveEditor : Grid
{
    private const double PadLeft = 44, PadBottom = 28, PadTop = 12, PadRight = 16, HandleSize = 14;

    private readonly Canvas _canvas = new();
    private readonly List<Ellipse> _handles = new();
    private readonly Polyline _line = new() { StrokeThickness = 2.5, StrokeLineJoin = PenLineJoin.Round };
    private readonly Polygon _fill = new() { Opacity = 0.18 };
    private readonly Border _tooltip = new() { CornerRadius = new CornerRadius(4), Padding = new Thickness(6, 2, 6, 2), Visibility = Visibility.Collapsed };
    private readonly TextBlock _tooltipText = new() { FontSize = 12 };
    private int _dragIndex = -1;
    private double _markerX = double.NaN;
    private readonly Line _marker = new() { StrokeThickness = 1, StrokeDashArray = new DoubleCollection { 4, 3 }, Visibility = Visibility.Collapsed };

    public CurveEditor()
    {
        Height = 300;
        Children.Add(_canvas);
        _tooltip.Child = _tooltipText;
        SizeChanged += (_, _) => Redraw();
        ActualThemeChanged += (_, _) => Redraw();
    }

    public List<Point> Points { get; private set; } = new();
    public double MinX { get; set; }
    public double MaxX { get; set; } = 100;
    public double MinY { get; set; }
    public double MaxY { get; set; } = 100;
    public string XUnit { get; set; } = "°C";
    public string YUnit { get; set; } = "%";
    public bool IsReadOnly { get; set; }

    public event EventHandler? PointsChanged;

    public void SetPoints(IEnumerable<Point> points)
    {
        Points = points.OrderBy(p => p.X).ToList();
        Redraw();
    }

    /// Shows the live sensor value as a vertical line.
    public void SetMarker(double x)
    {
        _markerX = x;
        Redraw();
    }

    private Rect Plot => new(PadLeft, PadTop,
        Math.Max(10, ActualWidth - PadLeft - PadRight),
        Math.Max(10, ActualHeight - PadTop - PadBottom));

    private Point ToScreen(Point p)
    {
        var r = Plot;
        return new Point(
            r.X + (p.X - MinX) / (MaxX - MinX) * r.Width,
            r.Y + r.Height - (p.Y - MinY) / (MaxY - MinY) * r.Height);
    }

    private Point FromScreen(Point s)
    {
        var r = Plot;
        return new Point(
            MinX + (s.X - r.X) / r.Width * (MaxX - MinX),
            MinY + (r.Y + r.Height - s.Y) / r.Height * (MaxY - MinY));
    }

    private Brush Res(string key) => (Brush)Application.Current.Resources[key];

    private void Redraw()
    {
        _canvas.Children.Clear();
        _handles.Clear();
        if (ActualWidth <= 0)
        {
            return;
        }

        var r = Plot;
        var gridBrush = Res("ControlStrokeColorDefaultBrush");
        var labelBrush = Res("TextFillColorTertiaryBrush");

        for (var i = 0; i <= 4; i++)
        {
            var y = r.Y + r.Height * i / 4;
            _canvas.Children.Add(new Line { X1 = r.X, X2 = r.Right, Y1 = y, Y2 = y, Stroke = gridBrush, StrokeThickness = 1 });
            AddLabel($"{MaxY - (MaxY - MinY) * i / 4:0}{YUnit}", 0, y - 8, labelBrush, PadLeft - 8, TextAlignment.Right);
        }
        var stepX = (MaxX - MinX) <= 100 ? 10 : 20;
        for (var x = Math.Ceiling(MinX / stepX) * stepX; x <= MaxX; x += stepX)
        {
            var sx = ToScreen(new Point(x, MinY)).X;
            _canvas.Children.Add(new Line { X1 = sx, X2 = sx, Y1 = r.Y, Y2 = r.Bottom, Stroke = gridBrush, StrokeThickness = 1, Opacity = 0.5 });
            AddLabel($"{x:0}{XUnit}", sx - 24, r.Bottom + 6, labelBrush, 48, TextAlignment.Center);
        }

        var accent = Res("AccentFillColorDefaultBrush");
        var screen = Points.Select(ToScreen).ToList();
        _line.Stroke = accent;
        _line.Points = new PointCollection();
        _fill.Fill = accent;
        _fill.Points = new PointCollection();
        if (screen.Count > 0)
        {
            var first = new Point(r.X, screen[0].Y);
            var last = new Point(r.Right, screen[^1].Y);
            foreach (var p in screen.Prepend(first).Append(last))
            {
                _line.Points.Add(p);
                _fill.Points.Add(p);
            }
            _fill.Points.Add(new Point(r.Right, r.Bottom));
            _fill.Points.Add(new Point(r.X, r.Bottom));
        }
        _canvas.Children.Add(_fill);
        _canvas.Children.Add(_line);

        if (!double.IsNaN(_markerX) && _markerX >= MinX && _markerX <= MaxX)
        {
            var mx = ToScreen(new Point(_markerX, 0)).X;
            _marker.X1 = _marker.X2 = mx;
            _marker.Y1 = r.Y;
            _marker.Y2 = r.Bottom;
            _marker.Stroke = Res("TextFillColorSecondaryBrush");
            _marker.Visibility = Visibility.Visible;
            _canvas.Children.Add(_marker);
            AddLabel($"now {_markerX:0.#}{XUnit}", mx + 4, r.Y, Res("TextFillColorSecondaryBrush"), 100, TextAlignment.Left);
        }

        for (var i = 0; i < screen.Count; i++)
        {
            var handle = new Ellipse
            {
                Width = HandleSize,
                Height = HandleSize,
                Fill = Res("SolidBackgroundFillColorBaseBrush"),
                Stroke = accent,
                StrokeThickness = 3,
                Tag = i,
            };
            Canvas.SetLeft(handle, screen[i].X - HandleSize / 2);
            Canvas.SetTop(handle, screen[i].Y - HandleSize / 2);
            if (!IsReadOnly)
            {
                handle.PointerPressed += Handle_PointerPressed;
                handle.PointerEntered += (s, _) => ((Ellipse)s).StrokeThickness = 4.5;
                handle.PointerExited += (s, _) => ((Ellipse)s).StrokeThickness = 3;
            }
            _handles.Add(handle);
            _canvas.Children.Add(handle);
        }

        _tooltip.Background = Res("SolidBackgroundFillColorTertiaryBrush");
        _canvas.Children.Add(_tooltip);
    }

    private void AddLabel(string text, double x, double y, Brush brush, double width, TextAlignment align)
    {
        var label = new TextBlock { Text = text, FontSize = 11, Foreground = brush, Width = width, TextAlignment = align };
        Canvas.SetLeft(label, x);
        Canvas.SetTop(label, y);
        _canvas.Children.Add(label);
    }

    private void Handle_PointerPressed(object sender, PointerRoutedEventArgs e)
    {
        _dragIndex = (int)((Ellipse)sender).Tag;
        _canvas.CapturePointer(e.Pointer);
        _canvas.PointerMoved += Canvas_PointerMoved;
        _canvas.PointerReleased += Canvas_PointerReleased;
        _canvas.PointerCaptureLost += Canvas_PointerReleased;
        e.Handled = true;
    }

    private void Canvas_PointerMoved(object sender, PointerRoutedEventArgs e)
    {
        if (_dragIndex < 0)
        {
            return;
        }

        var value = FromScreen(e.GetCurrentPoint(_canvas).Position);
        var lowX = _dragIndex > 0 ? Points[_dragIndex - 1].X + 1 : MinX;
        var highX = _dragIndex < Points.Count - 1 ? Points[_dragIndex + 1].X - 1 : MaxX;
        var x = Math.Round(Math.Clamp(value.X, lowX, highX));
        var y = Math.Round(Math.Clamp(value.Y, MinY, MaxY));
        Points[_dragIndex] = new Point(x, y);

        var index = _dragIndex;
        Redraw();
        _dragIndex = index;
        var s = ToScreen(Points[index]);
        _tooltipText.Text = $"{x:0}{XUnit} → {y:0}{YUnit}";
        _tooltip.Visibility = Visibility.Visible;
        Canvas.SetLeft(_tooltip, s.X + 10);
        Canvas.SetTop(_tooltip, s.Y - 30);
    }

    private void Canvas_PointerReleased(object sender, PointerRoutedEventArgs e)
    {
        _canvas.PointerMoved -= Canvas_PointerMoved;
        _canvas.PointerReleased -= Canvas_PointerReleased;
        _canvas.PointerCaptureLost -= Canvas_PointerReleased;
        _canvas.ReleasePointerCaptures();
        var changed = _dragIndex >= 0;
        _dragIndex = -1;
        _tooltip.Visibility = Visibility.Collapsed;
        if (changed)
        {
            PointsChanged?.Invoke(this, EventArgs.Empty);
        }
    }
}

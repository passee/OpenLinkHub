using System.Text.Json.Nodes;
using Microsoft.UI;
using Microsoft.UI.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Windows.UI;

namespace OpenLinkHub.App.Controls;

public enum KeySelectionMode
{
    None,
    Single,
    Multiple,
}

public sealed record KeyInfo(int Id, string Name, JsonObject Data)
{
    public bool OnlyColor => Data["onlyColor"]?.GetValue<bool>() == true;
    public bool NoColor => Data["noColor"]?.GetValue<bool>() == true;
}

/// Draws a keyboard from the service's layout JSON. Each row flows left to right:
/// a key's "left" is the gap after the previous key and "top" its offset inside the row.
public sealed class KeyboardView : UserControl, IKeySurface
{
    private const double SideLedWidth = 20;
    private const double MissingKeySpace = 122;

    private readonly Canvas _canvas = new();
    private readonly Dictionary<int, Border> _keys = new();
    private readonly Dictionary<int, KeyInfo> _info = new();
    private readonly HashSet<int> _selected = new();

    public KeyboardView()
    {
        Content = new Viewbox { Child = _canvas, Stretch = Stretch.Uniform, StretchDirection = StretchDirection.Both };
    }

    public KeySelectionMode SelectionMode { get; set; } = KeySelectionMode.Multiple;

    public bool AssignableOnly { get; set; }

    public bool ShowColors { get; set; } = true;

    public Func<KeyInfo, bool>? Selectable { get; set; }

    private readonly HashSet<int> _highlighted = new();

    public void SetLiveColors(IReadOnlyDictionary<int, Color> colors)
    {
        foreach (var (id, color) in colors)
        {
            if (_keys.TryGetValue(id, out var border) && _info.TryGetValue(id, out var info) && !info.NoColor && !_selected.Contains(id))
            {
                border.BorderBrush = new SolidColorBrush(color);
            }
        }
    }

    public void SetHighlighted(IEnumerable<int> ids)
    {
        _highlighted.Clear();
        _highlighted.UnionWith(ids);
        ApplySelectionVisuals();
    }

    public IReadOnlyCollection<int> SelectedKeys => _selected;

    public IReadOnlyDictionary<int, KeyInfo> Keys => _info;

    public event EventHandler? SelectionChanged;

    public sealed record PlacedKey(KeyInfo Info, Windows.Foundation.Rect? Rect);

    /// Every key of the layout with its drawn position; side LEDs have no position.
    public static List<PlacedKey> Place(JsonObject layout)
    {
        var placed = new List<PlacedKey>();
        var rows = (layout["row"] as JsonObject ?? new JsonObject()).OrderBy(r => int.Parse(r.Key)).ToList();
        double y = 0;
        foreach (var (_, rowNode) in rows)
        {
            if (rowNode?["keys"] is not JsonObject keys)
            {
                continue;
            }

            double x = 0, rowHeight = 0;
            var previousId = -1;
            foreach (var (idText, keyNode) in keys.OrderBy(k => int.Parse(k.Key)))
            {
                if (keyNode is not JsonObject key)
                {
                    continue;
                }
                var id = int.Parse(idText);
                // ISO layouts leave out the upper half of Enter; its id is skipped, and the space it took must stay.
                if (previousId >= 0 && id - previousId > 1)
                {
                    x += MissingKeySpace;
                }
                previousId = id;
                var width = key["width"]?.GetValue<double>() ?? 65;
                var height = key["height"]?.GetValue<double>() ?? 70;
                var left = key["left"]?.GetValue<double>() ?? 0;
                var top = key["top"]?.GetValue<double>() ?? 0;
                var info = new KeyInfo(id, key["keyName"]?.GetValue<string>() ?? "", key);
                if (left >= 1000)
                {
                    placed.Add(new PlacedKey(info, null));
                    continue;
                }

                x += left;
                if (info.OnlyColor && width <= SideLedWidth)
                {
                    placed.Add(new PlacedKey(info, null));
                }
                else
                {
                    placed.Add(new PlacedKey(info, new Windows.Foundation.Rect(x, y + top, width, height)));
                    rowHeight = Math.Max(rowHeight, top + Math.Min(height, 70));
                }
                x += width;
            }
            y += rowHeight;
        }
        return placed;
    }

    public void Load(JsonObject layout)
    {
        _canvas.Children.Clear();
        _keys.Clear();
        _info.Clear();

        double maxX = 0, maxY = 0;
        foreach (var (info, rect) in Place(layout))
        {
            _info[info.Id] = info;
            if (rect is { } r)
            {
                AddKey(info, r.X, r.Y, r.Width, r.Height);
                maxX = Math.Max(maxX, r.Right);
                maxY = Math.Max(maxY, r.Y + Math.Min(r.Height, 70));
            }
        }

        _canvas.Width = maxX;
        _canvas.Height = maxY + 10;
        ApplySelectionVisuals();
    }

    public void UpdateColors(JsonObject layout)
    {
        foreach (var (_, row) in layout["row"] as JsonObject ?? new JsonObject())
        {
            foreach (var (idText, key) in row?["keys"] as JsonObject ?? new JsonObject())
            {
                if (int.TryParse(idText, out var id) && _keys.TryGetValue(id, out var border) && key is JsonObject data)
                {
                    _info[id] = _info[id] with { Data = data };
                    Paint(border, _info[id]);
                }
            }
        }
    }

    public void SetColor(IEnumerable<int> ids, Color color)
    {
        foreach (var id in ids)
        {
            if (_keys.TryGetValue(id, out var border) && _info.TryGetValue(id, out var info) && !info.NoColor)
            {
                info.Data["color"] = new JsonObject { ["red"] = color.R, ["green"] = color.G, ["blue"] = color.B };
                Paint(border, info);
            }
        }
    }

    public void SelectAll()
    {
        _selected.Clear();
        foreach (var (id, info) in _info)
        {
            if (_keys.ContainsKey(id) && IsSelectable(info))
            {
                _selected.Add(id);
            }
        }
        ApplySelectionVisuals();
        SelectionChanged?.Invoke(this, EventArgs.Empty);
    }

    /// Pages rebuild their panels per tab, so listeners from the previous tab must go.
    public void ResetSelectionListeners() => SelectionChanged = null;

    public void ClearSelection()
    {
        _selected.Clear();
        ApplySelectionVisuals();
        SelectionChanged?.Invoke(this, EventArgs.Empty);
    }

    public void Select(int id, bool notify = true)
    {
        _selected.Clear();
        _selected.Add(id);
        ApplySelectionVisuals();
        if (notify)
        {
            SelectionChanged?.Invoke(this, EventArgs.Empty);
        }
    }

    private bool IsSelectable(KeyInfo info) => Selectable?.Invoke(info) ?? (AssignableOnly ? !info.OnlyColor : !info.NoColor);

    private void AddKey(KeyInfo info, double x, double y, double width, double height)
    {
        var label = new TextBlock
        {
            Text = info.Name,
            FontSize = info.Name.Length > 5 ? 14 : 19,
            FontWeight = FontWeights.SemiBold,
            HorizontalAlignment = HorizontalAlignment.Center,
            VerticalAlignment = VerticalAlignment.Center,
            TextTrimming = TextTrimming.CharacterEllipsis,
            Foreground = new SolidColorBrush(Colors.White),
            MaxWidth = width - 6,
        };
        var border = new Border
        {
            Width = width,
            Height = height,
            CornerRadius = new CornerRadius(height <= 24 ? 3 : 7),
            BorderThickness = new Thickness(2),
            Child = height > 24 ? label : null,
            Tag = info.Id,
        };
        ToolTipService.SetToolTip(border, string.IsNullOrEmpty(info.Name) ? $"LED {info.Id}" : info.Name);
        Canvas.SetLeft(border, x);
        Canvas.SetTop(border, y);
        border.PointerPressed += Key_PointerPressed;
        border.PointerEntered += (_, _) => border.Opacity = 0.85;
        border.PointerExited += (_, _) => border.Opacity = 1;
        _keys[info.Id] = border;
        _canvas.Children.Add(border);
        Paint(border, info);
    }

    private static Color KeyColor(KeyInfo info)
    {
        var c = info.Data["color"];
        return Color.FromArgb(255,
            (byte)(c?["red"]?.GetValue<double>() ?? 0),
            (byte)(c?["green"]?.GetValue<double>() ?? 0),
            (byte)(c?["blue"]?.GetValue<double>() ?? 0));
    }

    private void Paint(Border border, KeyInfo info)
    {
        if (info.NoColor)
        {
            border.Background = new SolidColorBrush(Color.FromArgb(255, 0x2B, 0x2B, 0x2B));
            border.BorderBrush = new SolidColorBrush(Color.FromArgb(255, 0x3A, 0x3A, 0x3A));
            return;
        }
        var color = KeyColor(info);
        border.Background = new SolidColorBrush(Color.FromArgb(255,
            (byte)(0x1C + color.R * 0.32), (byte)(0x1C + color.G * 0.32), (byte)(0x1C + color.B * 0.32)));
        if (!_selected.Contains(info.Id))
        {
            border.BorderBrush = new SolidColorBrush(color);
        }
    }

    private void ApplySelectionVisuals()
    {
        foreach (var (id, border) in _keys)
        {
            var info = _info[id];
            var selected = _selected.Contains(id);
            border.BorderThickness = new Thickness(selected ? 4 : _highlighted.Contains(id) ? 3 : 2);
            if (_highlighted.Contains(id) && !selected)
            {
                border.Background = new SolidColorBrush(Color.FromArgb(255, 0x8A, 0x8A, 0x8A));
            }
            if (selected)
            {
                border.BorderBrush = new SolidColorBrush(Colors.White);
            }
            else
            {
                Paint(border, info);
            }
            border.Opacity = SelectionMode != KeySelectionMode.None && !IsSelectable(info) ? 0.35 : 1;
        }
    }

    private void Key_PointerPressed(object sender, PointerRoutedEventArgs e)
    {
        if (SelectionMode == KeySelectionMode.None || sender is not Border { Tag: int id } || !IsSelectable(_info[id]))
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
        ApplySelectionVisuals();
        SelectionChanged?.Invoke(this, EventArgs.Empty);
        e.Handled = true;
    }
}

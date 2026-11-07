using System.Numerics;
using System.Runtime.CompilerServices;
using Microsoft.UI.Composition;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Hosting;
using Microsoft.UI.Xaml.Media;
using Windows.UI;

namespace OpenLinkHub.App.Controls;

/// Colours white glow masks and stacks them over a render, the way iCUE shows lighting.
/// Live effects call this several times a second, so unchanged masks only get a new colour.
public static class GlowLayer
{
    private sealed class State
    {
        public List<string> Masks = new();
        public List<CompositionColorBrush> Colors = new();
    }

    private static readonly Dictionary<string, LoadedImageSurface> Surfaces = new(StringComparer.OrdinalIgnoreCase);
    private static readonly ConditionalWeakTable<FrameworkElement, State> States = new();

    public static void Apply(FrameworkElement host, IEnumerable<(string Mask, Color Color)> glows)
    {
        var list = glows.ToList();
        var state = States.GetOrCreateValue(host);
        if (state.Masks.SequenceEqual(list.Select(g => g.Mask)))
        {
            for (var i = 0; i < list.Count; i++)
            {
                state.Colors[i].Color = list[i].Color;
            }
            return;
        }

        var compositor = ElementCompositionPreview.GetElementVisual(host).Compositor;
        var size = new Vector2((float)host.Width, (float)host.Height);
        var container = compositor.CreateContainerVisual();
        container.Size = size;
        state.Masks = list.Select(g => g.Mask).ToList();
        state.Colors = new List<CompositionColorBrush>();

        foreach (var (mask, color) in list)
        {
            if (!Surfaces.TryGetValue(mask, out var surface))
            {
                Surfaces[mask] = surface = LoadedImageSurface.StartLoadFromUri(new Uri(mask));
            }
            var shape = compositor.CreateSurfaceBrush(surface);
            shape.Stretch = CompositionStretch.Fill;
            var fill = compositor.CreateColorBrush(color);
            state.Colors.Add(fill);
            var brush = compositor.CreateMaskBrush();
            brush.Source = fill;
            brush.Mask = shape;
            var sprite = compositor.CreateSpriteVisual();
            sprite.Size = size;
            sprite.Brush = brush;
            container.Children.InsertAtTop(sprite);
        }
        ElementCompositionPreview.SetElementChildVisual(host, container);
    }
}

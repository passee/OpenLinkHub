using Microsoft.UI.Xaml;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services;
using OpenLinkHub.App.Services.Icue;

namespace OpenLinkHub.App.Controls;

/// A keyboard to click when choosing which key an action should press, like iCUE's remap view.
/// It shows the user's own keyboard; keys that type nothing a PC knows stay unclickable.
public sealed class KeyboardPicker
{
    private static Task<(KeyboardSurfaces.Parts Parts, Dictionary<int, string> Actions)?>? _source;

    private readonly Dictionary<int, string> _keyToAction;
    private readonly Dictionary<string, int> _actionToKey;

    private KeyboardPicker(UIElement view, IKeySurface surface, Dictionary<int, string> keyToAction)
    {
        View = view;
        Surface = surface;
        _keyToAction = keyToAction;
        _actionToKey = keyToAction.GroupBy(kv => kv.Value).ToDictionary(g => g.Key, g => g.First().Key);
        surface.Selectable = key => _keyToAction.ContainsKey(key.Id);
        surface.SelectionMode = KeySelectionMode.Single;
        surface.SelectionChanged += (_, _) =>
        {
            if (Surface.SelectedKeys.FirstOrDefault() is var id && _keyToAction.TryGetValue(id, out var action))
            {
                ActionPicked?.Invoke(this, action);
            }
        };
    }

    public UIElement View { get; }

    public IKeySurface Surface { get; }

    /// The keyboard action index (the value of the "Keyboard" action) the clicked key stands for.
    public event EventHandler<string>? ActionPicked;

    public void Show(string? actionIndex)
    {
        if (actionIndex != null && _actionToKey.TryGetValue(actionIndex, out var key))
        {
            Surface.Select(key, notify: false);
        }
        else
        {
            Surface.ClearSelection();
        }
    }

    public static async Task<KeyboardPicker?> CreateAsync(double maxHeight)
    {
        var source = await (_source ??= LoadSourceAsync());
        if (source is not { } s)
        {
            return null;
        }
        var (view, surface) = KeyboardSurfaces.Create(s.Parts, maxHeight);
        return new KeyboardPicker(view, surface, s.Actions);
    }

    private static async Task<(KeyboardSurfaces.Parts, Dictionary<int, string>)?> LoadSourceAsync()
    {
        try
        {
            var devices = await App.Api.GetDevicesAsync();
            foreach (var device in devices.Where(d => d.Category == DeviceCategory.Keyboard))
            {
                var detail = await App.Api.GetDeviceAsync(device.Serial);
                if (await KeyboardSurfaces.LoadAsync(device, detail) is not { } parts)
                {
                    continue;
                }
                var byCode = await InputCatalog.KeyboardByCodeAsync();
                var actions = new Dictionary<int, string>();
                foreach (var placed in KeyboardView.Place(parts.Layout))
                {
                    if (KeyboardSurfaces.KeyName(parts, placed.Info) is { } name
                        && KeyCodes.HidUsage.TryGetValue(name, out var usage)
                        && KeyCodes.HidToEvdev.TryGetValue(usage, out var code)
                        && byCode.TryGetValue(code, out var action))
                    {
                        actions[placed.Info.Id] = action;
                    }
                }
                return (parts, actions);
            }
        }
        catch (Exception ex)
        {
            App.Log(ex);
        }
        _source = null;
        return null;
    }
}

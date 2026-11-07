using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using OpenLinkHub.App.Controls;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private const int TypeNone = 0, TypeMedia = 1, TypeKeyboard = 3, TypeController = 4, TypeMouse = 9, TypeMacro = 10;

    private JsonObject? AssignmentTypes() => _detail?["KeyAssignmentTypes"] as JsonObject;

    private bool HasKeys() => AssignmentTypes() != null
        && (_keyboard != null || _detail?["KeyAssignment"] is JsonObject);

    private IEnumerable<UIElement> KeysSection()
    {
        if (_keyboard != null)
        {
            var host = new StackPanel { Spacing = 4 };
            var thing = IsController ? "button" : "key";
            host.Children.Add(Hint($"Click a {thing} above to change what it does. Highlighted {thing}s already do something else."));
            _keyEditorHost = host;
            _keyboard.SelectionChanged += (_, _) =>
            {
                if (_keyboard.SelectedKeys.FirstOrDefault() is var id && id != 0)
                {
                    OpenKeyEditor(id);
                }
            };
            yield return host;
            yield return Header(IsController ? "All buttons" : "Changed keys");
            var changed = new StackPanel { Spacing = 4 };
            _ = FillChangedKeysAsync(changed);
            yield return changed;
            yield break;
        }

        yield return Header("Buttons");
        var grid = new Grid { ColumnSpacing = 8, RowSpacing = 8, HorizontalAlignment = HorizontalAlignment.Left };
        var editorHost = new StackPanel { Spacing = 4, Margin = new Thickness(0, 12, 0, 0) };
        _ = FillMouseButtonsAsync(grid, editorHost);
        yield return grid;
        yield return editorHost;
    }

    private StackPanel? _keyEditorHost;

    private void OpenKeyEditor(int id)
    {
        if (_keyEditorHost == null || _keyboard == null || !_keyboard.Keys.TryGetValue(id, out var key))
        {
            return;
        }
        _keyEditorHost.Children.Clear();
        _keyEditorHost.Children.Add(Header(key.Name));
        var editor = new AssignmentEditor(this, key.Data, KeyIndexFor(id), KeyEndpoint);
        editor.Saved += (_, _) => RefreshKeyHighlights();
        _keyEditorHost.Children.Add(editor.Build());
    }

    private void RefreshKeyHighlights() => _keyboard?.SetHighlighted(_keyboard.Keys.Values
        .Where(k => !k.OnlyColor && k.Data["default"]?.GetValue<bool>() == false)
        .Select(k => k.Id));

    private async Task FillChangedKeysAsync(StackPanel host)
    {
        foreach (var (id, key) in _keyboard!.Keys.OrderBy(k => k.Key))
        {
            if (key.OnlyColor || !IsController && key.Data["default"]?.GetValue<bool>() != false)
            {
                continue;
            }
            var card = new SettingsCard
            {
                Header = key.Name,
                Description = await DescribeAsync(key.Data),
                HeaderIcon = new FontIcon { Glyph = "\uE765" },
                IsClickEnabled = true,
            };
            card.Click += (_, _) =>
            {
                _keyboard.Select(id, notify: false);
                OpenKeyEditor(id);
            };
            host.Children.Add(card);
        }
        if (host.Children.Count == 0)
        {
            host.Children.Add(Hint("Every key does what is printed on it."));
        }
    }

    private static string ActionGlyph(JsonObject key) => key["default"]?.GetValue<bool>() != false
        ? ""
        : (key["actionType"]?.GetValue<int>() ?? 0) switch
        {
            TypeNone => "\uE711",
            TypeMedia => "\uE768",
            TypeKeyboard => "\uE765",
            TypeMouse => "\uE962",
            TypeMacro => "\uE8FD",
            2 or 4 or 24 => "\uE9D9",
            _ => "\uE945",
        };

    /// iCUE's two-column button list: a tile per button showing what it is set to.
    private async Task FillMouseButtonsAsync(Grid grid, StackPanel editorHost)
    {
        var buttons = (_detail?["KeyAssignment"] as JsonObject ?? new JsonObject())
            .Where(b => b.Value is JsonObject)
            .OrderBy(b => int.Parse(b.Key))
            .ToList();
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(260) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(260) });
        var tiles = new List<Button>();

        for (var i = 0; i < buttons.Count; i++)
        {
            var (id, node) = buttons[i];
            var button = (JsonObject)node!;
            if (i % 2 == 0)
            {
                grid.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
            }

            var icon = new FontIcon { Glyph = ActionGlyph(button), FontSize = 16 };
            var iconBox = new Border
            {
                Width = 36,
                Height = 36,
                CornerRadius = new CornerRadius(4),
                Background = (Brush)Application.Current.Resources["SubtleFillColorSecondaryBrush"],
                Child = icon,
            };
            var subtitle = new TextBlock
            {
                Text = await DescribeAsync(button),
                Style = (Style)Application.Current.Resources["CaptionTextBlockStyle"],
                Opacity = 0.75,
                TextTrimming = TextTrimming.CharacterEllipsis,
            };
            var tile = new Button
            {
                HorizontalAlignment = HorizontalAlignment.Stretch,
                HorizontalContentAlignment = HorizontalAlignment.Left,
                Padding = new Thickness(10, 8, 10, 8),
                Content = new StackPanel
                {
                    Orientation = Orientation.Horizontal,
                    Spacing = 12,
                    Children =
                    {
                        iconBox,
                        new StackPanel
                        {
                            VerticalAlignment = VerticalAlignment.Center,
                            Children =
                            {
                                new TextBlock { Text = Str(button, "name") ?? $"Button {id}", Style = (Style)Application.Current.Resources["BodyStrongTextBlockStyle"] },
                                subtitle,
                            },
                        },
                    },
                },
            };
            Grid.SetRow(tile, i / 2);
            Grid.SetColumn(tile, i % 2);
            grid.Children.Add(tile);
            tiles.Add(tile);

            tile.Click += (_, _) =>
            {
                foreach (var t in tiles)
                {
                    t.Style = (Style)Application.Current.Resources[t == tile ? "AccentButtonStyle" : "DefaultButtonStyle"];
                }
                editorHost.Children.Clear();
                editorHost.Children.Add(Header(Str(button, "name") ?? $"Button {id}"));
                var editor = new AssignmentEditor(this, button, int.Parse(id), "api/mouse/updateKeyAssignment");
                editor.Saved += async (_, _) =>
                {
                    subtitle.Text = await DescribeAsync(button);
                    icon.Glyph = ActionGlyph(button);
                };
                editorHost.Children.Add(editor.Build());
            };
        }
    }

    private async Task<string> DescribeAsync(JsonObject key)
    {
        if (key["default"]?.GetValue<bool>() != false)
        {
            return "Default";
        }
        var type = key["actionType"]?.GetValue<int>() ?? 0;
        var typeName = Str(AssignmentTypes(), type.ToString()) ?? $"Action {type}";
        var command = key["actionCommand"]?.GetValue<int>() ?? 0;
        var value = type switch
        {
            TypeMedia or TypeKeyboard or TypeMouse or TypeController => (await InputCatalog.NameAsync(command)).Replace("(Controller) ", ""),
            TypeMacro => Str((await App.Api.GetAsync($"api/macro/{command}"))?["data"] as JsonObject, "name"),
            _ => Str(key, "deviceId") is { Length: > 0 } s ? await DeviceNameAsync(s) : null,
        };
        var hold = key["actionHold"]?.GetValue<bool>() == true ? " (hold)" : "";
        return value == null ? typeName + hold : $"{typeName}: {value}{hold}";
    }

    private Dictionary<string, string>? _deviceNames;

    private async Task<string> DeviceNameAsync(string serialOrName)
    {
        _deviceNames ??= (await App.Api.GetDevicesAsync()).ToDictionary(d => d.Serial, d => d.DisplayName);
        return _deviceNames.GetValueOrDefault(serialOrName, serialOrName);
    }

    /// Choose an action for one key or button. Some action types take a device serial or profile
    /// name instead of a number, and which ones differs between keyboards and mice.
    private sealed class AssignmentEditor
    {
        private readonly DevicePage _page;
        private readonly JsonObject _key;
        private readonly long _keyIndex;
        private readonly string _endpoint;
        private readonly ComboBox _type = new() { Header = "Action", MinWidth = 240 };
        private readonly ComboBox _value = new() { Header = "Value", MinWidth = 280, IsTextSearchEnabled = true };
        private readonly ToggleSwitch _hold = new() { Header = "Hold", OnContent = "While held down", OffContent = "Once per press" };

        public AssignmentEditor(DevicePage page, JsonObject key, long keyIndex, string endpoint)
        {
            _page = page;
            _key = key;
            _keyIndex = keyIndex;
            _endpoint = endpoint;
        }

        public event EventHandler? Saved;

        private bool IsKeyboard => _page._device!.Category == DeviceCategory.Keyboard;

        private bool UsesString(int type) => IsKeyboard ? type is 2 or 4 or 8 or 21 or 23 or 40 : type is 23 or 40;

        public UIElement Build()
        {
            var types = (_page.AssignmentTypes() ?? new JsonObject())
                .Select(t => new Choice(t.Key, t.Value?.GetValue<string>() ?? t.Key))
                .Where(c => c.Value != "0")
                .Prepend(new Choice("default", "Default (what is printed on it)"))
                .ToList();
            _type.ItemsSource = types;
            var isDefault = _key["default"]?.GetValue<bool>() != false;
            var currentType = isDefault ? "default" : (_key["actionType"]?.GetValue<int>() ?? 0).ToString();
            _type.SelectedItem = types.FirstOrDefault(t => t.Value == currentType) ?? types[0];
            _hold.IsOn = _key["actionHold"]?.GetValue<bool>() == true;
            _ = LoadValuesAsync(preselect: true);
            _type.SelectionChanged += async (_, _) => await LoadValuesAsync(preselect: false);

            var save = new Button { Content = "Save", Style = (Style)Application.Current.Resources["AccentButtonStyle"], VerticalAlignment = VerticalAlignment.Bottom };
            save.Click += async (_, _) => await SaveAsync();

            _value.SelectionChanged += (_, _) => _picker?.Show((_value.SelectedItem as Choice)?.Value);

            return new Border
            {
                Style = (Style)Application.Current.Resources["CardBorderStyle"],
                Child = new StackPanel
                {
                    Spacing = 16,
                    Children =
                    {
                        new StackPanel { Orientation = Orientation.Horizontal, Spacing = 16, Children = { _type, _value, _hold, save } },
                        _pickerHost,
                    },
                },
            };
        }

        private readonly StackPanel _pickerHost = new() { Spacing = 8, Visibility = Visibility.Collapsed };
        private KeyboardPicker? _picker;

        /// Choosing "Keyboard" shows the user's keyboard to click the key to send, as iCUE does.
        private async Task ShowPickerAsync(bool visible)
        {
            _pickerHost.Visibility = visible ? Visibility.Visible : Visibility.Collapsed;
            if (!visible || _picker != null)
            {
                _picker?.Show((_value.SelectedItem as Choice)?.Value);
                return;
            }
            _picker = await KeyboardPicker.CreateAsync(300);
            if (_picker == null)
            {
                _pickerHost.Visibility = Visibility.Collapsed;
                return;
            }
            _picker.ActionPicked += (_, action) =>
                _value.SelectedItem = (_value.ItemsSource as IEnumerable<Choice>)?.FirstOrDefault(c => c.Value == action);
            _pickerHost.Children.Add(new TextBlock
            {
                Text = "Click the key it should press",
                Foreground = (Brush)Application.Current.Resources["TextFillColorSecondaryBrush"],
            });
            _pickerHost.Children.Add(_picker.View);
            _picker.Show((_value.SelectedItem as Choice)?.Value);
        }

        private int SelectedType => _type.SelectedItem is Choice { Value: var v } && int.TryParse(v, out var n) ? n : TypeNone;

        private async Task LoadValuesAsync(bool preselect)
        {
            var type = SelectedType;
            var isDefault = (_type.SelectedItem as Choice)?.Value == "default";
            IReadOnlyList<Choice> values = isDefault ? Array.Empty<Choice>() : await ValuesForAsync(type);
            _value.ItemsSource = values;
            _value.Visibility = values.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
            _hold.Visibility = !isDefault && type is TypeKeyboard or TypeMouse or TypeMedia or TypeController or 8 ? Visibility.Visible : Visibility.Collapsed;
            if (preselect)
            {
                var current = UsesString(type) ? Str(_key, "deviceId") : (_key["actionCommand"]?.GetValue<int>() ?? 0).ToString();
                _value.SelectedItem = values.FirstOrDefault(v => v.Value == current);
            }
            await ShowPickerAsync(!isDefault && type == TypeKeyboard);
        }

        private async Task<IReadOnlyList<Choice>> ValuesForAsync(int type)
        {
            switch (type)
            {
                case TypeMedia:
                    return await InputCatalog.MediaAsync();
                case TypeKeyboard:
                    return await InputCatalog.KeyboardAsync();
                case TypeMouse:
                    return await InputCatalog.MouseAsync();
                case TypeController when _page.IsController:
                    return await InputCatalog.ControllerAsync();
                case TypeMacro:
                    return Map(await App.Api.GetAsync("api/macro/"), v => v?["name"]?.GetValue<string>());
                case 2 or 4 or 8 when IsKeyboard:
                    return Map(await App.Api.GetAsync("api/devices/mouse"), v => v?.GetValue<string>());
                case 40:
                    return Map(await App.Api.GetAsync("api/devices/headsets"), v => v?.GetValue<string>());
                case 21:
                    var profiles = (await App.Api.GetAsync($"api/device/profiles/{_page._device!.Serial}"))?["data"] as JsonArray;
                    return profiles?.Select(p => p?.GetValue<string>() ?? "").Select(p => new Choice(p, p == "default" ? "Default" : p)).ToList()
                           ?? new List<Choice>();
                case 23:
                    return Map(await App.Api.GetAsync("api/application/manualProfiles"), v => v?["applicationName"]?.GetValue<string>());
                default:
                    return Array.Empty<Choice>();
            }
        }

        private static IReadOnlyList<Choice> Map(JsonNode? response, Func<JsonNode?, string?> label) =>
            (response?["data"] as JsonObject ?? new JsonObject())
            .Select(kv => new Choice(kv.Key, label(kv.Value) ?? kv.Key))
            .OrderBy(c => c.Label, StringComparer.OrdinalIgnoreCase)
            .ToList();

        private async Task SaveAsync()
        {
            var isDefault = (_type.SelectedItem as Choice)?.Value == "default";
            var type = isDefault ? TypeNone : SelectedType;
            var value = _value.SelectedItem as Choice;
            if (!isDefault && _value.Visibility == Visibility.Visible && value == null)
            {
                App.Window?.ShowToast("Choose a value first", false);
                return;
            }

            var number = !UsesString(type) && value != null && int.TryParse(value.Value, out var n) ? n : 0;
            var text = UsesString(type) ? value?.Value ?? "" : "";
            var hold = _hold.Visibility == Visibility.Visible && _hold.IsOn;
            var result = await App.Api.PostAsync(_endpoint, new
            {
                deviceId = _page._device!.Serial,
                keyIndex = _keyIndex,
                enabled = isDefault,
                pressAndHold = hold,
                onRelease = false,
                keyAssignmentOriginal = false,
                keyAssignmentModifier = 0,
                keyAssignmentType = type,
                keyAssignmentValue = number,
                keyAssignmentValueString = text,
                toggleDelay = 30,
            });
            Report(result);
            if (!result.Ok)
            {
                return;
            }
            _key["default"] = isDefault;
            _key["actionType"] = type;
            _key["actionCommand"] = number;
            _key["deviceId"] = text;
            _key["actionHold"] = hold;
            Saved?.Invoke(this, EventArgs.Empty);
        }
    }
}

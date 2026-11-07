using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services;

namespace OpenLinkHub.App.Views;

public sealed partial class MacrosPage : Page
{
    private const int KeyStep = 3, MouseStep = 9, PauseStep = 5, TextStep = 6, MoveStep = 20, LaunchStep = 30, MediaStep = 1;

    private JsonObject? _macro;
    private int? _macroId;
    private bool _loading;
    private readonly DispatcherTimer _repeatDebounce = new() { Interval = TimeSpan.FromMilliseconds(500) };

    public MacrosPage()
    {
        InitializeComponent();
        _repeatDebounce.Tick += async (_, _) =>
        {
            _repeatDebounce.Stop();
            await SaveRepeatAsync();
        };
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        await LoadListAsync(e.Parameter as int?);
        AddKind_Changed(AddKind, null!);
    }

    private async Task LoadListAsync(int? select = null)
    {
        var data = (await App.Api.GetAsync("api/macro/"))?["data"] as JsonObject ?? new JsonObject();
        var items = data
            .Select(kv => new Choice(kv.Key, kv.Value?["name"]?.GetValue<string>() ?? kv.Key))
            .OrderBy(c => c.Label, StringComparer.OrdinalIgnoreCase)
            .ToList();
        MacroList.ItemsSource = items;
        var target = select?.ToString() ?? _macroId?.ToString();
        MacroList.SelectedItem = items.FirstOrDefault(i => i.Value == target);
        if (MacroList.SelectedItem == null)
        {
            ShowEditor(false);
        }
    }

    private void ShowEditor(bool visible)
    {
        Editor.Visibility = visible ? Visibility.Visible : Visibility.Collapsed;
        EmptyText.Visibility = visible ? Visibility.Collapsed : Visibility.Visible;
    }

    private async void MacroList_SelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (MacroList.SelectedItem is Choice choice)
        {
            _macroId = int.Parse(choice.Value);
            await LoadMacroAsync();
        }
    }

    private async Task LoadMacroAsync()
    {
        if (_macroId == null)
        {
            return;
        }
        _macro = (await App.Api.GetAsync($"api/macro/{_macroId}"))?["data"] as JsonObject;
        if (_macro == null)
        {
            ShowEditor(false);
            return;
        }

        ShowEditor(true);
        MacroTitle.Text = _macro["name"]?.GetValue<string>();

        _loading = true;
        var repeat = _macro["repeat"]?.GetValue<int>() ?? 0;
        RepeatMode.SelectedIndex = repeat switch { < 0 => 2, 0 => 0, _ => 1 };
        RepeatCount.Value = Math.Max(1, repeat);
        RepeatDelay.Value = _macro["repeatDelay"]?.GetValue<int>() ?? 0;
        UpdateRepeatVisibility();
        _loading = false;

        await RenderStepsAsync();
    }

    private async Task RenderStepsAsync()
    {
        Steps.Children.Clear();
        var actions = (_macro?["actions"] as JsonObject ?? new JsonObject())
            .Select(kv => (Index: int.Parse(kv.Key), Action: kv.Value as JsonObject))
            .Where(a => a.Action != null)
            .OrderBy(a => a.Index)
            .ToList();
        NoSteps.Visibility = actions.Count == 0 ? Visibility.Visible : Visibility.Collapsed;

        var number = 1;
        foreach (var (index, action) in actions)
        {
            var type = action!["actionType"]?.GetValue<int>() ?? 0;
            var command = action["actionCommand"]?.GetValue<int>() ?? 0;
            var (glyph, text) = type switch
            {
                KeyStep => ("\uE765", $"Press {await InputCatalog.NameAsync(command)}"),
                MouseStep => ("\uE962", $"Click {(await InputCatalog.NameAsync(command)).Replace("(Mouse) ", "")}"),
                PauseStep => ("\uE916", $"Wait {action["actionDelay"]?.GetValue<int>()} ms"),
                TextStep => ("\uE8D2", $"Type “{action["actionText"]?.GetValue<string>()}”"),
                MoveStep => ("\uE7C9", $"Move mouse to {action["mousePositionX"]}, {action["mousePositionY"]}"),
                LaunchStep => ("\uE8E5", $"Open {Path.GetFileName(action["actionText"]?.GetValue<string>() ?? "")}"),
                MediaStep => ("\uE8D6", "Media key"),
                _ => ("\uE9CE", $"Action {type}"),
            };

            var controls = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 12 };
            if (type is KeyStep or MouseStep)
            {
                var hold = new ToggleSwitch
                {
                    IsOn = action["actionHold"]?.GetValue<bool>() == true,
                    OnContent = "Hold until next key",
                    OffContent = "Press and release",
                };
                var stepIndex = index;
                var stepAction = action;
                hold.Toggled += async (_, _) => await UpdateHoldAsync(stepIndex, stepAction, hold.IsOn);
                controls.Children.Add(hold);
            }
            var remove = new Button { Content = new FontIcon { Glyph = "\uE74D", FontSize = 14 }, Tag = index };
            ToolTipService.SetToolTip(remove, "Remove step");
            remove.Click += RemoveStep_Click;
            controls.Children.Add(remove);

            Steps.Children.Add(new SettingsCard
            {
                Header = text,
                Description = $"Step {number++}",
                HeaderIcon = new FontIcon { Glyph = glyph },
                Content = controls,
            });
        }
    }

    private async Task UpdateHoldAsync(int index, JsonObject action, bool hold)
    {
        var result = await App.Api.PostAsync("api/macro/updateValue", new
        {
            macroId = _macroId,
            macroIndex = index,
            pressAndHold = hold,
            actionRepeatValue = action["actionRepeat"]?.GetValue<int>() ?? 0,
            actionRepeatDelay = action["actionRepeatDelay"]?.GetValue<int>() ?? 0,
            mousePositionX = action["mousePositionX"]?.GetValue<int>() ?? 0,
            mousePositionY = action["mousePositionY"]?.GetValue<int>() ?? 0,
            mousePositionAbsolute = action["mousePositionAbsolute"]?.GetValue<bool>() ?? false,
        });
        if (!result.Ok)
        {
            App.Window?.ShowToast(result.Message, false);
        }
    }

    private async void RemoveStep_Click(object sender, RoutedEventArgs e)
    {
        var result = await App.Api.DeleteAsync("api/macro/value", new { macroId = _macroId, macroIndex = (int)((Button)sender).Tag });
        if (!result.Ok)
        {
            App.Window?.ShowToast(result.Message, false);
        }
        await LoadMacroAsync();
    }

    private async void AddKind_Changed(SelectorBar sender, SelectorBarSelectionChangedEventArgs args)
    {
        var kind = int.Parse(sender.SelectedItem?.Tag as string ?? "3");
        AddKey.Visibility = kind is KeyStep or MouseStep ? Visibility.Visible : Visibility.Collapsed;
        AddDelay.Visibility = kind == PauseStep ? Visibility.Visible : Visibility.Collapsed;
        AddText.Visibility = kind == TextStep ? Visibility.Visible : Visibility.Collapsed;
        if (kind is KeyStep or MouseStep)
        {
            AddKey.ItemsSource = await (kind == KeyStep ? InputCatalog.KeyboardAsync() : InputCatalog.MouseAsync());
            AddKey.PlaceholderText = kind == KeyStep ? "Choose a key" : "Choose a button";
        }
    }

    private async void AddStep_Click(object sender, RoutedEventArgs e)
    {
        var kind = int.Parse(AddKind.SelectedItem?.Tag as string ?? "3");
        var command = kind is KeyStep or MouseStep && AddKey.SelectedItem is Choice c ? int.Parse(c.Value) : 0;
        if (kind is KeyStep or MouseStep && command == 0)
        {
            App.Window?.ShowToast("Choose a key first", false);
            return;
        }

        var result = await App.Api.PostAsync("api/macro/newValue", new
        {
            macroId = _macroId,
            macroType = kind,
            macroValue = command,
            macroDelay = kind == PauseStep ? (int)AddDelay.Value : 0,
            macroText = kind == TextStep ? AddText.Text : "",
            mousePositionX = 0,
            mousePositionY = 0,
            mousePositionAbsolute = false,
        });
        if (!result.Ok)
        {
            App.Window?.ShowToast(result.Message, false);
            return;
        }
        AddText.Text = "";
        await LoadMacroAsync();
    }

    private void Repeat_Changed(object sender, SelectionChangedEventArgs e)
    {
        UpdateRepeatVisibility();
        if (!_loading)
        {
            _repeatDebounce.Stop();
            _repeatDebounce.Start();
        }
    }

    private void RepeatValue_Changed(NumberBox sender, NumberBoxValueChangedEventArgs args)
    {
        if (!_loading)
        {
            _repeatDebounce.Stop();
            _repeatDebounce.Start();
        }
    }

    private void UpdateRepeatVisibility()
    {
        var mode = (RepeatMode.SelectedItem as ComboBoxItem)?.Tag as string;
        RepeatCount.Visibility = mode == "count" ? Visibility.Visible : Visibility.Collapsed;
        RepeatDelay.Visibility = mode == "once" ? Visibility.Collapsed : Visibility.Visible;
    }

    private async Task SaveRepeatAsync()
    {
        var mode = (RepeatMode.SelectedItem as ComboBoxItem)?.Tag as string;
        var repeat = mode switch { "loop" => -1, "count" => (int)RepeatCount.Value, _ => 0 };
        var result = await App.Api.PostAsync("api/macro/updateSettings", new
        {
            macroId = _macroId,
            macroRepeat = repeat,
            macroRepeatDelay = double.IsNaN(RepeatDelay.Value) ? 0 : (int)RepeatDelay.Value,
        });
        App.Window?.ShowToast(result.Ok ? "Playback saved" : result.Message, result.Ok);
    }

    private async void NewMacro_Click(object sender, RoutedEventArgs e)
    {
        var name = new TextBox { PlaceholderText = "At least 3 letters or numbers" };
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = "New macro",
            Content = name,
            PrimaryButtonText = "Create",
            CloseButtonText = "Cancel",
            DefaultButton = ContentDialogButton.Primary,
        };
        if (await dialog.ShowAsync() != ContentDialogResult.Primary)
        {
            return;
        }
        var result = await App.Api.PutAsync("api/macro/new", new { macroName = name.Text.Trim() });
        App.Window?.ShowToast(result.Message, result.Ok);
        if (result.Ok)
        {
            _macroId = null;
            var data = (await App.Api.GetAsync("api/macro/"))?["data"] as JsonObject;
            var created = data?.FirstOrDefault(kv => kv.Value?["name"]?.GetValue<string>() == name.Text.Trim()).Key;
            await LoadListAsync(created == null ? null : int.Parse(created));
        }
    }

    private async void DeleteMacro_Click(object sender, RoutedEventArgs e)
    {
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = $"Delete \"{MacroTitle.Text}\"?",
            Content = "Keys that play this macro stop doing anything.",
            PrimaryButtonText = "Delete",
            CloseButtonText = "Cancel",
        };
        if (await dialog.ShowAsync() != ContentDialogResult.Primary)
        {
            return;
        }
        var result = await App.Api.DeleteAsync("api/macro/profile", new { macroId = _macroId });
        App.Window?.ShowToast(result.Message, result.Ok);
        _macroId = null;
        await LoadListAsync();
    }
}

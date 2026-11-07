using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;
using Microsoft.UI.Xaml.Navigation;
using OpenLinkHub.App.Models;
using OpenLinkHub.App.Services;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage : Page
{
    private DeviceSummary? _device;
    private JsonObject? _detail;
    private IReadOnlyList<Choice> _rgbProfiles = Array.Empty<Choice>();
    private IReadOnlyList<Choice> _speedProfiles = Array.Empty<Choice>();
    private readonly Dictionary<string, Func<IEnumerable<UIElement>>> _sections = new();
    private readonly Dictionary<int, (TextBlock Rpm, TextBlock Temp)> _liveChannels = new();
    private readonly DispatcherTimer _refresh = new() { Interval = TimeSpan.FromSeconds(3) };

    public DevicePage()
    {
        InitializeComponent();
        _refresh.Tick += async (_, _) => await RefreshLiveAsync();
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        _device = e.Parameter as DeviceSummary;
        if (_device == null)
        {
            return;
        }

        DeviceGlyph.Glyph = _device.Glyph;
        DeviceName.Text = _device.DisplayName;
        DeviceInfo.Text = _device.CategoryName;

        try
        {
            _detail = await App.Api.GetDeviceAsync(_device.Serial);
            _rgbProfiles = (await App.Api.GetRgbProfilesAsync(_device.Serial))
                .Select(c => c.Value == ZoneProfileName ? c with { Label = ZoneProfileLabel } : c)
                .ToList();
            _speedProfiles = (await App.Api.GetSpeedProfileNamesAsync()).Select(n => new Choice(n, n)).ToList();
        }
        catch (Exception ex)
        {
            App.Window?.ShowToast($"Could not load device: {ex.Message}", false);
        }

        LoadingRing.IsActive = false;
        LoadingRing.Visibility = Visibility.Collapsed;
        if (_detail == null)
        {
            return;
        }

        DeviceInfo.Text = string.Join("  ·  ", new[]
        {
            _device.CategoryName,
            Str(_detail, "firmware") is { Length: > 0 } fw ? $"Firmware {fw}" : null,
            DeviceTile.DescribeStatus(_detail) is { Length: > 0 } s ? s : null,
        }.Where(p => p != null));

        await BuildPreviewAsync();
        BuildSections();
        _refresh.Start();
        StartLiveColors();
    }

    protected override void OnNavigatedFrom(NavigationEventArgs e)
    {
        _refresh.Stop();
        StopLiveColors();
    }

    private void BuildSections()
    {
        _sections.Clear();
        Sections.Items.Clear();

        AddSection("Lighting", "\uE781", LightingSection);
        if (HasKeys())
        {
            AddSection("Keys", "\uE765", KeysSection);
        }
        if (IsController && _detail?["ThumbStickModes"] is JsonObject)
        {
            AddSection("Thumbsticks", "\uE7FC", SticksSection);
            AddSection("Vibration", "\uE877", VibrationSection);
        }
        if (HasAudio())
        {
            AddSection("Audio", "\uE767", AudioSection);
        }
        if (FanChannels().Any())
        {
            AddSection("Cooling", "\uE9CA", CoolingSection);
        }
        if (HasPerformance())
        {
            AddSection("Performance", "\uE9D9", PerformanceSection);
        }
        if (_detail?["HasLCD"]?.GetValue<bool>() == true && _detail["LCDModes"] is JsonObject)
        {
            AddSection("LCD", "\uE7F4", LcdSection);
        }
        AddSection("Device", "\uE713", DeviceSection);

        Sections.SelectedItem = Sections.Items[0];
    }

    private void AddSection(string title, string glyph, Func<IEnumerable<UIElement>> build)
    {
        _sections[title] = build;
        Sections.Items.Add(new SelectorBarItem { Text = title, Icon = new FontIcon { Glyph = glyph }, Tag = title });
    }

    private void Sections_SelectionChanged(SelectorBar sender, SelectorBarSelectionChangedEventArgs args)
    {
        _liveChannels.Clear();
        SectionHost.Children.Clear();
        OnSectionChanging(sender.SelectedItem?.Tag as string);
        if (OfflineNotice() is { } notice)
        {
            SectionHost.Children.Add(notice);
        }
        if (sender.SelectedItem?.Tag is string key && _sections.TryGetValue(key, out var build))
        {
            foreach (var element in build())
            {
                SectionHost.Children.Add(element);
            }
        }
    }

    // ---------- Lighting ----------

    private IEnumerable<UIElement> LightingSection()
    {
        var profile = _detail?["DeviceProfile"] as JsonObject;
        var serial = _device!.Serial;

        if (profile?["RGBProfiles"] is JsonObject perChannel && perChannel.Count > 0)
        {
            yield return Header("Lighting effect");
            yield return ChoiceCard("All channels", "Apply one effect to everything connected", "\uE781",
                _rgbProfiles, perChannel.Select(c => c.Value?.GetValue<string>()).Distinct().Count() == 1
                    ? perChannel.First().Value?.GetValue<string>() : null,
                async value => Report(await App.Api.SetRgbProfileAsync(serial, -1, value)));

            var expander = new SettingsExpander
            {
                Header = "Per channel",
                Description = "Give each fan, strip or pump its own effect",
                HeaderIcon = new FontIcon { Glyph = "\uE8FD" },
            };
            foreach (var (key, value) in perChannel.OrderBy(c => int.TryParse(c.Key, out var n) ? n : 0))
            {
                var channelId = int.Parse(key);
                expander.Items.Add(ChoiceCard(ChannelName(channelId), null, null, _rgbProfiles,
                    value?.GetValue<string>(), async v => Report(await App.Api.SetRgbProfileAsync(serial, channelId, v))));
            }
            yield return expander;
        }
        else if (Str(profile, "RGBProfile") is { } current)
        {
            yield return Header("Lighting effect");
            var effect = ChoiceCard("Effect", "What the lights on this device do", "\uE781", _rgbProfiles, current,
                async value =>
                {
                    if (_syncingEffect)
                    {
                        return;
                    }
                    var result = await App.Api.SetRgbProfileAsync(serial, 0, value);
                    Report(result);
                    if (result.Ok && profile != null)
                    {
                        profile["RGBProfile"] = value;
                    }
                });
            _effectCombo = effect.Content as ComboBox;
            yield return effect;
        }

        foreach (var element in ZoneLightingCards())
        {
            yield return element;
        }

        foreach (var element in BrightnessCards(profile))
        {
            yield return element;
        }

        if (_keyboard == null && _drawing == null)
        {
            yield return Hint("Gradients and custom effect colours are in Advanced (web) for now.");
        }
    }

    private IEnumerable<UIElement> BrightnessCards(JsonObject? profile)
    {
        var serial = _device!.Serial;
        if (profile?["BrightnessSlider"] is JsonNode sliderNode)
        {
            yield return Header("Brightness");
            var slider = new Slider { Minimum = 0, Maximum = 100, StepFrequency = 5, Width = 260, Value = sliderNode.GetValue<int>() };
            var debounce = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(250) };
            debounce.Tick += async (_, _) =>
            {
                debounce.Stop();
                var result = await App.Api.SetBrightnessGradualAsync(serial, (int)slider.Value);
                if (!result.Ok)
                {
                    Report(result);
                }
            };
            slider.ValueChanged += (_, _) => { debounce.Stop(); debounce.Start(); };
            yield return new SettingsCard
            {
                Header = "Brightness",
                Description = "Applies to every LED on this device",
                HeaderIcon = new FontIcon { Glyph = "\uE706" },
                Content = slider,
            };
        }
        else if (_detail?["Brightness"] is JsonObject levels && levels.Count > 0)
        {
            yield return Header("Brightness");
            yield return ChoiceCard("Brightness", "Hardware brightness level", "\uE706",
                levels.Select(l => new Choice(l.Key, l.Value?.GetValue<string>() ?? l.Key)).ToList(),
                profile?["Brightness"]?.ToString(),
                async value => Report(await App.Api.SetBrightnessAsync(serial, int.Parse(value))));
        }
    }

    // ---------- Cooling ----------

    private IEnumerable<(int Id, JsonObject Channel)> FanChannels() =>
        (_detail?["devices"] as JsonObject ?? new JsonObject())
        .Where(c => c.Value?["HasSpeed"]?.GetValue<bool>() == true)
        .Select(c => (int.Parse(c.Key), c.Value!.AsObject()))
        .OrderBy(c => c.Item1);

    private IEnumerable<UIElement> CoolingSection()
    {
        var serial = _device!.Serial;
        var fans = FanChannels().ToList();

        yield return Header("Fan & pump speed");
        yield return ChoiceCard("All fans", "Apply one speed curve to every fan and pump", "\uE9CA", _speedProfiles,
            fans.Select(f => Str(f.Channel, "profile")).Distinct().Count() == 1 ? Str(fans[0].Channel, "profile") : null,
            async value => Report(await App.Api.SetSpeedProfileAsync(serial, -1, value)));

        foreach (var (id, channel) in fans)
        {
            var rpm = new TextBlock { Style = (Style)Application.Current.Resources["BodyStrongTextBlockStyle"], MinWidth = 80, TextAlignment = TextAlignment.Right, VerticalAlignment = VerticalAlignment.Center };
            var temp = new TextBlock { Foreground = (Microsoft.UI.Xaml.Media.Brush)Application.Current.Resources["TextFillColorSecondaryBrush"], VerticalAlignment = VerticalAlignment.Center };
            _liveChannels[id] = (rpm, temp);
            UpdateLive(id, channel);

            var combo = Combo(_speedProfiles, Str(channel, "profile"),
                async value => Report(await App.Api.SetSpeedProfileAsync(serial, id, value)));

            yield return new SettingsCard
            {
                Header = ChannelName(id),
                Description = temp,
                HeaderIcon = new FontIcon { Glyph = "\uE9CA" },
                Content = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 16, Children = { rpm, combo } },
            };
        }

        var curves = new SettingsCard
        {
            Header = "Edit fan curves",
            Description = "Change how fast fans spin at each temperature, or create your own curve",
            HeaderIcon = new FontIcon { Glyph = "\uE9D2" },
            IsClickEnabled = true,
        };
        curves.Click += (_, _) => Frame.Navigate(typeof(FanCurvesPage), FanChannels().Select(f => Str(f.Channel, "profile")).FirstOrDefault());
        yield return Header("Curves");
        yield return curves;
    }

    private void UpdateLive(int id, JsonObject channel)
    {
        if (!_liveChannels.TryGetValue(id, out var live))
        {
            return;
        }
        live.Rpm.Text = $"{channel["rpm"]?.GetValue<int>() ?? 0} RPM";
        var temp = Str(channel, "temperatureString");
        live.Temp.Text = string.IsNullOrEmpty(temp) ? Str(channel, "description") ?? "" : $"{Str(channel, "description")} · {temp}";
    }

    private async Task RefreshLiveAsync()
    {
        if (_device == null || _liveChannels.Count == 0)
        {
            return;
        }
        try
        {
            var detail = await App.Api.GetDeviceAsync(_device.Serial);
            if (detail?["devices"] is JsonObject channels)
            {
                foreach (var (key, value) in channels)
                {
                    if (int.TryParse(key, out var id) && value is JsonObject channel)
                    {
                        UpdateLive(id, channel);
                    }
                }
            }
        }
        catch
        {
        }
    }

    // ---------- Performance ----------

    private bool HasPerformance()
    {
        var profile = _detail?["DeviceProfile"] as JsonObject;
        return _detail?["PollingRates"] is JsonObject
               || _detail?["SleepModes"] is JsonObject
               || (_device!.Category == DeviceCategory.Mouse && profile?["Profiles"] is JsonObject);
    }

    private IEnumerable<UIElement> PerformanceSection()
    {
        var serial = _device!.Serial;
        var profile = _detail?["DeviceProfile"] as JsonObject;

        if (_device.Category == DeviceCategory.Mouse && profile?["Profiles"] is JsonObject stages)
        {
            yield return Header("DPI stages");
            var min = _detail?["MinDPI"]?.GetValue<int>() ?? 100;
            var max = _detail?["MaxDPI"]?.GetValue<int>() ?? 26000;
            var activeStage = profile["Profile"]?.GetValue<int>() ?? -1;
            var boxes = new Dictionary<string, NumberBox>();

            var expander = new SettingsExpander
            {
                Header = "Sensitivity",
                Description = $"{stages.Count} stages · switch with the DPI button on the mouse",
                HeaderIcon = new FontIcon { Glyph = "\uE962" },
                IsExpanded = true,
            };
            foreach (var (key, stage) in stages.OrderBy(s => int.Parse(s.Key)))
            {
                var box = new NumberBox
                {
                    Minimum = min,
                    Maximum = max,
                    SmallChange = 50,
                    LargeChange = 400,
                    Value = stage?["Value"]?.GetValue<int>() ?? 800,
                    SpinButtonPlacementMode = NumberBoxSpinButtonPlacementMode.Inline,
                    Width = 180,
                };
                boxes[key] = box;
                var name = Str(stage as JsonObject, "name") ?? $"Stage {int.Parse(key) + 1}";
                if (stage?["Sniper"]?.GetValue<bool>() == true && !name.Contains("sniper", StringComparison.OrdinalIgnoreCase))
                {
                    name += " (sniper)";
                }
                expander.Items.Add(new SettingsCard
                {
                    Header = name,
                    Description = int.Parse(key) == activeStage ? "Active" : "",
                    Content = box,
                });
            }

            var save = new Button { Content = "Save DPI", Style = (Style)Application.Current.Resources["AccentButtonStyle"] };
            save.Click += async (_, _) =>
            {
                save.IsEnabled = false;
                var values = boxes.ToDictionary(b => b.Key, b => (int)Math.Round(b.Value.Value / 50) * 50);
                Report(await App.Api.SetMouseDpiAsync(serial, values));
                save.IsEnabled = true;
            };
            expander.ItemsFooter = new Grid { Padding = new Thickness(58, 12, 44, 12), Children = { new StackPanel { HorizontalAlignment = HorizontalAlignment.Right, Children = { save } } } };
            yield return expander;
        }

        if (_detail?["PollingRates"] is JsonObject rates)
        {
            yield return Header("Responsiveness");
            yield return ChoiceCard("Polling rate", "How often the device reports to the PC", "\uE916",
                rates.Where(r => r.Key != "0").Select(r => new Choice(r.Key, r.Value?.GetValue<string>() ?? r.Key)).ToList(),
                profile?["PollingRate"]?.ToString(),
                async value => Report(await App.Api.SetPollingRateAsync(serial, _device.Category, int.Parse(value))));
        }

        if (_detail?["SleepModes"] is JsonObject sleepModes)
        {
            yield return Header("Power");
            yield return ChoiceCard("Sleep after", "Turn off lighting and radio when idle", "\uE708",
                sleepModes.OrderBy(s => int.Parse(s.Key)).Select(s => new Choice(s.Key, s.Value?.GetValue<string>() ?? s.Key)).ToList(),
                profile?["SleepMode"]?.ToString(),
                async value => Report(await App.Api.SetSleepModeAsync(serial, _device.Category, int.Parse(value))));
        }
    }

    // ---------- Device ----------

    private IEnumerable<UIElement> DeviceSection()
    {
        yield return Header("About");
        yield return InfoCard("Product", _device!.Product, "\uE946");
        yield return InfoCard("Serial", _device.Serial, "\uE8EC");
        if (Str(_detail, "firmware") is { Length: > 0 } fw)
        {
            yield return InfoCard("Firmware", fw, "\uE8B5");
        }

        var open = new SettingsCard
        {
            Header = "Key assignments, macros and more",
            Description = "Open this device in the OpenLinkHub web interface",
            HeaderIcon = new FontIcon { Glyph = "\uE774" },
            IsClickEnabled = true,
            ActionIcon = new FontIcon { Glyph = "\uE8A7" },
        };
        open.Click += OpenWebUi_Click;
        yield return Header("Advanced");
        yield return open;
    }

    // ---------- helpers ----------

    private string ChannelName(int channelId)
    {
        var channel = _detail?["devices"]?[channelId.ToString()];
        var label = Str(channel as JsonObject, "label");
        var name = Str(channel as JsonObject, "name") ?? $"Channel {channelId + 1}";
        return string.IsNullOrEmpty(label) || label == "Set Label" ? name : $"{label} ({name})";
    }

    private static string? Str(JsonObject? obj, string key) =>
        obj?[key] is JsonValue v && v.TryGetValue<string>(out var s) ? s : null;

    private static TextBlock Header(string text) => new()
    {
        Text = text,
        Style = (Style)Application.Current.Resources["SectionHeaderStyle"],
    };

    private static TextBlock Hint(string text) => new()
    {
        Text = text,
        Margin = new Thickness(2, 16, 0, 0),
        TextWrapping = TextWrapping.Wrap,
        Foreground = (Microsoft.UI.Xaml.Media.Brush)Application.Current.Resources["TextFillColorTertiaryBrush"],
        Style = (Style)Application.Current.Resources["CaptionTextBlockStyle"],
    };

    private static SettingsCard InfoCard(string header, string value, string glyph) => new()
    {
        Header = header,
        HeaderIcon = new FontIcon { Glyph = glyph },
        Content = new TextBlock { Text = value, IsTextSelectionEnabled = true },
    };

    private static SettingsCard ChoiceCard(string header, string? description, string? glyph, IReadOnlyList<Choice> choices,
        string? current, Func<string, Task> onChanged) => new()
    {
        Header = header,
        Description = description ?? "",
        HeaderIcon = glyph == null ? null! : new FontIcon { Glyph = glyph },
        Content = Combo(choices, current, onChanged),
    };

    private static ComboBox Combo(IReadOnlyList<Choice> choices, string? current, Func<string, Task> onChanged)
    {
        var combo = new ComboBox { ItemsSource = choices, MinWidth = 200, PlaceholderText = current == null ? "Mixed" : current };
        combo.SelectedItem = choices.FirstOrDefault(c => string.Equals(c.Value, current, StringComparison.OrdinalIgnoreCase));
        combo.SelectionChanged += async (_, e) =>
        {
            if (e.AddedItems is [Choice choice])
            {
                await onChanged(choice.Value);
            }
        };
        return combo;
    }

    private static void Report(ApiResult result) => App.Window?.ShowToast(result.Message, result.Ok);

    private void OpenWebUi_Click(object sender, RoutedEventArgs e)
    {
        if (_device != null)
        {
            _ = Windows.System.Launcher.LaunchUriAsync(App.Api.WebUiDeviceUri(_device.Serial));
        }
    }
}

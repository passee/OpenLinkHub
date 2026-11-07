using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using OpenLinkHub.App.Controls;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private const double EqRange = 12;

    private bool HasAudio() => _device!.Category == DeviceCategory.Headset;

    private IEnumerable<UIElement> AudioSection()
    {
        var host = new StackPanel { Spacing = 4 };
        _ = FillAudioAsync(host);
        yield return host;
    }

    private async Task FillAudioAsync(StackPanel host)
    {
        var serial = _device!.Serial;
        var settings = (await App.Api.GetAsync($"api/headset/settings/{serial}"))?["data"] as JsonObject ?? new JsonObject();
        var bands = (await App.Api.GetAsync($"api/headset/getEqualizers/{serial}"))?["data"] as JsonObject;
        var presets = (await App.Api.GetAsync("api/headset/eqPresets"))?["data"] as JsonArray ?? new JsonArray();

        if (bands != null && settings["eqAvailable"]?.GetValue<bool>() != false)
        {
            host.Children.Add(Header("Equalizer"));
            host.Children.Add(BuildEqualizer(serial, bands, presets, settings["eqClippingProtection"]?.GetValue<bool>() == true));
        }

        host.Children.Add(Header("Microphone"));
        if (settings["micVolume"]?.GetValue<int>() is >= 0 and var volume)
        {
            host.Children.Add(SliderCard("Mic volume", "Windows recording level", "\uE720", 0, 100, 1, volume, "%",
                async v => await Quiet(App.Api.PostAsync("api/headset/micVolume", new { deviceId = serial, volume = (int)v }))));
        }
        if (settings["micBoost"]?.GetValue<double>() is >= 0 and var boost)
        {
            host.Children.Add(SliderCard("Mic boost", "Extra gain through Equalizer APO", "\uE995", 0, 20, 1, boost, " dB",
                async v => await Quiet(App.Api.PostAsync("api/headset/micBoost", new { boost = v }))));
        }

        if (settings["micLed"] is JsonObject micLed)
        {
            host.Children.Add(BuildMicLed(serial, micLed));
        }

        if (settings["voicePromptOptions"] is JsonArray prompts)
        {
            host.Children.Add(Header("Voice prompts"));
            host.Children.Add(BuildVoicePrompts(serial, settings["voicePromptsEnabled"]?.GetValue<bool>() != false, prompts));
        }
    }

    private UIElement BuildEqualizer(string serial, JsonObject bands, JsonArray presets, bool clipping)
    {
        var ordered = bands.OrderBy(b => int.Parse(b.Key)).ToList();
        var sliders = new List<Slider>();
        var applying = false;
        var debounce = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(300) };
        debounce.Tick += async (_, _) =>
        {
            debounce.Stop();
            var values = ordered.Select((b, i) => (b.Key, Value: Math.Round(sliders[i].Value, 1)))
                .ToDictionary(b => b.Key, b => b.Value);
            var result = await App.Api.PostAsync("api/headset/equalizer", new { deviceId = serial, equalizers = values });
            if (!result.Ok && !result.Message.Contains("not changed", StringComparison.OrdinalIgnoreCase))
            {
                Report(result);
            }
        };

        var grid = new Grid { ColumnSpacing = 4, Margin = new Thickness(0, 8, 0, 0) };
        for (var i = 0; i < ordered.Count; i++)
        {
            grid.ColumnDefinitions.Add(new ColumnDefinition());
            var value = new TextBlock { HorizontalAlignment = HorizontalAlignment.Center, Style = (Style)Application.Current.Resources["CaptionTextBlockStyle"] };
            var slider = new Slider
            {
                Orientation = Orientation.Vertical,
                Minimum = -EqRange,
                Maximum = EqRange,
                StepFrequency = 0.5,
                Height = 200,
                HorizontalAlignment = HorizontalAlignment.Center,
                Value = ordered[i].Value?["Value"]?.GetValue<double>() ?? 0,
                TickFrequency = 6,
                TickPlacement = Microsoft.UI.Xaml.Controls.Primitives.TickPlacement.Outside,
            };
            value.Text = $"{slider.Value:+0.0;-0.0;0}";
            slider.ValueChanged += (_, e) =>
            {
                value.Text = $"{e.NewValue:+0.0;-0.0;0}";
                if (!applying)
                {
                    debounce.Stop();
                    debounce.Start();
                }
            };
            sliders.Add(slider);
            var label = new TextBlock
            {
                Text = $"{ordered[i].Value?["Name"]?.GetValue<string>()}",
                HorizontalAlignment = HorizontalAlignment.Center,
                Foreground = (Brush)Application.Current.Resources["TextFillColorSecondaryBrush"],
            };
            var column = new StackPanel { Spacing = 6, Children = { value, slider, label } };
            Grid.SetColumn(column, i);
            grid.Children.Add(column);
        }

        var presetCombo = new ComboBox
        {
            PlaceholderText = "Choose a preset",
            MinWidth = 220,
            ItemsSource = presets.Select(p => p?["name"]?.GetValue<string>() ?? "").ToList(),
        };
        presetCombo.SelectionChanged += async (_, e) =>
        {
            if (e.AddedItems is not [string name])
            {
                return;
            }
            var result = await App.Api.PostAsync("api/headset/eqPreset", new { deviceId = serial, name });
            Report(result);
            var preset = presets.FirstOrDefault(p => p?["name"]?.GetValue<string>() == name)?["bands"] as JsonArray;
            if (result.Ok && preset != null)
            {
                applying = true;
                for (var i = 0; i < sliders.Count && i < preset.Count; i++)
                {
                    sliders[i].Value = preset[i]?.GetValue<double>() ?? 0;
                }
                applying = false;
            }
        };
        var flat = new Button { Content = "Flat" };
        flat.Click += (_, _) =>
        {
            presetCombo.SelectedItem = null;
            foreach (var slider in sliders)
            {
                slider.Value = 0;
            }
        };
        var clip = new ToggleSwitch { IsOn = clipping, OnContent = "Clipping protection", OffContent = "Clipping protection" };
        ToolTipService.SetToolTip(clip, "Lowers the overall level so boosted bands don't distort");
        clip.Toggled += async (_, _) => Report(await App.Api.PostAsync("api/headset/eqClippingProtection", new { enabled = clip.IsOn }));

        var toolbar = new Grid();
        toolbar.Children.Add(new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8, Children = { presetCombo, flat } });
        toolbar.Children.Add(new StackPanel { HorizontalAlignment = HorizontalAlignment.Right, Children = { clip } });

        return new Border
        {
            Style = (Style)Application.Current.Resources["CardBorderStyle"],
            Child = new StackPanel { Spacing = 8, Children = { toolbar, grid } },
        };
    }

    private UIElement BuildMicLed(string serial, JsonObject micLed)
    {
        var active = new ColorButton(ColorButton.Parse(Str(micLed, "activeColor")));
        var muted = new ColorButton(ColorButton.Parse(Str(micLed, "mutedColor")));
        var mode = new ComboBox
        {
            ItemsSource = new[] { new Choice("lighting", "Follow the lighting effect"), new Choice("color", "Show mute state"), new Choice("off", "Off") },
            MinWidth = 220,
        };
        mode.SelectedItem = ((Choice[])mode.ItemsSource).FirstOrDefault(c => c.Value == Str(micLed, "active"));
        var colors = new StackPanel
        {
            Orientation = Orientation.Horizontal,
            Spacing = 8,
            VerticalAlignment = VerticalAlignment.Center,
            Children = { new TextBlock { Text = "Live", VerticalAlignment = VerticalAlignment.Center }, active, new TextBlock { Text = "Muted", VerticalAlignment = VerticalAlignment.Center, Margin = new Thickness(8, 0, 0, 0) }, muted },
        };

        async Task SaveAsync()
        {
            colors.Visibility = (mode.SelectedItem as Choice)?.Value == "color" ? Visibility.Visible : Visibility.Collapsed;
            Report(await App.Api.PostAsync("api/headset/micLed", new
            {
                deviceId = serial,
                active = (mode.SelectedItem as Choice)?.Value ?? "lighting",
                activeColor = ColorButton.ToJson(active.Color),
                mutedColor = ColorButton.ToJson(muted.Color),
            }));
        }

        colors.Visibility = (mode.SelectedItem as Choice)?.Value == "color" ? Visibility.Visible : Visibility.Collapsed;
        mode.SelectionChanged += async (_, _) => await SaveAsync();
        active.ColorCommitted += async (_, _) => await SaveAsync();
        muted.ColorCommitted += async (_, _) => await SaveAsync();

        return new SettingsCard
        {
            Header = "Mic LED",
            Description = "The light at the tip of the microphone",
            HeaderIcon = new FontIcon { Glyph = "\uE781" },
            Content = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 16, Children = { colors, mode } },
        };
    }

    private UIElement BuildVoicePrompts(string serial, bool enabled, JsonArray prompts)
    {
        var expander = new SettingsExpander
        {
            Header = "Spoken announcements",
            Description = "The headset says when the mic mutes, the battery runs low, and more",
            HeaderIcon = new FontIcon { Glyph = "\uE767" },
            IsExpanded = enabled,
        };
        var master = new ToggleSwitch { IsOn = enabled };
        expander.Content = master;

        var boxes = new Dictionary<string, CheckBox>();
        foreach (var prompt in prompts.OfType<JsonObject>())
        {
            var id = Str(prompt, "Id") ?? "";
            var box = new CheckBox { IsChecked = prompt["Active"]?.GetValue<bool>() == true, IsEnabled = enabled };
            boxes[id] = box;
            box.Click += async (_, _) => Report(await App.Api.PostAsync("api/headset/voicePrompts", new
            {
                deviceId = serial,
                enabled = master.IsOn,
                prompts = boxes.ToDictionary(b => b.Key, b => b.Value.IsChecked == true),
            }));
            expander.Items.Add(new SettingsCard { Header = Str(prompt, "Label") ?? id, Content = box });
        }

        master.Toggled += async (_, _) =>
        {
            foreach (var box in boxes.Values)
            {
                box.IsEnabled = master.IsOn;
            }
            expander.IsExpanded = master.IsOn;
            Report(await App.Api.PostAsync("api/headset/voicePrompts", new { deviceId = serial, enabled = master.IsOn }));
        };
        return expander;
    }

    private static SettingsCard SliderCard(string header, string description, string glyph, double min, double max, double step,
        double value, string unit, Func<double, Task> onChanged)
    {
        var label = new TextBlock { MinWidth = 48, TextAlignment = TextAlignment.Right, VerticalAlignment = VerticalAlignment.Center };
        var slider = new Slider { Minimum = min, Maximum = max, StepFrequency = step, Width = 220, Value = value };
        label.Text = $"{value:0}{unit}";
        var debounce = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(300) };
        debounce.Tick += async (_, _) =>
        {
            debounce.Stop();
            await onChanged(slider.Value);
        };
        slider.ValueChanged += (_, e) =>
        {
            label.Text = $"{e.NewValue:0}{unit}";
            debounce.Stop();
            debounce.Start();
        };
        return new SettingsCard
        {
            Header = header,
            Description = description,
            HeaderIcon = new FontIcon { Glyph = glyph },
            Content = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8, Children = { label, slider } },
        };
    }

    private static async Task Quiet(Task<Services.ApiResult> call)
    {
        var result = await call;
        if (!result.Ok && !string.IsNullOrEmpty(result.Message))
        {
            Report(result);
        }
    }
}

using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Views;

public sealed partial class DevicePage
{
    private const int LcdImageMode = 10;
    private bool _fillingGallery;

    private IEnumerable<UIElement> LcdSection()
    {
        var serial = _device!.Serial;
        var profile = _detail?["DeviceProfile"] as JsonObject;
        var modes = (JsonObject)_detail!["LCDModes"]!;

        yield return Header("Screen");
        var modeCard = ChoiceCard("Shows", "What the LCD displays", "\uE7F4",
            modes.OrderBy(m => int.Parse(m.Key)).Select(m => new Choice(m.Key, m.Value?.GetValue<string>() ?? m.Key)).ToList(),
            profile?["LCDMode"]?.ToString(),
            async value =>
            {
                var result = await App.Api.SetLcdModeAsync(serial, 0, int.Parse(value));
                Report(result);
                if (result.Ok && profile != null)
                {
                    profile["LCDMode"] = int.Parse(value);
                }
            });
        yield return modeCard;

        if (_detail["LCDRotations"] is JsonObject rotations)
        {
            yield return ChoiceCard("Rotation", "Turn the picture to match how the pump is mounted", "\uE7AD",
                rotations.OrderBy(r => int.Parse(r.Key)).Select(r => new Choice(r.Key, Capitalize(r.Value?.GetValue<string>() ?? r.Key))).ToList(),
                profile?["LCDRotation"]?.ToString(),
                async value => Report(await App.Api.PostAsync("api/lcd/rotation", new { deviceId = serial, channelId = 0, rotation = int.Parse(value) })));
        }

        if (_detail["LCDBrightnessLevels"] is JsonObject levels)
        {
            yield return ChoiceCard("Screen brightness", null, "\uE706",
                levels.OrderBy(l => int.Parse(l.Key)).Select(l => new Choice(l.Key, l.Value?.GetValue<string>() ?? l.Key)).ToList(),
                profile?["LCDBrightness"]?.ToString(),
                async value => Report(await App.Api.PostAsync("api/lcd/brightness", new { deviceId = serial, channelId = 0, brightness = int.Parse(value) })));
        }

        yield return Header("Images");
        var gallery = new GridView
        {
            SelectionMode = ListViewSelectionMode.Single,
            IsItemClickEnabled = false,
            Padding = new Thickness(0),
        };
        var upload = new Button
        {
            Content = new StackPanel
            {
                Orientation = Orientation.Horizontal,
                Spacing = 8,
                Children = { new FontIcon { Glyph = "\uE898", FontSize = 14 }, new TextBlock { Text = "Add image or GIF" } },
            },
        };
        upload.Click += async (_, _) => await UploadLcdImageAsync(gallery);

        yield return new Grid
        {
            Margin = new Thickness(0, 0, 0, 8),
            Children =
            {
                new TextBlock
                {
                    Text = "Pick an image to show it on the screen. JPG, PNG, GIF, WebP or BMP, up to 5 MB.",
                    VerticalAlignment = VerticalAlignment.Center,
                    Foreground = (Brush)Application.Current.Resources["TextFillColorSecondaryBrush"],
                },
                new StackPanel { HorizontalAlignment = HorizontalAlignment.Right, Children = { upload } },
            },
        };
        yield return gallery;

        gallery.SelectionChanged += async (_, e) =>
        {
            if (_fillingGallery || e.AddedItems is not [GridViewItem { Tag: string name }])
            {
                return;
            }
            if (profile?["LCDMode"]?.GetValue<int>() != LcdImageMode)
            {
                await App.Api.SetLcdModeAsync(serial, 0, LcdImageMode);
                if (profile != null)
                {
                    profile["LCDMode"] = LcdImageMode;
                }
                if (modeCard.Content is ComboBox modeCombo)
                {
                    modeCombo.SelectedItem = (modeCombo.ItemsSource as IEnumerable<Choice>)?.FirstOrDefault(c => c.Value == LcdImageMode.ToString());
                }
            }
            var result = await App.Api.PostAsync("api/lcd/image", new { deviceId = serial, channelId = 0, image = name });
            Report(result);
            if (result.Ok && profile != null)
            {
                profile["LCDImage"] = name;
            }
        };
        _ = FillGalleryAsync(gallery, Str(profile, "LCDImage"));
    }

    private async Task FillGalleryAsync(GridView gallery, string? selected)
    {
        var images = (await App.Api.GetAsync("api/lcd/images"))?["data"] as JsonArray ?? new JsonArray();
        var folder = LcdImageFolder();
        gallery.Items.Clear();
        foreach (var image in images)
        {
            var name = image?["name"]?.GetValue<string>() ?? "";
            var file = folder == null ? null : Directory.EnumerateFiles(folder, name + ".*").FirstOrDefault();
            var tile = new Grid { Width = 128, Height = 150, RowSpacing = 6 };
            tile.RowDefinitions.Add(new RowDefinition { Height = new GridLength(120) });
            tile.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
            var circle = new Microsoft.UI.Xaml.Shapes.Ellipse
            {
                Width = 116,
                Height = 116,
                Stroke = (Brush)Application.Current.Resources["ControlStrokeColorDefaultBrush"],
                StrokeThickness = 1,
                Fill = file == null
                    ? (Brush)Application.Current.Resources["SubtleFillColorSecondaryBrush"]
                    : new ImageBrush { ImageSource = new BitmapImage(new Uri(file)) { DecodePixelWidth = 232 }, Stretch = Stretch.UniformToFill },
            };
            tile.Children.Add(circle);
            var label = new TextBlock { Text = name, HorizontalAlignment = HorizontalAlignment.Center, TextTrimming = TextTrimming.CharacterEllipsis };
            Grid.SetRow(label, 1);
            tile.Children.Add(label);

            var item = new GridViewItem { Content = tile, Tag = name, Margin = new Thickness(0, 0, 8, 8) };
            ToolTipService.SetToolTip(item, (image?["frames"]?.GetValue<int>() ?? 1) > 1 ? $"{name} (animated)" : name);
            gallery.Items.Add(item);
        }
        _fillingGallery = true;
        gallery.SelectedItem = gallery.Items.OfType<GridViewItem>().FirstOrDefault(i => (string)i.Tag == selected);
        _fillingGallery = false;
    }

    private async Task UploadLcdImageAsync(GridView gallery)
    {
        var picker = new Windows.Storage.Pickers.FileOpenPicker();
        WinRT.Interop.InitializeWithWindow.Initialize(picker, WinRT.Interop.WindowNative.GetWindowHandle(App.Window));
        foreach (var ext in new[] { ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico" })
        {
            picker.FileTypeFilter.Add(ext);
        }
        var file = await picker.PickSingleFileAsync();
        if (file == null)
        {
            return;
        }
        var result = await App.Api.UploadLcdImageAsync(file.Path);
        Report(result);
        if (result.Ok)
        {
            await FillGalleryAsync(gallery, Str(_detail?["DeviceProfile"] as JsonObject, "LCDImage"));
        }
    }

    /// The service serves no image files, but it runs on this PC and its profile paths point into its database folder.
    private string? LcdImageFolder()
    {
        var profilePath = Str(_detail?["DeviceProfile"] as JsonObject, "Path");
        if (profilePath == null)
        {
            return null;
        }
        var database = Path.GetDirectoryName(Path.GetDirectoryName(Path.GetFullPath(profilePath)));
        var folder = database == null ? null : Path.Combine(database, "lcd", "images");
        return Directory.Exists(folder) ? folder : null;
    }

    private static string Capitalize(string text) => text.Length == 0 ? text : char.ToUpper(text[0]) + text[1..];
}

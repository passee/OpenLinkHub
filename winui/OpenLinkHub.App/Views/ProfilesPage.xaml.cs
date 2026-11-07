using System.Text.Json.Nodes;
using CommunityToolkit.WinUI.Controls;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using OpenLinkHub.App.Services;

namespace OpenLinkHub.App.Views;

public sealed partial class ProfilesPage : Page
{
    private IReadOnlyDictionary<string, JsonObject> _details = new Dictionary<string, JsonObject>();
    private bool _loadingLock;

    public ProfilesPage()
    {
        InitializeComponent();
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e) => await LoadAsync();

    private async Task LoadAsync()
    {
        LoadingRing.IsActive = true;
        LoadingRing.Visibility = Visibility.Visible;
        try
        {
            _loadingLock = true;
            LockToggle.IsOn = await App.Api.GetProfileLockAsync();
            _loadingLock = false;

            var devices = (await App.Api.GetDevicesAsync()).Where(d => !d.Hidden).ToList();
            var (profiles, details) = await ProfileService.LoadAsync(devices);
            _details = details;

            ProfileList.Children.Clear();
            foreach (var profile in profiles)
            {
                var active = profile.ActiveCount > 0;
                var button = new Button
                {
                    Content = active ? "Active" : "Activate",
                    IsEnabled = !profile.IsActiveEverywhere,
                    Tag = profile.Name,
                    MinWidth = 96,
                };
                button.Click += Activate_Click;
                ProfileList.Children.Add(new SettingsCard
                {
                    Header = profile.DisplayName,
                    Description = active && !profile.IsActiveEverywhere
                        ? $"Active on {profile.ActiveCount} of {profile.DeviceCount} devices"
                        : $"{profile.DeviceCount} devices",
                    HeaderIcon = new FontIcon { Glyph = active ? "\uE73E" : "\uE8F1" },
                    Content = button,
                });
            }

            AppList.Children.Clear();
            foreach (var app in (await App.Api.GetAppProfilesAsync()).OrderBy(a => a.Name))
            {
                var targets = app.ActiveProfiles.Values.Distinct().ToList();
                AppList.Children.Add(new SettingsCard
                {
                    Header = app.Name,
                    Description = $"Uses {string.Join(", ", targets)} on {app.ActiveProfiles.Count} devices",
                    HeaderIcon = new FontIcon { Glyph = "\uE7FC" },
                    Content = new TextBlock
                    {
                        Text = app.Enabled ? "On" : "Off",
                        Foreground = (Microsoft.UI.Xaml.Media.Brush)Application.Current.Resources["TextFillColorSecondaryBrush"],
                    },
                });
            }
        }
        catch (Exception ex)
        {
            App.Window?.ShowToast($"Could not load profiles: {ex.Message}", false);
        }
        finally
        {
            _loadingLock = false;
            LoadingRing.IsActive = false;
            LoadingRing.Visibility = Visibility.Collapsed;
        }
    }

    private async void Activate_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: string name } button)
        {
            return;
        }
        button.IsEnabled = false;
        var (ok, failed) = await ProfileService.ActivateAsync(name, _details);
        App.Window?.ShowToast(failed == 0 ? $"Applied to {ok} devices" : $"Applied to {ok} devices, {failed} failed", failed == 0);
        await LoadAsync();
    }

    private async void LockToggle_Toggled(object sender, RoutedEventArgs e)
    {
        if (_loadingLock)
        {
            return;
        }
        var result = await App.Api.SetProfileLockAsync(LockToggle.IsOn);
        App.Window?.ShowToast(result.Message, result.Ok);
    }
}

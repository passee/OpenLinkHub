using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace OpenLinkHub.App.Views;

public sealed partial class SettingsPage : Page
{
    public SettingsPage()
    {
        InitializeComponent();
        AddressCard.Description = App.Api.BaseAddress.ToString();
        var current = (App.Window?.Content as FrameworkElement)?.RequestedTheme.ToString() ?? "Default";
        ThemeCombo.SelectedItem = ThemeCombo.Items.OfType<ComboBoxItem>().FirstOrDefault(i => (string)i.Tag == current);
    }

    private void ThemeCombo_SelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (ThemeCombo.SelectedItem is ComboBoxItem { Tag: string tag } && App.Window?.Content is FrameworkElement root)
        {
            root.RequestedTheme = Enum.Parse<ElementTheme>(tag);
        }
    }

    private void OpenWeb_Click(object sender, RoutedEventArgs e) =>
        _ = Windows.System.Launcher.LaunchUriAsync(App.Api.BaseAddress);
}

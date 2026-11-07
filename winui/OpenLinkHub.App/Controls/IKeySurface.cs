using Windows.UI;

namespace OpenLinkHub.App.Controls;

/// A clickable picture of a keyboard: the iCUE render when available, otherwise a drawn layout.
public interface IKeySurface
{
    KeySelectionMode SelectionMode { get; set; }
    bool AssignableOnly { get; set; }
    bool ShowColors { get; set; }
    Func<KeyInfo, bool>? Selectable { get; set; }
    IReadOnlyCollection<int> SelectedKeys { get; }
    IReadOnlyDictionary<int, KeyInfo> Keys { get; }

    event EventHandler? SelectionChanged;

    void ResetSelectionListeners();
    void ClearSelection();
    void SelectAll();
    void Select(int id, bool notify = true);
    void SetColor(IEnumerable<int> ids, Color color);
    void SetHighlighted(IEnumerable<int> ids);

    /// What the LEDs show right now; takes precedence over the stored key colours.
    void SetLiveColors(IReadOnlyDictionary<int, Color> colors);
}

using System.Text.Json.Nodes;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Services;

/// Key, media and mouse-button names the service accepts as action commands, keyed by their list index.
public static class InputCatalog
{
    private static Task<IReadOnlyList<Choice>>? _keyboard, _media, _mouse;

    public static Task<IReadOnlyList<Choice>> KeyboardAsync() => _keyboard ??= LoadAsync("api/input/keyboard");
    public static Task<IReadOnlyList<Choice>> MediaAsync() => _media ??= LoadAsync("api/input/media");
    public static Task<IReadOnlyList<Choice>> MouseAsync() => _mouse ??= LoadAsync("api/input/mouse");
    public static Task<IReadOnlyList<Choice>> ControllerAsync() => _controller ??= LoadAsync("api/input/controller");
    private static Task<IReadOnlyList<Choice>>? _controller;

    public static async Task<string> NameAsync(int index)
    {
        var id = index.ToString();
        foreach (var list in new[] { KeyboardAsync(), MediaAsync(), MouseAsync(), ControllerAsync() })
        {
            if ((await list).FirstOrDefault(c => c.Value == id) is { } match)
            {
                return match.Label;
            }
        }
        return $"#{index}";
    }

    private static Task<Dictionary<int, string>>? _keyboardByCode;

    /// Keyboard action indices by Linux input event code.
    public static Task<Dictionary<int, string>> KeyboardByCodeAsync() => _keyboardByCode ??= LoadCodesAsync();

    private static async Task<Dictionary<int, string>> LoadCodesAsync()
    {
        var data = (await App.Api.GetAsync("api/input/keyboard"))?["data"] as JsonObject ?? new JsonObject();
        var result = new Dictionary<int, string>();
        foreach (var (index, entry) in data)
        {
            if (index != "0" && entry?["Media"]?.GetValue<bool>() != true && entry?["Mouse"]?.GetValue<bool>() != true
                && entry?["CommandCode"]?.GetValue<int>() is int code)
            {
                result.TryAdd(code, index);
            }
        }
        return result;
    }

    private static async Task<IReadOnlyList<Choice>> LoadAsync(string path)
    {
        var data = (await App.Api.GetAsync(path))?["data"] as JsonObject ?? new JsonObject();
        return data
            .Where(kv => kv.Key != "0")
            .Select(kv => new Choice(kv.Key, kv.Value?["Name"]?.GetValue<string>() ?? kv.Key))
            .OrderBy(c => int.TryParse(c.Value, out var n) ? n : 0)
            .ToList();
    }
}

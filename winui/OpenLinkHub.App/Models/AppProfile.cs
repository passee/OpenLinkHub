using System.Text.Json.Nodes;

namespace OpenLinkHub.App.Models;

public sealed class AppProfile
{
    public required string Id { get; init; }
    public required string Name { get; init; }
    public bool Enabled { get; init; }
    public bool Manual { get; init; }
    public Dictionary<string, string> ActiveProfiles { get; init; } = new();

    public static AppProfile FromJson(string id, JsonObject json) => new()
    {
        Id = id,
        Name = json["applicationName"]?.GetValue<string>() ?? id,
        Enabled = json["enabled"]?.GetValue<bool>() ?? false,
        Manual = json["manual"]?.GetValue<bool>() ?? false,
        ActiveProfiles = json["activeProfiles"]?.AsObject()
            .ToDictionary(kv => kv.Key, kv => kv.Value?.GetValue<string>() ?? "") ?? new(),
    };
}

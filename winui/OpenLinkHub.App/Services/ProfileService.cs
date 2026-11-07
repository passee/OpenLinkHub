using System.Text.Json.Nodes;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Services;

public sealed record ProfileInfo(string Name, int DeviceCount, int ActiveCount)
{
    public bool IsActiveEverywhere => ActiveCount > 0 && ActiveCount == DeviceCount;
    public string DisplayName => Name == "default" ? "Default" : Name;
}

/// iCUE treats a profile as global; OpenLinkHub stores one per device under a shared name.
public static class ProfileService
{
    public static async Task<(IReadOnlyList<ProfileInfo> Profiles, IReadOnlyDictionary<string, JsonObject> Details)> LoadAsync(
        IEnumerable<DeviceSummary> devices)
    {
        var details = new Dictionary<string, JsonObject>();
        var fetches = devices
            .Where(d => d.Category != DeviceCategory.Receiver)
            .Select(async d =>
            {
                try
                {
                    var detail = await App.Api.GetDeviceAsync(d.Serial);
                    if (detail != null)
                    {
                        lock (details)
                        {
                            details[d.Serial] = detail;
                        }
                    }
                }
                catch
                {
                }
            });
        await Task.WhenAll(fetches);

        var counts = new Dictionary<string, (int Devices, int Active)>(StringComparer.OrdinalIgnoreCase);
        foreach (var detail in details.Values)
        {
            if (detail["userProfiles"] is not JsonObject profiles)
            {
                continue;
            }

            var current = ActiveProfileName(detail);
            foreach (var (name, _) in profiles)
            {
                var active = string.Equals(name, current, StringComparison.OrdinalIgnoreCase);
                counts.TryGetValue(name, out var c);
                counts[name] = (c.Devices + 1, c.Active + (active ? 1 : 0));
            }
        }

        var list = counts
            .Select(kv => new ProfileInfo(kv.Key, kv.Value.Devices, kv.Value.Active))
            .OrderByDescending(p => p.Name == "default")
            .ThenBy(p => p.Name, StringComparer.OrdinalIgnoreCase)
            .ToList();
        return (list, details);
    }

    /// The per-profile "Active" flags are written once at load and go stale after a switch;
    /// the live DeviceProfile path is the reliable marker.
    public static string? ActiveProfileName(JsonObject detail)
    {
        if (detail["userProfiles"] is not JsonObject profiles)
        {
            return null;
        }
        var path = detail["DeviceProfile"]?["Path"]?.GetValue<string>();
        return profiles.FirstOrDefault(p => p.Value?["Path"]?.GetValue<string>() == path).Key ?? "default";
    }

    public static async Task<(int Ok, int Failed)> ActivateAsync(string profileName, IReadOnlyDictionary<string, JsonObject> details)
    {
        var targets = details
            .Where(kv => kv.Value["userProfiles"] is JsonObject p && p.ContainsKey(profileName))
            .Select(kv => kv.Key)
            .ToList();

        var results = await Task.WhenAll(targets.Select(serial => App.Api.SetUserProfileAsync(serial, profileName)));
        return (results.Count(r => r.Ok), results.Count(r => !r.Ok));
    }
}

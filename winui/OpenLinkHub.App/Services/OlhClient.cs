using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Nodes;
using OpenLinkHub.App.Models;

namespace OpenLinkHub.App.Services;

public sealed record ApiResult(bool Ok, string Message);

public sealed partial class OlhClient
{
    private static readonly JsonSerializerOptions JsonOptions = new() { PropertyNameCaseInsensitive = true };
    private readonly HttpClient _http;

    public OlhClient(string baseAddress = "http://127.0.0.1:27003/")
    {
        _http = new HttpClient { BaseAddress = new Uri(baseAddress), Timeout = TimeSpan.FromSeconds(8) };
    }

    public Uri BaseAddress => _http.BaseAddress!;

    public Uri WebUiDeviceUri(string serial) => new(BaseAddress, $"device/{serial}");

    public async Task<IReadOnlyList<DeviceSummary>> GetDevicesAsync(CancellationToken ct = default)
    {
        var root = await GetJsonAsync("api/", ct);
        var map = root?["device"]?.AsObject();
        if (map == null)
        {
            return Array.Empty<DeviceSummary>();
        }

        return map
            .Select(kv => DeviceSummary.FromJson(kv.Value!.AsObject()))
            .ToList();
    }

    public async Task<JsonObject?> GetDeviceAsync(string serial, CancellationToken ct = default)
    {
        var root = await GetJsonAsync($"api/devices/{Uri.EscapeDataString(serial)}", ct);
        return root?["device"] as JsonObject;
    }

    public async Task<IReadOnlyList<Choice>> GetRgbProfilesAsync(string serial, CancellationToken ct = default)
    {
        var root = await GetJsonAsync("api/color/", ct);
        var profiles = root?["data"]?[serial]?["profiles"]?.AsObject();
        return profiles?
            .Select(p => new Choice(p.Key, p.Value?["profileName"]?.GetValue<string>() is { Length: > 0 } name ? name : p.Key))
            .OrderBy(c => c.Label, StringComparer.OrdinalIgnoreCase)
            .ToList() ?? new List<Choice>();
    }

    public async Task<IReadOnlyList<string>> GetSpeedProfileNamesAsync(CancellationToken ct = default)
    {
        var root = await GetJsonAsync("api/temperatures/", ct);
        return root?["data"]?.AsObject().Select(p => p.Key).ToList() ?? new List<string>();
    }

    public async Task<double?> GetCpuTempAsync(CancellationToken ct = default) =>
        (await GetJsonAsync("api/cpuTemp/clean", ct))?["data"]?.GetValue<double>();

    public async Task<double?> GetGpuTempAsync(CancellationToken ct = default) =>
        (await GetJsonAsync("api/gpuTemp/clean", ct))?["data"]?.GetValue<double>();

    public async Task<Dictionary<string, int>> GetBatteryLevelsAsync(CancellationToken ct = default)
    {
        var data = (await GetJsonAsync("api/batteryStats", ct))?["data"]?.AsObject();
        return data?.ToDictionary(kv => kv.Key, kv => kv.Value?["Level"]?.GetValue<int>() ?? 0)
               ?? new Dictionary<string, int>();
    }

    public async Task<IReadOnlyList<AppProfile>> GetAppProfilesAsync(CancellationToken ct = default)
    {
        var data = (await GetJsonAsync("api/application/profiles", ct))?["data"]?.AsObject();
        return data?.Select(kv => AppProfile.FromJson(kv.Key, kv.Value!.AsObject())).ToList()
               ?? new List<AppProfile>();
    }

    public async Task<bool> GetProfileLockAsync(CancellationToken ct = default) =>
        (await GetJsonAsync("api/application/lock", ct))?["data"]?.GetValue<bool>() ?? false;

    public Task<ApiResult> SetProfileLockAsync(bool locked) =>
        SendAsync(HttpMethod.Post, "api/application/lock", new { locked });

    public Task<ApiResult> SetRgbProfileAsync(string serial, int channelId, string profile) =>
        SendAsync(HttpMethod.Post, "api/color", new { deviceId = serial, channelId, profile });

    public Task<ApiResult> SetSpeedProfileAsync(string serial, int channelId, string profile) =>
        SendAsync(HttpMethod.Post, "api/speed", new { deviceId = serial, channelId, profile });

    public Task<ApiResult> SetBrightnessAsync(string serial, int brightness) =>
        SendAsync(HttpMethod.Post, "api/brightness", new { deviceId = serial, brightness });

    public Task<ApiResult> SetBrightnessGradualAsync(string serial, int brightness) =>
        SendAsync(HttpMethod.Post, "api/brightness/gradual", new { deviceId = serial, brightness });

    public Task<ApiResult> SetUserProfileAsync(string serial, string userProfileName) =>
        SendAsync(HttpMethod.Post, "api/userProfile/change", new { deviceId = serial, userProfileName });

    public Task<ApiResult> SetPollingRateAsync(string serial, DeviceCategory category, int pollingRate) =>
        SendAsync(HttpMethod.Post, category == DeviceCategory.Mouse ? "api/mouse/pollingRate" : "api/keyboard/pollingRate",
            new { deviceId = serial, pollingRate });

    public Task<ApiResult> SetMouseDpiAsync(string serial, IDictionary<string, int> stages) =>
        SendAsync(HttpMethod.Post, "api/mouse/dpi", new { deviceId = serial, stages });

    public Task<ApiResult> SetSleepModeAsync(string serial, DeviceCategory category, int sleepMode) =>
        SendAsync(HttpMethod.Post, category switch
        {
            DeviceCategory.Headset => "api/headset/sleep",
            DeviceCategory.Controller => "api/controller/sleep",
            DeviceCategory.Keyboard => "api/keyboard/sleep",
            _ => "api/mouse/sleep",
        }, new { deviceId = serial, sleepMode });

    public Task<ApiResult> SetLcdModeAsync(string serial, int channelId, int lcdMode) =>
        SendAsync(HttpMethod.Post, "api/lcd", new { deviceId = serial, channelId, mode = lcdMode });

    public Task<ApiResult> SetLabelAsync(string serial, int channelId, string label, int deviceType = 0) =>
        SendAsync(HttpMethod.Post, "api/label", new { deviceId = serial, channelId, label, deviceType });

    public async Task<ApiResult> UploadLcdImageAsync(string filePath)
    {
        try
        {
            using var form = new MultipartFormDataContent();
            var file = new StreamContent(File.OpenRead(filePath));
            form.Add(file, "animationFile", Path.GetFileName(filePath));
            using var response = await _http.PostAsync("api/lcd/upload", form);
            var text = await response.Content.ReadAsStringAsync();
            if (!response.IsSuccessStatusCode)
            {
                return new ApiResult(false, text.Trim());
            }
            var node = JsonNode.Parse(text);
            return new ApiResult(node?["status"]?.GetValue<int>() == 1, node?["message"]?.GetValue<string>() ?? "Uploaded");
        }
        catch (Exception ex)
        {
            return new ApiResult(false, ex.Message);
        }
    }

    /// The colour bytes last sent to a device, in its driver's layout; null before the first frame.
    public async Task<byte[]?> GetLiveFrameAsync(string serial)
    {
        var data = (await GetJsonAsync($"api/devices/live/{Uri.EscapeDataString(serial)}", default))?["data"]?["data"] as JsonArray;
        return data?.Select(v => (byte)(v?.GetValue<int>() ?? 0)).ToArray();
    }

    public Task<JsonNode?> GetAsync(string path, CancellationToken ct = default) => GetJsonAsync(path, ct);

    public Task<ApiResult> PostAsync(string path, object body) => SendAsync(HttpMethod.Post, path, body);
    public Task<ApiResult> PutAsync(string path, object body) => SendAsync(HttpMethod.Put, path, body);
    public Task<ApiResult> DeleteAsync(string path, object body) => SendAsync(HttpMethod.Delete, path, body);

    private async Task<JsonNode?> GetJsonAsync(string path, CancellationToken ct)
    {
        using var response = await _http.GetAsync(path, ct);
        response.EnsureSuccessStatusCode();
        await using var stream = await response.Content.ReadAsStreamAsync(ct);
        return await JsonNode.ParseAsync(stream, cancellationToken: ct);
    }

    private async Task<ApiResult> SendAsync(HttpMethod method, string path, object body)
    {
        try
        {
            using var request = new HttpRequestMessage(method, path) { Content = JsonContent.Create(body, options: JsonOptions) };
            using var response = await _http.SendAsync(request);
            var node = JsonNode.Parse(await response.Content.ReadAsStringAsync());
            var ok = node?["status"]?.GetValue<int>() == 1;
            return new ApiResult(ok, node?["message"]?.GetValue<string>() ?? (ok ? "Saved" : "Request failed"));
        }
        catch (Exception ex)
        {
            return new ApiResult(false, ex.Message);
        }
    }
}

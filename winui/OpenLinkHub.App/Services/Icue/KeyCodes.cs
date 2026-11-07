namespace OpenLinkHub.App.Services.Icue;

/// iCUE names keys after their USB HID usage; OpenLinkHub colours keys by the same LED index
/// and maps actions with Linux input event codes.
public static class KeyCodes
{
    public static readonly IReadOnlyDictionary<string, int> HidUsage = BuildHid();

    /// HID keyboard usage to Linux input event code (drivers/hid/hid-input.c, hid_keyboard[]).
    public static readonly IReadOnlyDictionary<int, int> HidToEvdev = new Dictionary<int, int>
    {
        [0x04] = 30, [0x05] = 48, [0x06] = 46, [0x07] = 32, [0x08] = 18, [0x09] = 33, [0x0A] = 34, [0x0B] = 35,
        [0x0C] = 23, [0x0D] = 36, [0x0E] = 37, [0x0F] = 38, [0x10] = 50, [0x11] = 49, [0x12] = 24, [0x13] = 25,
        [0x14] = 16, [0x15] = 19, [0x16] = 31, [0x17] = 20, [0x18] = 22, [0x19] = 47, [0x1A] = 17, [0x1B] = 45,
        [0x1C] = 21, [0x1D] = 44, [0x1E] = 2, [0x1F] = 3, [0x20] = 4, [0x21] = 5, [0x22] = 6, [0x23] = 7,
        [0x24] = 8, [0x25] = 9, [0x26] = 10, [0x27] = 11, [0x28] = 28, [0x29] = 1, [0x2A] = 14, [0x2B] = 15,
        [0x2C] = 57, [0x2D] = 12, [0x2E] = 13, [0x2F] = 26, [0x30] = 27, [0x31] = 43, [0x32] = 43, [0x33] = 39,
        [0x34] = 40, [0x35] = 41, [0x36] = 51, [0x37] = 52, [0x38] = 53, [0x39] = 58, [0x3A] = 59, [0x3B] = 60,
        [0x3C] = 61, [0x3D] = 62, [0x3E] = 63, [0x3F] = 64, [0x40] = 65, [0x41] = 66, [0x42] = 67, [0x43] = 68,
        [0x44] = 87, [0x45] = 88, [0x46] = 99, [0x47] = 70, [0x48] = 119, [0x49] = 110, [0x4A] = 102, [0x4B] = 104,
        [0x4C] = 111, [0x4D] = 107, [0x4E] = 109, [0x4F] = 106, [0x50] = 105, [0x51] = 108, [0x52] = 103, [0x53] = 69,
        [0x54] = 98, [0x55] = 55, [0x56] = 74, [0x57] = 78, [0x58] = 96, [0x59] = 79, [0x5A] = 80, [0x5B] = 81,
        [0x5C] = 75, [0x5D] = 76, [0x5E] = 77, [0x5F] = 71, [0x60] = 72, [0x61] = 73, [0x62] = 82, [0x63] = 83,
        [0x64] = 86, [0x65] = 127,
        [0xE0] = 29, [0xE1] = 42, [0xE2] = 56, [0xE3] = 125, [0xE4] = 97, [0xE5] = 54, [0xE6] = 100, [0xE7] = 126,
    };

    private static Dictionary<string, int> BuildHid()
    {
        var map = new Dictionary<string, int>(StringComparer.Ordinal);
        for (var c = 'A'; c <= 'Z'; c++)
        {
            map[c.ToString()] = 0x04 + (c - 'A');
        }
        for (var d = 1; d <= 9; d++)
        {
            map[d.ToString()] = 0x1E + d - 1;
            map[$"Keypad{d}"] = 0x59 + d - 1;
        }
        map["0"] = 0x27;
        map["Keypad0"] = 0x62;
        for (var f = 1; f <= 12; f++)
        {
            map[$"F{f}"] = f <= 12 ? 0x3A + f - 1 : 0;
        }

        var named = new (string Name, int Usage)[]
        {
            ("Enter", 0x28), ("Escape", 0x29), ("Backspace", 0x2A), ("Tab", 0x2B), ("Space", 0x2C),
            ("MinusAndUnderscore", 0x2D), ("EqualsAndPlus", 0x2E), ("BracketLeft", 0x2F), ("BracketRight", 0x30),
            ("Backslash", 0x31), ("NonUsTilde", 0x32), ("SemicolonAndColon", 0x33), ("ApostropheAndDoubleQuote", 0x34),
            ("GraveAccentAndTilde", 0x35), ("CommaAndLessThan", 0x36), ("PeriodAndBiggerThan", 0x37),
            ("SlashAndQuestionMark", 0x38), ("CapsLock", 0x39), ("PrintScreen", 0x46), ("ScrollLock", 0x47),
            ("PauseBreak", 0x48), ("Insert", 0x49), ("Home", 0x4A), ("PageUp", 0x4B), ("Delete", 0x4C), ("End", 0x4D),
            ("PageDown", 0x4E), ("RightArrow", 0x4F), ("LeftArrow", 0x50), ("DownArrow", 0x51), ("UpArrow", 0x52),
            ("NumLock", 0x53), ("KeypadSlash", 0x54), ("KeypadAsterisk", 0x55), ("KeypadMinus", 0x56),
            ("KeypadPlus", 0x57), ("KeypadEnter", 0x58), ("KeypadPeriodAndDelete", 0x63), ("NonUsBackslash", 0x64),
            ("Application", 0x65), ("LeftCtrl", 0xE0), ("LeftShift", 0xE1), ("LeftAlt", 0xE2), ("LeftGui", 0xE3),
            ("RightCtrl", 0xE4), ("RightShift", 0xE5), ("RightAlt", 0xE6), ("RightGui", 0xE7),
        };
        foreach (var (name, usage) in named)
        {
            map[name] = usage;
        }
        return map;
    }
}

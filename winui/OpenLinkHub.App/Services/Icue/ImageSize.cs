using System.Buffers.Binary;

namespace OpenLinkHub.App.Services.Icue;

/// Reads pixel dimensions from a JPEG or PNG header, so renders can be laid out before they load.
public static class ImageSize
{
    public static (int Width, int Height) Read(string path)
    {
        var b = File.ReadAllBytes(path);
        if (b.Length > 24 && b[0] == 0x89 && b[1] == (byte)'P')
        {
            return ((int)BinaryPrimitives.ReadUInt32BigEndian(b.AsSpan(16)), (int)BinaryPrimitives.ReadUInt32BigEndian(b.AsSpan(20)));
        }
        var i = 2;
        while (i + 9 < b.Length)
        {
            if (b[i] != 0xFF)
            {
                i++;
                continue;
            }
            var marker = b[i + 1];
            if (marker is 0xC0 or 0xC1 or 0xC2)
            {
                return (BinaryPrimitives.ReadUInt16BigEndian(b.AsSpan(i + 7)), BinaryPrimitives.ReadUInt16BigEndian(b.AsSpan(i + 5)));
            }
            i += 2 + BinaryPrimitives.ReadUInt16BigEndian(b.AsSpan(i + 2));
        }
        return (0, 0);
    }
}

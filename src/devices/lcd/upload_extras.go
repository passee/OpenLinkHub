package lcd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
	"unicode"
)

// sanitizeImageName drops everything but letters and digits, since stored image names are
// matched against an alphanumeric pattern; "Arc Raiders" becomes "ArcRaiders".
func sanitizeImageName(name string) string {
	return strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return r
		}
		return -1
	}, name)
}

// extensionForContent maps a detected MIME type to the extension it is handled as, so a JPEG
// named .png or an icon renamed to .png still uploads.
func extensionForContent(mime string) (string, bool) {
	ext, ok := map[string]string{
		"image/gif":    ".gif",
		"image/jpeg":   ".jpg",
		"image/png":    ".png",
		"image/webp":   ".webp",
		"image/bmp":    ".bmp",
		"image/x-icon": ".ico",
	}[mime]
	return ext, ok
}

// convertedToJpg lists formats the LCD pipeline cannot read; they are stored as JPEG.
func convertedToJpg(ext string) bool {
	return ext == ".png" || ext == ".ico"
}

func decodeStill(ext string, r io.Reader) (image.Image, error) {
	if ext == ".ico" {
		return decodeIco(r)
	}
	return png.Decode(r)
}

// saveAsJpg stores an image as JPEG. Transparent areas turn black, matching the dark screen
// around the image.
func saveAsJpg(src image.Image, path string) error {
	flat := image.NewRGBA(src.Bounds())
	draw.Draw(flat, flat.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), src, src.Bounds().Min, draw.Over)

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if err = jpeg.Encode(out, flat, &jpeg.Options{Quality: 95}); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// decodeIco returns the largest image of a Windows icon. Entries are PNG or a 32-bit or 24-bit
// bottom-up bitmap whose stored height covers the colour rows plus the transparency mask.
func decodeIco(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) < 6 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("not an icon")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	var best []byte
	bestSize := -1
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		if entry+16 > len(data) {
			break
		}
		width := int(data[entry])
		if width == 0 {
			width = 256
		}
		size := int(binary.LittleEndian.Uint32(data[entry+8:]))
		offset := int(binary.LittleEndian.Uint32(data[entry+12:]))
		if offset < 0 || size <= 0 || offset+size > len(data) || width <= bestSize {
			continue
		}
		best, bestSize = data[offset:offset+size], width
	}
	if best == nil {
		return nil, errors.New("icon has no images")
	}
	if bytes.HasPrefix(best, []byte("\x89PNG")) {
		return png.Decode(bytes.NewReader(best))
	}
	return decodeIcoBitmap(best)
}

func decodeIcoBitmap(b []byte) (image.Image, error) {
	if len(b) < 40 {
		return nil, errors.New("icon bitmap too short")
	}
	headerSize := int(binary.LittleEndian.Uint32(b[0:]))
	width := int(int32(binary.LittleEndian.Uint32(b[4:])))
	height := int(int32(binary.LittleEndian.Uint32(b[8:]))) / 2
	bpp := int(binary.LittleEndian.Uint16(b[14:]))
	if width <= 0 || height <= 0 || width > 1024 || height > 1024 || (bpp != 32 && bpp != 24) {
		return nil, errors.New("unsupported icon bitmap")
	}
	stride := (width*bpp/8 + 3) &^ 3
	maskStride := ((width + 31) / 32) * 4
	pixels := b[headerSize:]
	if len(pixels) < stride*height {
		return nil, errors.New("icon bitmap truncated")
	}
	mask := pixels[stride*height:]
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := pixels[(height-1-y)*stride:]
		for x := 0; x < width; x++ {
			p := row[x*bpp/8:]
			alpha := uint8(255)
			if bpp == 32 {
				alpha = p[3]
			} else if maskRow := (height - 1 - y) * maskStride; maskRow+x/8 < len(mask) && mask[maskRow+x/8]&(0x80>>(x%8)) != 0 {
				alpha = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{R: p[2], G: p[1], B: p[0], A: alpha})
		}
	}
	return img, nil
}

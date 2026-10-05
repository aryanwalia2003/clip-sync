//go:build windows

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"
)

func TestHDropRoundTrip(t *testing.T) {
	b := hDrop(`C:\Users\a\Downloads\clipsync\मेरी File-1.pdf`)
	got := parseHDrop(b)
	if len(got) != 1 || got[0] != `C:\Users\a\Downloads\clipsync\मेरी File-1.pdf` {
		t.Errorf("parseHDrop = %q", got)
	}

	// do files, ANSI (fWide=0)
	ansi := make([]byte, dropFilesSize)
	ansi[0] = dropFilesSize
	ansi = append(ansi, "C:\\a.txt\x00D:\\b c.pdf\x00\x00"...)
	if got := parseHDrop(ansi); !slices.Equal(got, []string{`C:\a.txt`, `D:\b c.pdf`}) {
		t.Errorf("ansi parseHDrop = %q", got)
	}
	if parseHDrop(nil) != nil || parseHDrop([]byte{1, 2}) != nil {
		t.Error("kharab input pe nil aana chahiye")
	}
}

func TestDIBRoundTrip(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	src.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	src.SetNRGBA(2, 1, color.NRGBA{0, 0, 255, 128})
	p, err := dibToPNG(imageToDIB(src))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(p))
	if err != nil {
		t.Fatal(err)
	}
	for _, pt := range []image.Point{{0, 0}, {2, 1}, {1, 0}} {
		want := src.NRGBAAt(pt.X, pt.Y)
		got := color.NRGBAModel.Convert(img.At(pt.X, pt.Y)).(color.NRGBA)
		if want.A == 0 { // alpha sab 0 nahi hai, to transparent hi rehna chahiye
			if got.A != 0 {
				t.Errorf("%v: transparent pixel opaque ho gaya: %v", pt, got)
			}
			continue
		}
		if got != want {
			t.Errorf("%v: got %v want %v", pt, got, want)
		}
	}
	if _, err := dibToPNG([]byte("garbage")); err == nil {
		t.Error("garbage pe error aana chahiye")
	}
}

func TestDIBZeroAlphaIsOpaque(t *testing.T) {
	d := imageToDIB(image.NewNRGBA(image.Rect(0, 0, 2, 2))) // sab alpha 0
	p, err := dibToPNG(d)
	if err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(bytes.NewReader(p))
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0xffff {
		t.Errorf("alpha 0 wala DIB opaque hona chahiye, a=%x", a)
	}
}

func TestUTF16AndPSQuote(t *testing.T) {
	s := "héllo 😀 नमस्ते"
	if got := utf16Bytes(toUTF16Bytes(s)); got != s {
		t.Errorf("utf16 round trip = %q", got)
	}
	if got := psQuote(`it's $x `+"`"); got != `'it''s $x `+"`"+`'` {
		t.Errorf("psQuote = %s", got)
	}
}

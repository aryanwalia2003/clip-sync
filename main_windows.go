//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard           = user32.NewProc("OpenClipboard")
	procCloseClipboard          = user32.NewProc("CloseClipboard")
	procEmptyClipboard          = user32.NewProc("EmptyClipboard")
	procGetClipboardData        = user32.NewProc("GetClipboardData")
	procSetClipboardData        = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
	procGlobalSize   = kernel32.NewProc("GlobalSize")
)

const (
	cfDIB         = 8
	cfUnicodeText = 13
	cfHDrop       = 15
	gmemMoveable  = 0x0002
	dropFilesSize = 20 // DROPFILES struct
)

// Toast PowerShell se (koi module nahi chahiye), fail ho to chup-chaap skip.
// AppID PowerShell ka registered wala, warna Windows anjaan ID ka toast chup-chaap gira deta hai
func notify(msg string) {
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType=WindowsRuntime] | Out-Null
$x = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$t = $x.GetElementsByTagName('text')
$t.Item(0).AppendChild($x.CreateTextNode('clipsync')) | Out-Null
$t.Item(1).AppendChild($x.CreateTextNode(` + psQuote(msg) + `)) | Out-Null
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show([Windows.UI.Notifications.ToastNotification]::new($x))`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
}

// PowerShell single-quote string: andar ka ' double karo, $ aur ` literal rehte hain
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// doosri app ne clipboard pakda ho to thoda ruk ke retry
func openClipboard() error {
	for range 20 {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("clipboard busy hai")
}

func closeClipboard() { procCloseClipboard.Call() }

func cfPNG() uintptr {
	name, _ := syscall.UTF16PtrFromString("PNG")
	r, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	return r
}

// clipboard open hona chahiye. format na ho to nil
func getData(format uintptr) []byte {
	h, _, _ := procGetClipboardData.Call(format)
	if h == 0 {
		return nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(h)
	size, _, _ := procGlobalSize.Call(h)
	return bytes.Clone(unsafe.Slice((*byte)(globalPtr(p)), size))
}

// GlobalLock ka pointer Go heap ka nahi, GC ise nahi hilata (vet warning se bachne ka tareeka)
func globalPtr(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

// har format ke liye GlobalAlloc copy; SetClipboardData ke baad memory OS ki,
// isliye X11 jaisa process zinda rakhne ki zaroorat nahi
func setData(items map[uintptr][]byte) error {
	if err := openClipboard(); err != nil {
		return err
	}
	defer closeClipboard()
	procEmptyClipboard.Call()
	for format, data := range items {
		h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
		if h == 0 {
			return fmt.Errorf("GlobalAlloc fail")
		}
		p, _, _ := procGlobalLock.Call(h)
		if p == 0 {
			return fmt.Errorf("GlobalLock fail")
		}
		copy(unsafe.Slice((*byte)(globalPtr(p)), len(data)), data)
		procGlobalUnlock.Call(h)
		if r, _, err := procSetClipboardData.Call(format, h); r == 0 {
			return fmt.Errorf("SetClipboardData: %w", err)
		}
	}
	return nil
}

func readClip() string {
	if openClipboard() != nil {
		return ""
	}
	defer closeClipboard()
	return utf16Bytes(getData(cfUnicodeText))
}

// pehle null tak UTF-16LE decode
func utf16Bytes(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func toUTF16Bytes(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2+2) // + null terminator
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	return b
}

func writeClip(s string) error {
	// Windows apps CRLF expect karti hain (Notepad wagairah)
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	return setData(map[uintptr][]byte{cfUnicodeText: toUTF16Bytes(s)})
}

// pehle "PNG" format (browser/Office/WhatsApp), na mile to CF_DIB (screenshot, Paint)
func clipboardImage() ([]byte, string, bool) {
	if openClipboard() != nil {
		return nil, "", false
	}
	defer closeClipboard()
	if b := getData(cfPNG()); b != nil {
		return b, "clip.png", true
	}
	if b := getData(cfDIB); b != nil {
		if p, err := dibToPNG(b); err == nil {
			return p, "clip.png", true
		}
	}
	return nil, "", false
}

// dono formats daalo taaki naye aur purane dono tarah ke apps paste kar sakein
func writeImage(p []byte) error {
	img, err := png.Decode(bytes.NewReader(p))
	if err != nil {
		return err
	}
	return setData(map[uintptr][]byte{cfPNG(): p, cfDIB: imageToDIB(img)})
}

// BITMAPINFOHEADER + pixels. sirf uncompressed 24/32 bit (BI_RGB, BI_BITFIELDS BGRA)
func dibToPNG(b []byte) ([]byte, error) {
	if len(b) < 40 {
		return nil, fmt.Errorf("dib chhota hai")
	}
	hdr := int(binary.LittleEndian.Uint32(b[0:]))
	w := int(int32(binary.LittleEndian.Uint32(b[4:])))
	h := int(int32(binary.LittleEndian.Uint32(b[8:])))
	bpp := int(binary.LittleEndian.Uint16(b[14:]))
	comp := binary.LittleEndian.Uint32(b[16:])
	bottomUp := h > 0
	if h < 0 {
		h = -h
	}
	if w <= 0 || h == 0 || (bpp != 24 && bpp != 32) || (comp != 0 && comp != 3) {
		return nil, fmt.Errorf("dib format support nahi: %dbpp comp=%d", bpp, comp)
	}
	off := hdr
	if comp == 3 && hdr == 40 {
		off += 12 // BI_BITFIELDS ke 3 masks header ke baad
	}
	stride := (w*bpp/8 + 3) &^ 3
	if off+stride*h > len(b) {
		return nil, fmt.Errorf("dib adhoora hai")
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	hasAlpha := false
	for y := range h {
		sy := y
		if bottomUp {
			sy = h - 1 - y
		}
		row := b[off+sy*stride:]
		for x := range w {
			o := x * bpp / 8
			a := byte(255)
			if bpp == 32 {
				a = row[o+3]
				hasAlpha = hasAlpha || a != 0
			}
			img.SetNRGBA(x, y, color.NRGBA{row[o+2], row[o+1], row[o], a})
		}
	}
	if bpp == 32 && !hasAlpha { // zyada tar apps alpha 0 chhod dete hain, matlab opaque
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// 32bpp bottom-up BI_RGB
func imageToDIB(img image.Image) []byte {
	r := img.Bounds()
	w, h := r.Dx(), r.Dy()
	b := make([]byte, 40+w*h*4)
	binary.LittleEndian.PutUint32(b[0:], 40)
	binary.LittleEndian.PutUint32(b[4:], uint32(w))
	binary.LittleEndian.PutUint32(b[8:], uint32(h))
	binary.LittleEndian.PutUint16(b[12:], 1)
	binary.LittleEndian.PutUint16(b[14:], 32)
	binary.LittleEndian.PutUint32(b[20:], uint32(w*h*4))
	for y := range h {
		row := b[40+(h-1-y)*w*4:]
		for x := range w {
			c := color.NRGBAModel.Convert(img.At(r.Min.X+x, r.Min.Y+y)).(color.NRGBA)
			copy(row[x*4:], []byte{c.B, c.G, c.R, c.A})
		}
	}
	return b
}

// Explorer mein Ctrl+C ki hui files (CF_HDROP)
func copiedFiles() []string {
	if openClipboard() != nil {
		return nil
	}
	defer closeClipboard()
	return parseHDrop(getData(cfHDrop))
}

// DROPFILES header, phir null-separated paths, double null pe khatam
func parseHDrop(b []byte) []string {
	if len(b) < dropFilesSize {
		return nil
	}
	off := int(binary.LittleEndian.Uint32(b[0:]))
	wide := binary.LittleEndian.Uint32(b[16:]) != 0
	if off > len(b) {
		return nil
	}
	var paths []string
	rest := b[off:]
	for len(rest) > 0 {
		var p string
		if wide {
			p = utf16Bytes(rest)
			rest = rest[min(len(rest), len(utf16.Encode([]rune(p)))*2+2):]
		} else {
			n := bytes.IndexByte(rest, 0)
			if n < 0 {
				n = len(rest)
			}
			p, rest = string(rest[:n]), rest[min(len(rest), n+1):]
		}
		if p == "" {
			break
		}
		paths = append(paths, p)
	}
	return paths
}

func hDrop(path string) []byte {
	b := make([]byte, dropFilesSize)
	binary.LittleEndian.PutUint32(b[0:], dropFilesSize)
	binary.LittleEndian.PutUint32(b[16:], 1) // fWide
	b = append(b, toUTF16Bytes(path)...)
	return append(b, 0, 0) // list ka double null
}

// phone se aayi file Downloads\clipsync mein, clipboard mein "copied file" (Explorer/WhatsApp mein Ctrl+V)
func saveFileToClip(name string, data []byte) error {
	path, err := writeDownload(name, data)
	if err != nil {
		return err
	}
	return setData(map[uintptr][]byte{cfHDrop: hDrop(path)})
}

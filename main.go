// clipsync: "clipsync" = phone ka text suno, "clipsync send" = clipboard phone ko bhejo
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// ntfy ka message limit 4096 bytes hai
const maxLen = 4096

var (
	server = flag.String("server", "https://ntfy.sh", "ntfy server url")
)

func main() {
	flag.Parse()
	topic := loadTopic()
	if flag.Arg(0) == "send" {
		desc, err := send(topic)
		if err != nil {
			notify("Bhejna fail: " + err.Error())
			log.Fatal(err)
		}
		notify(desc)
		return
	}
	log.Printf("topic: %s (phone pe yahi use karo)", topic)
	subscribe(topic)
}

// desktop notification gdbus se (libnotify-bin ki zaroorat nahi), na ho to chup-chaap skip
func notify(msg string) {
	q := "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(msg) + "'" // GVariant string quote
	exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.Notifications", "--object-path", "/org/freedesktop/Notifications",
		"--method", "org.freedesktop.Notifications.Notify",
		"clipsync", "0", "", "clipsync", q, "[]", "{}", "5000").Run()
}

// hotkey se chalta hai, sirf jaan-boojh ke bhejna. files > image > text
func send(topic string) (string, error) {
	if files := copiedFiles(); len(files) > 0 {
		for _, p := range files {
			data, err := readFile(p)
			if err != nil {
				return "", err
			}
			if err := publishFile(topic, filepath.Base(p), data); err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("Phone ko bheji: %d file", len(files)), nil
	}
	for _, t := range []struct{ mime, name string }{{"image/png", "clip.png"}, {"image/jpeg", "clip.jpg"}} {
		if hasTarget(t.mime) {
			img, err := exec.Command("xclip", "-selection", "clipboard", "-t", t.mime, "-o").Output()
			if err != nil {
				return "", err
			}
			return "Phone ko bheji: image", publishFile(topic, t.name, img)
		}
	}
	cur := readClip()
	if cur == "" {
		return "", fmt.Errorf("clipboard khali hai")
	}
	if len(cur) > maxLen { // bada text attachment ban ke jata hai
		return fmt.Sprintf("Phone ko bheja: bada text (%d bytes)", len(cur)), publishFile(topic, "clip.txt", []byte(cur))
	}
	return fmt.Sprintf("Phone ko bheja: text (%d chars)", utf8.RuneCountInString(cur)), publish(topic, cur)
}

// nautilus ki "copied files" clipboard se file paths nikalo
func copiedFiles() []string {
	if !hasTarget("x-special/gnome-copied-files") {
		return nil
	}
	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "x-special/gnome-copied-files", "-o").Output()
	if err != nil {
		return nil
	}
	return parseCopiedFiles(string(out))
}

// format: pehli line "copy"/"cut", baaki file:// uri
func parseCopiedFiles(s string) []string {
	var paths []string
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for _, l := range lines[1:] {
		u, err := url.Parse(strings.TrimSpace(l))
		if err != nil || u.Scheme != "file" || u.Path == "" {
			continue
		}
		paths = append(paths, u.Path)
	}
	return paths
}

func readFile(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%s folder hai, sirf files bhej sakte hain", filepath.Base(path))
	}
	if fi.Size() > maxFile {
		return nil, fmt.Errorf("%s: %d bytes, limit %d", filepath.Base(path), fi.Size(), maxFile)
	}
	return os.ReadFile(path)
}

// topic file se padho, nahi mila to random bana ke save karo
func loadTopic() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(dir, "clipsync", "topic")
	if b, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(b))
	}
	r := make([]byte, 16)
	if _, err := rand.Read(r); err != nil {
		log.Fatal(err)
	}
	topic := "clip-" + hex.EncodeToString(r)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(topic), 0o600); err != nil {
		log.Fatal(err)
	}
	return topic
}

func readClip() string {
	out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
	if err != nil {
		return "" // clipboard khali ya non-text
	}
	return string(out)
}

func writeClip(s string) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-i")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

// clipboard mein ye target hai ya nahi
func hasTarget(mime string) bool {
	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
	if err != nil {
		return false
	}
	for _, l := range strings.Fields(string(out)) {
		if l == mime {
			return true
		}
	}
	return false
}

func writeImage(png []byte) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-i")
	cmd.Stdin = bytes.NewReader(png)
	return cmd.Run()
}

// ntfy attachment limit 15MB
const maxFile = 15 << 20

// laptop ke apne messages ka tag, daemon inhe skip karta hai (echo roko)
const selfTag = "from-laptop"

func publishFile(topic, name string, data []byte) error {
	if len(data) > maxFile {
		return fmt.Errorf("%s: %d bytes, limit %d", name, len(data), maxFile)
	}
	req, err := http.NewRequest(http.MethodPut, *server+"/"+topic, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Filename", name)
	req.Header.Set("Tags", selfTag)
	return do(req)
}

func publish(topic, text string) error {
	req, err := http.NewRequest(http.MethodPost, *server+"/"+topic, strings.NewReader(text))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Tags", selfTag)
	return do(req)
}

func do(req *http.Request) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}
	return nil
}

// ntfy stream sunke clipboard set karo, drop hone pe reconnect
func subscribe(topic string) {
	backoff := time.Second
	for {
		start := time.Now()
		if err := stream(topic); err != nil {
			log.Printf("subscribe: %v", err)
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second // lamba chala tha, backoff reset
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func stream(topic string) error {
	resp, err := http.Get(*server + "/" + topic + "/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		ev, ok := parseEvent(sc.Bytes())
		if !ok {
			continue
		}
		desc, err := apply(ev)
		if err != nil {
			log.Printf("clipboard write fail: %v", err)
			notify("Phone se aaya, par fail: " + err.Error())
			continue
		}
		notify(desc)
	}
	return sc.Err()
}

type event struct {
	Message string
	Att     *attachment
}

type attachment struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// json line se asli message/attachment nikalo (open/keepalive ignore)
func parseEvent(line []byte) (event, bool) {
	var ev struct {
		Event      string      `json:"event"`
		Message    string      `json:"message"`
		Attachment *attachment `json:"attachment"`
		Tags       []string    `json:"tags"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &ev); err != nil {
		return event{}, false
	}
	if ev.Event != "message" || (ev.Message == "" && ev.Attachment == nil) {
		return event{}, false
	}
	for _, t := range ev.Tags {
		if t == selfTag {
			return event{}, false // apna hi bheja hua
		}
	}
	return event{ev.Message, ev.Attachment}, true
}

// text seedha, attachment download karke clipboard mein. desc notification ke liye
func apply(ev event) (string, error) {
	if ev.Att == nil {
		return fmt.Sprintf("Phone se aaya: text (%d chars)", utf8.RuneCountInString(ev.Message)), writeClip(ev.Message)
	}
	resp, err := http.Get(ev.Att.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("attachment status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFile))
	if err != nil {
		return "", err
	}
	switch {
	case strings.HasPrefix(ev.Att.Type, "image/"):
		p, err := toPNG(data)
		if err != nil {
			return "", fmt.Errorf("%s: %w", ev.Att.Type, err) // HEIC jaisa format
		}
		return "Phone se aayi: image", writeImage(p)
	case strings.HasPrefix(ev.Att.Type, "text/"):
		return fmt.Sprintf("Phone se aaya: bada text (%d bytes)", len(data)), writeClip(string(data))
	}
	return "Phone se aayi: file " + filepath.Base(ev.Att.Name) + " (~/Downloads/clipsync)", saveFileToClip(ev.Att.Name, data)
}

// baaki files (pdf etc) ~/Downloads/clipsync mein save, clipboard mein "copied file"
func saveFileToClip(name string, data []byte) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Downloads", "clipsync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// naam ka basename + timestamp, taaki purani file overwrite na ho
	name = filepath.Base(name)
	ext := filepath.Ext(name)
	path := filepath.Join(dir, fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), time.Now().Unix(), ext))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: path}
	cmd := exec.Command("python3", "-c", fileClipPy, u.String())
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // clipboard owner badalne pe script khud band hoti hai
	return nil
}

// xclip ek hi format deta hai, isliye GTK4 se dono: nautilus wala aur browser wala (text/uri-list)
const fileClipPy = `
import sys, gi
gi.require_version("Gtk", "4.0")
from gi.repository import Gtk, Gdk, GLib
Gtk.init()
uri = sys.argv[1]
def prov(mime, text):
    return Gdk.ContentProvider.new_for_bytes(mime, GLib.Bytes.new(text.encode()))
cb = Gdk.Display.get_default().get_clipboard()
cb.set_content(Gdk.ContentProvider.new_union([
    prov("x-special/gnome-copied-files", "copy\n" + uri),
    prov("text/uri-list", uri + "\r\n")]))
loop = GLib.MainLoop()
cb.connect("changed", lambda c: None if c.is_local() else loop.quit())
loop.run()
`

// png/jpeg/gif ko png bana do, xclip apps png hi maante hain
func toPNG(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, []byte("\x89PNG")) {
		return data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return magickPNG(data) // HEIC jaise formats ke liye
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ImageMagick (heic support wala) se png banao
func magickPNG(data []byte) ([]byte, error) {
	cmd := exec.Command("convert", "-", "png:-")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("decode fail (imagemagick heic support chahiye): %w", err)
	}
	return out, nil
}

var _ = []any{gif.Decode, jpeg.Decode} // decoder register hone ke liye

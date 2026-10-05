//go:build linux

package main

import (
	"bytes"
	"net/url"
	"os/exec"
	"strings"
)

// desktop notification gdbus se (libnotify-bin ki zaroorat nahi), na ho to chup-chaap skip
func notify(msg string) {
	q := "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(msg) + "'" // GVariant string quote
	exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.Notifications", "--object-path", "/org/freedesktop/Notifications",
		"--method", "org.freedesktop.Notifications.Notify",
		"clipsync", "0", "", "clipsync", q, "[]", "{}", "5000").Run()
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

// clipboard mein image ho to raw bytes + attachment naam
func clipboardImage() ([]byte, string, bool) {
	for _, t := range []struct{ mime, name string }{{"image/png", "clip.png"}, {"image/jpeg", "clip.jpg"}} {
		if hasTarget(t.mime) {
			img, err := exec.Command("xclip", "-selection", "clipboard", "-t", t.mime, "-o").Output()
			if err != nil {
				return nil, "", false
			}
			return img, t.name, true
		}
	}
	return nil, "", false
}

func writeImage(png []byte) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-i")
	cmd.Stdin = bytes.NewReader(png)
	return cmd.Run()
}

// baaki files (pdf etc) ~/Downloads/clipsync mein save, clipboard mein "copied file"
func saveFileToClip(name string, data []byte) error {
	path, err := writeDownload(name, data)
	if err != nil {
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

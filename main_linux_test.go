//go:build linux

package main

import "testing"

func TestParseCopiedFiles(t *testing.T) {
	in := "copy\nfile:///home/a/My%20File.pdf\nfile:///tmp/x.txt\r\nnot-a-uri\n"
	got := parseCopiedFiles(in)
	want := []string{"/home/a/My File.pdf", "/tmp/x.txt"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("parseCopiedFiles = %q want %q", got, want)
	}
	if len(parseCopiedFiles("copy")) != 0 {
		t.Error("sirf action line pe koi path nahi aana chahiye")
	}
}

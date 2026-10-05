package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestParseEvent(t *testing.T) {
	cases := []struct {
		in   string
		want string
		att  string
		ok   bool
	}{
		{`{"event":"message","message":"hello"}`, "hello", "", true},
		{`{"event":"message","message":"You received a file: a.png","attachment":{"type":"image/png","url":"http://x/a.png"}}`, "You received a file: a.png", "http://x/a.png", true},
		{`{"event":"message","message":"mera","tags":["from-laptop"]}`, "", "", false},
		{`{"event":"message","message":"phone","tags":["other"]}`, "phone", "", true},
		{`{"event":"open"}`, "", "", false},
		{`{"event":"keepalive"}`, "", "", false},
		{`{"event":"message","message":""}`, "", "", false},
		{`not json`, "", "", false},
	}
	for _, c := range cases {
		got, ok := parseEvent([]byte(c.in))
		att := ""
		if got.Att != nil {
			att = got.Att.URL
		}
		if got.Message != c.want || att != c.att || ok != c.ok {
			t.Errorf("parseEvent(%q) = %q,%q,%v want %q,%q,%v", c.in, got.Message, att, ok, c.want, c.att, c.ok)
		}
	}
}

func TestToPNG(t *testing.T) {
	var jb bytes.Buffer
	if err := jpeg.Encode(&jb, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	p, err := toPNG(jb.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(p)); err != nil {
		t.Errorf("jpeg se png nahi bana: %v", err)
	}
	if _, err := toPNG([]byte("garbage")); err == nil {
		t.Error("garbage pe error aana chahiye")
	}
}

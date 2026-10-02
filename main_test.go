package main

import "testing"

func TestParseEvent(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{`{"event":"message","message":"hello"}`, "hello", true},
		{`{"event":"open"}`, "", false},
		{`{"event":"keepalive"}`, "", false},
		{`{"event":"message","message":""}`, "", false},
		{`not json`, "", false},
	}
	for _, c := range cases {
		got, ok := parseEvent([]byte(c.in))
		if got != c.want || ok != c.ok {
			t.Errorf("parseEvent(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

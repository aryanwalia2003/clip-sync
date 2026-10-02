// clipsync: Linux clipboard <-> ntfy.sh topic <-> iPhone
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ntfy ka message limit 4096 bytes hai
const maxLen = 4096

var (
	server = flag.String("server", "https://ntfy.sh", "ntfy server url")
	poll   = flag.Duration("poll", 500*time.Millisecond, "clipboard poll interval")

	mu   sync.Mutex
	last string // aakhri synced text, echo loop rokne ke liye
)

func main() {
	flag.Parse()
	topic := loadTopic()
	log.Printf("topic: %s (phone pe yahi subscribe karo)", topic)
	go subscribe(topic)
	watch(topic)
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

// laptop clipboard badle to ntfy pe bhejo
func watch(topic string) {
	mu.Lock()
	last = readClip() // startup pe purana content mat bhejo
	mu.Unlock()
	for range time.Tick(*poll) {
		cur := readClip()
		mu.Lock()
		skip := cur == "" || cur == last
		if !skip {
			last = cur
		}
		mu.Unlock()
		if skip {
			continue
		}
		if len(cur) > maxLen {
			log.Printf("skip: %d bytes, limit %d", len(cur), maxLen)
			continue
		}
		if err := publish(topic, cur); err != nil {
			log.Printf("publish fail: %v", err)
		}
	}
}

func publish(topic, text string) error {
	resp, err := http.Post(*server+"/"+topic, "text/plain", strings.NewReader(text))
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
		msg, ok := parseEvent(sc.Bytes())
		if !ok {
			continue
		}
		mu.Lock()
		same := msg == last
		if !same {
			last = msg
		}
		mu.Unlock()
		if same {
			continue // apna hi message wapas aaya
		}
		if err := writeClip(msg); err != nil {
			log.Printf("clipboard write fail: %v", err)
		}
	}
	return sc.Err()
}

// json line se sirf asli message nikalo (open/keepalive ignore)
func parseEvent(line []byte) (string, bool) {
	var ev struct {
		Event   string `json:"event"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &ev); err != nil {
		return "", false
	}
	if ev.Event != "message" || ev.Message == "" {
		return "", false
	}
	return ev.Message, true
}

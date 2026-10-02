# clipsync

Linux (X11) clipboard <-> iPhone, ntfy.sh ke through. Sirf Go stdlib + `xclip`.

## Laptop setup

```
sudo apt install xclip
go build -o ~/.local/bin/clipsync .
clipsync            # pehli run pe topic print hoga, ~/.config/clipsync/topic mein save
```

Auto-start:

```
mkdir -p ~/.config/systemd/user && cp clipsync.service ~/.config/systemd/user/
systemctl --user enable --now clipsync
```

## iPhone setup

1. App Store se **ntfy** install karo, topic subscribe karo (server `ntfy.sh`).
2. **iPhone -> laptop:** Shortcuts app mein naya shortcut:
   `Get Clipboard` -> `Get Contents of URL` (URL `https://ntfy.sh/<topic>`, Method `POST`, Request Body `File` = Clipboard).
   Isse Back Tap / Action Button / Share Sheet pe laga lo.
3. **Laptop -> iPhone:** ntfy notification aayegi, long-press/open karke text copy karo.

## Limitations

- iOS background mein clipboard read/write nahi karne deta, isliye phone side pe ek gesture lagta hai.
- Payload plaintext hai (TLS ke saath). Topic ka random naam hi secret hai, to share mat karo. ntfy.sh messages ~12h cache karta hai. Passwords copy karte ho to self-host ya encryption add karo.
- Sirf text, max 4096 bytes. Wayland pe `xclip` ki jagah `wl-clipboard` chahiye (abhi supported nahi).

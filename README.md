# clipsync

Linux (X11) clipboard <-> iPhone, ntfy.sh ke through. Sirf Go stdlib + `xclip`.

Design: normal copy **laptop pe hi rehti hai**. Phone ko bhejna ho to hotkey dabao
(`clipsync send`). Phone se aaya text apne aap laptop clipboard mein set ho jata hai.

## Laptop setup

```
sudo apt install xclip
go build -o ~/.local/bin/clipsync .
clipsync            # pehli run pe topic print hoga, ~/.config/clipsync/topic mein save
```

Daemon auto-start (phone ka text sunta hai):

```
mkdir -p ~/.config/systemd/user && cp clipsync.service ~/.config/systemd/user/
systemctl --user enable --now clipsync
```

Hotkey (laptop -> phone): Settings -> Keyboard -> Custom Shortcuts, naya shortcut:
command `~/.local/bin/clipsync send` (full path likhna), key `Ctrl+Alt+V`.

## iPhone setup

1. **Phone -> laptop:** Shortcuts app mein naya shortcut:
   `Get Clipboard` -> `Get Contents of URL` (URL `https://ntfy.sh/<topic>`, Method `POST`, Request Body `File` = Clipboard).
   Isse Back Tap / Action Button / Share Sheet pe laga lo.
2. **Laptop -> phone:** App Store se **ntfy** install karo, topic subscribe karo (server `ntfy.sh`).
   Hotkey dabane pe notification aayegi, text long-press karke copy karo.

Topic kisi se share mat karo, wahi secret hai.

## Limitations

- iOS background mein clipboard read/write nahi karne deta, isliye phone side pe ek gesture lagta hai.
- Jo bhejte ho wo plaintext hai (TLS ke saath) aur ntfy.sh pe ~12h cache rehta hai. Isliye sirf hotkey se wahi bhejo jo share karna hai.
- Text max 4096 bytes. Images (PNG/JPEG, max 15MB) bhi chalte hain, ntfy attachment ban ke jaate hain. Phone se JPEG/GIF aaye to laptop pe PNG mein convert hoke clipboard mein jata hai. iPhone ki HEIC images ke liye ImageMagick chahiye (`convert` mein heic support, Pop!_OS pe pehle se hota hai).
- Phone se PDF/doc jaisi files `~/Downloads/clipsync/` mein save hoti hain aur clipboard mein "copied file" ban jaati hain. File manager (Nautilus) ya browser (WhatsApp Web) mein Ctrl+V se paste karo. Iske liye `python3` + GTK4 (`gir1.2-gtk-4.0`) chahiye. Sirf phone -> laptop; laptop se file bhejna abhi nahi hai.
- Laptop -> phone image: ntfy app mein image aati hai, kholke Share -> Copy (ya Save to Photos). Kuch bhi phone clipboard mein apne aap nahi jata.
- Wayland pe `xclip` ki jagah `wl-clipboard` chahiye (abhi supported nahi).

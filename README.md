# clipsync

Linux (X11) / Windows clipboard <-> iPhone, ntfy.sh ke through. Sirf Go stdlib (Linux pe `xclip` bhi).

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

## Windows setup

Kuch install nahi karna (clipboard Win32 API se, notification PowerShell toast se).

```
go build -ldflags -H=windowsgui -o %LOCALAPPDATA%\clipsync\clipsync.exe .
```

`-H=windowsgui` se console window nahi khulti (na daemon ke liye, na hotkey pe). Isliye topic print nahi dikhega:
ek baar exe chala ke band karo, phir `type %APPDATA%\clipsync\topic` se topic dekho.

- **Daemon auto-start:** `Win+R` -> `shell:startup`, wahan `clipsync.exe` ka shortcut bana do.
- **Hotkey (laptop -> phone):** Start Menu folder mein shortcut banao (`Win+R` -> `shell:programs`),
  Target `"%LOCALAPPDATA%\clipsync\clipsync.exe" send`, Properties -> Shortcut key `Ctrl+Alt+V`.
  (Shortcut key sirf Desktop / Start Menu wale shortcuts pe chalti hai.)
- Explorer mein Ctrl+C ki hui files bhi hotkey se jaati hain. Phone se aayi files `Downloads\clipsync\` mein save
  hoke clipboard mein "copied file" ban jaati hain, Explorer / WhatsApp Desktop / browser mein Ctrl+V karo.
- Images "PNG" aur bitmap dono formats mein clipboard mein jaati hain, to browser, Office, Paint sab mein paste hoti hain.
- iPhone ki HEIC images ke liye ImageMagick chahiye: `winget install ImageMagick.ImageMagick` (`magick` PATH mein hona chahiye).

## iPhone setup

1. **Phone -> laptop:** Shortcuts app mein naya shortcut:
   `Get Clipboard` -> `Get Contents of URL` (URL `https://ntfy.sh/<topic>`, Method `POST`, Request Body `File` = Clipboard).
   Isse Back Tap / Action Button / Share Sheet pe laga lo.
2. **Laptop -> phone:** App Store se **ntfy** install karo, topic subscribe karo (server `ntfy.sh`).
   Hotkey dabane pe notification aayegi, text long-press karke copy karo.

Topic kisi se share mat karo, wahi secret hai.

Har bhejne/aane pe desktop notification aati hai (`gdbus` se, alag install nahi). Laptop ke apne messages pe `from-laptop` tag lagta hai taaki daemon unhe wapas na utha le (ek hi laptop per topic).

## Limitations

- iOS background mein clipboard read/write nahi karne deta, isliye phone side pe ek gesture lagta hai.
- Jo bhejte ho wo plaintext hai (TLS ke saath) aur ntfy.sh pe ~12h cache rehta hai. Isliye sirf hotkey se wahi bhejo jo share karna hai.
- Hotkey (`clipsync send`) ka priority: Nautilus mein copy ki hui files > image > text. Bada text (4096 bytes se upar) `.txt` attachment ban ke jata hai. Folder nahi bhej sakte, sirf files. Max 15MB. Images (PNG/JPEG, max 15MB) bhi chalte hain, ntfy attachment ban ke jaate hain. Phone se JPEG/GIF aaye to laptop pe PNG mein convert hoke clipboard mein jata hai. iPhone ki HEIC images ke liye ImageMagick chahiye (`convert` mein heic support, Pop!_OS pe pehle se hota hai).
- Phone se PDF/doc jaisi files `~/Downloads/clipsync/` mein save hoti hain aur clipboard mein "copied file" ban jaati hain. File manager (Nautilus) ya browser (WhatsApp Web) mein Ctrl+V se paste karo. Iske liye `python3` + GTK4 (`gir1.2-gtk-4.0`) chahiye.
- Laptop -> phone image: ntfy app mein image aati hai, kholke Share -> Copy (ya Save to Photos). Kuch bhi phone clipboard mein apne aap nahi jata.
- Upar ki Nautilus / GTK4 / `~/Downloads` wali baatein Linux ki hain, Windows ka hisaab "Windows setup" mein hai.
- Wayland pe `xclip` ki jagah `wl-clipboard` chahiye (abhi supported nahi).

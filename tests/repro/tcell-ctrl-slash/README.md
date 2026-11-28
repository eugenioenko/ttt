# Ctrl+/ produces no EventKey in Windows Terminal's win32-input-mode

With tcell v3.4.1, pressing left Ctrl+/ delivers a Win32 key-down report to
`Tty.Read`, but no corresponding `EventKey` reaches the application. The same
keypress was captured in a local Windows session and over SSH. This standalone
module uses stock tcell and forwards each read unchanged. [Downstream report](https://github.com/eugenioenko/ttt/issues/1671)

## Steps to reproduce

1. Open a PowerShell tab in Windows Terminal. With Go 1.25 or later, change to
   `tests/repro/tcell-ctrl-slash` and run `go run .`. You can also copy this
   directory on its own.
2. At the prompt, tap the main keyboard's `/` key, hold left Ctrl and tap the
   same key, release both keys, then tap `q`. The program restores the screen
   and prints the raw reads and the key events. The first line shows whether
   tcell selected win32-input-mode (DEC private mode 9001).
3. For the SSH comparison, copy this directory to a host with Go and connect
   from the same Windows Terminal with `ssh -t user@host`. Run `go run .` in
   the remote directory and repeat the keystrokes from step 2. To save the
   trace, run `go run . > trace.txt` inside the session.

## Observed

Both captures used Windows 11 Pro 24H2 (build 26100.4946), Windows Terminal
1.25.3131.0, and tcell v3.4.1 built with Go 1.26.1. Tcell negotiated the
keyboard protocol automatically and used its default key-event reporting.

| Session | Host running the probe | win32-input-mode | Key events for `/`, left Ctrl+/, `q` |
| --- | --- | --- | --- |
| Local | Windows x64 | `true` | `Rune[/]`, none, `Rune[q]` |
| SSH (OpenSSH for Windows 9.5p1) | Ubuntu 22.04.3 LTS, Linux 5.10.0+ arm64, `TERM=dumb` | `true` | `Rune[/]`, none, `Rune[q]` |

The local capture included these reads and key events for the three keys.

```text
read "\x1b[191;53;47;1;0;1_"
key Rune[/] str="/"
read "\x1b[191;53;0;1;8;1_"
read "\x1b[191;53;0;0;8;1_"
read "\x1b[81;16;113;1;0;1_"
key Rune[q] str="q"
```

Over SSH, the final byte of the Ctrl+/ key-down report arrived in a second read.

```text
read "\x1b[191;53;0;1;8;1"
read "_"
```

The joined report has `Vk=191` (`VK_OEM_2`), scan code 53, Unicode character
0, key down 1, left Ctrl 8, and repeat count 1.

## Expected

The Ctrl+/ press should produce one `EventKey` that identifies the slash
shortcut with `ModCtrl`, so an application can bind it separately from plain
`/`.

## Replay without Windows Terminal

Run `go run . -replay` from this directory. It feeds the three key-down reports
to a tcell screen through `vt.MockTerm.SendRaw`. The final `q` marks the end
of the input, so detecting the missing event requires no timeout.

```text
win32-input-mode=true
read "\x1b[191;53;47;1;0;1_\x1b[191;53;0;1;8;1_\x1b[81;16;113;1;0;1_"
key Rune[/] str="/"
key Rune[q] str="q"
```

Replaying these bytes against tcell v3.6.0 in a separate copy of the module
produced the same two events.

# ToothTray-go

[English](README_en.md) | [Русский](README.md)

[![GitHub Repository](https://img.shields.io/badge/GitHub-internetson%2FToothTray--go-181717?logo=github)](https://github.com/internetson/ToothTray-go)
[![Release](https://img.shields.io/github/v/release/internetson/ToothTray-go?logo=github)](https://github.com/internetson/ToothTray-go/releases)
[![Arch](https://img.shields.io/badge/Arch-x64%20%7C%20ARM64-orange)](https://github.com/internetson/ToothTray-go/releases)
[![License](https://img.shields.io/badge/License-BSD_2--Clause-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.18+-00ADD8?logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows_10%20%2F%2011-0078D6?logo=windows)](https://microsoft.com/windows)

Repository: **[https://github.com/internetson/ToothTray-go](https://github.com/internetson/ToothTray-go)**

**ToothTray-go** is a modern, fast, and enhanced reimplementation of ToothTray written in **Go**.

ToothTray-go resides quietly in the Windows notification area (System Tray) and enables instant one-click connection and disconnection of paired Bluetooth devices without opening Windows Settings or action center panels.

---

## 📌 Attribution & Original Project

This project is a complete Go rewrite and evolution of the original **[ToothTray](https://github.com/m2jean/ToothTray)** project created by **[Jing Xia (m2jean)](https://github.com/m2jean)** and released under the **BSD 2-Clause License**.

The core discovery of directly commanding Bluetooth drivers through the Windows Core Audio Device Topology API (`IKsControl`, `KSPROPSETID_BtAudio`) belongs to the original author. This **ToothTray-go** implementation builds upon that foundation, overcomes the limitations of the C++ original, and introduces numerous user-experience enhancements.

---

## 🚀 Advantages of ToothTray-go Over Original ToothTray (C++)

| Feature / Aspect | Original (m2jean/ToothTray, C++) | ToothTray-go |
| :--- | :--- | :--- |
| **Dependencies & Runtime** | Requires Visual Studio, C++/WinRT, NuGet package `wil`, and VC++ redistributables (`msvcp*.dll`) | **Pure Go (Zero CGO)**. No external DLLs, C++ runtimes, or build-time compilers needed. Compiles into a single self-contained `.exe` (~2.3 MB). |
| **UI Responsiveness** | Synchronous `KsProperty` calls on the window thread — slow driver responses froze the tray menu | **Fully asynchronous**. Connection and disconnection execute in background goroutines; the popup menu closes instantly. |
| **User Feedback (Notifications)** | None (logs only emitted to `OutputDebugString`) | **Native Windows tray balloons / notifications**. Provides real-time feedback: "Connecting to...", "Connected", "Disconnected", "Error". |
| **Connection State Accuracy** | Checked based only on endpoint active state | **Dual-source status verification**. Correlates Win32 Bluetooth API with Core Audio endpoints; connected devices are clearly marked with `✓`. |
| **Device Support** | Audio endpoints only | **All paired Bluetooth devices**. Audio devices leverage fast Kernel Streaming; non-audio devices fall back to Bluetooth service state management. |
| **Architectures** | x86 / x64 builds only | **x64 (amd64) and native ARM64** (Snapdragon X Elite, Surface Pro ARM, etc.). |
| **Autostart with Windows** | None (manual creation of shortcuts in `shell:startup` was required) | **Built-in "Start with Windows" option**. Toggles startup registration with a single click via registry (`HKCU\...\Run`). |
| **No Console Popups** | Dependent on Visual Studio project subsystem configuration | **Guaranteed zero console windows**. Built with `-H windowsgui`, preventing command prompt windows from ever appearing. |
| **Duplicate Prevention** | Could spawn multiple tray icons if executed repeatedly | **Windows System Mutex**. Subsequent launches terminate quietly without littering duplicate tray icons. |
| **Quick Access to Settings** | None | **"Bluetooth Settings..." menu item**, opening `ms-settings:bluetooth` directly via Shell API without cmd prompt popups. |

---

## 💡 How It Works

Windows typically provides two ways to connect Bluetooth audio devices: the "Connect" flyout (`Win + K`) and the "Bluetooth & other devices" settings page. Both require multiple clicks and waiting.

Windows does not provide a standard public high-level API to connect Bluetooth audio devices programmatically.

**ToothTray-go** uses a low-level kernel streaming mechanism:
1. Enumerates audio render endpoints using **Windows Core Audio API** (`IMMDeviceEnumerator`).
2. Navigates the audio graph using the **Device Topology API** (`IDeviceTopology`, `IConnector`, `IPart`) to find the Kernel Streaming (KS) filter connected to the endpoint (matched by `{2}.\\?\bth` — `bthenum` / `bthhfenum`).
3. Sends a property request to the filter's `IKsControl` interface using the `KSPROPSETID_BtAudio` GUID (`{7FA06C40-B8F6-4C7E-8556-E8C33A12E54D}`):
   * `KSPROPERTY_ONESHOT_RECONNECT` (0) — triggers the driver to connect to the Bluetooth device;
   * `KSPROPERTY_ONESHOT_DISCONNECT` (1) — triggers the driver to disconnect from the device.
4. Composite headsets featuring multiple profiles (Stereo A2DP and Hands-Free HFP) are grouped by `ContainerId` and toggled together.

---

## 🎮 Usage

1. Launch `ToothTray-go.exe` (or `ToothTray-go-arm64.exe` on ARM64). A Bluetooth icon will appear in your system tray (near the clock).
2. **Click the tray icon** (right-click or left-click):
   * `✓ Device Name` — device is currently connected (click to disconnect).
   * `  Device Name` — device is disconnected (click to connect).
3. **Refresh List** — rescans Bluetooth devices.
4. **Bluetooth Settings...** — opens standard Windows Bluetooth settings.
5. **Start with Windows** — toggles application startup on boot.
6. **Exit** — quits the program and removes the tray icon.

---

## 🔨 Building from Source

### Prerequisites:
* [Go](https://go.dev/) (version 1.18 or newer).
* C compiler is **not required** (Pure Go).

### Clone and Quick Build (x64 and ARM64):
```cmd
git clone https://github.com/internetson/ToothTray-go.git
cd ToothTray-go
build.bat
```
The `build.bat` script automatically compiles both binaries:
* `ToothTray-go.exe` — for Windows x64 (amd64)
* `ToothTray-go-arm64.exe` — for Windows ARM64 (Snapdragon X Elite, Surface Pro ARM, etc.)

### Build manually:

**For x64 (amd64):**
```cmd
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-H windowsgui -s -w" -o ToothTray-go.exe .
```

**For ARM64:**
```cmd
set GOOS=windows
set GOARCH=arm64
go build -ldflags="-H windowsgui -s -w" -o ToothTray-go-arm64.exe .
```

*To embed the application icon into the executables:*
```cmd
go install github.com/akavel/rsrc@latest
rsrc -arch amd64 -ico ToothTray.ico -o rsrc_windows_amd64.syso
rsrc -arch arm64 -ico ToothTray.ico -o rsrc_windows_arm64.syso
```

---

## 📄 License and Copyright

This project is licensed under the **BSD 2-Clause License**:

* Copyright (c) 2023, **Jing Xia** (original ToothTray author: [github.com/m2jean/ToothTray](https://github.com/m2jean/ToothTray))
* Copyright (c) 2026, **ToothTray-go Contributors** ([github.com/internetson/ToothTray-go](https://github.com/internetson/ToothTray-go))

See the full license text in [LICENSE.md](LICENSE.md).

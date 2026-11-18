package tray

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/internetson/ToothTray-go/pkg/autostart"
	"github.com/internetson/ToothTray-go/pkg/bluetooth"
	"github.com/internetson/ToothTray-go/pkg/config"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostMessage         = user32.NewProc("PostMessageW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procLoadImage           = user32.NewProc("LoadImageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procTrackPopupMenuEx    = user32.NewProc("TrackPopupMenuEx")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")

	procShell_NotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	procShellExecute     = shell32.NewProc("ShellExecuteW")
)

const (
	WM_NULL        = 0x0000
	WM_DESTROY     = 0x0002
	WM_COMMAND     = 0x0111
	WM_USER        = 0x0400
	WM_TRAYICON    = WM_USER + 1
	WM_LBUTTONUP   = 0x0202
	WM_RBUTTONUP   = 0x0205
	WM_CONTEXTMENU = 0x007B
	NIN_SELECT     = 0x0400
	NIN_KEYSELECT  = 0x0401

	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIF_MESSAGE    = 0x00000001
	NIF_ICON       = 0x00000002
	NIF_TIP        = 0x00000004
	NIF_INFO       = 0x00000010
	NIIF_NONE      = 0x00000000
	NIIF_INFO      = 0x00000001
	NIIF_WARNING   = 0x00000002
	NIIF_ERROR     = 0x00000003

	MF_STRING      = 0x00000000
	MF_CHECKED     = 0x00000008
	MF_UNCHECKED   = 0x00000000
	MF_GRAYED      = 0x00000001
	MF_DISABLED    = 0x00000002
	MF_SEPARATOR   = 0x00000800

	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	IMAGE_ICON       = 1
	LR_LOADFROMFILE  = 0x00000010
	LR_DEFAULTSIZE   = 0x00000040
	IDI_APPLICATION  = 32512

	ID_DEVICE_BASE = 1000
	ID_REFRESH     = 2001
	ID_SETTINGS    = 2002
	ID_NOTIFS      = 2003
	ID_AUTOSTART   = 2004
	ID_EXIT        = 2005
)

type POINT struct {
	X, Y int32
}

type MSG struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type NOTIFYICONDATAW struct {
	CbSize            uint32
	_                 uint32
	HWnd              windows.Handle
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	_                 uint32
	HIcon             windows.Handle
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          windows.GUID
	HBalloonIcon      windows.Handle
}

type App struct {
	hWnd      windows.Handle
	hInstance windows.Handle
	hIcon     windows.Handle
	btManager *bluetooth.Manager
	cfg       *config.Config
}

var globalApp *App

func NewApp() *App {
	return &App{
		btManager: bluetooth.NewManager(),
		cfg:       config.Load(),
	}
}

func (a *App) Run() error {
	globalApp = a

	hInst, _, _ := procGetModuleHandle.Call(0)
	a.hInstance = windows.Handle(hInst)

	// Load Icon
	a.hIcon = a.loadAppIcon()

	className := windows.StringToUTF16Ptr("ToothTrayGoHiddenWindowClass")
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     a.hInstance,
		HIcon:         a.hIcon,
		LpszClassName: className,
	}

	atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassEx failed: %v", err)
	}

	hwnd, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("ToothTray-go"))),
		0,
		0, 0, 0, 0,
		0,
		0,
		uintptr(a.hInstance),
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx failed: %v", err)
	}
	a.hWnd = windows.Handle(hwnd)

	// Add Tray Icon
	if err := a.addTrayIcon(); err != nil {
		return err
	}
	defer a.removeTrayIcon()

	// Windows Message Loop
	var msg MSG
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}

	return nil
}

func (a *App) loadAppIcon() windows.Handle {
	// 1. Try loading embedded resource icon (from rsrc.syso, resource ID 1)
	hIcon, _, _ := procLoadIcon.Call(uintptr(a.hInstance), 1)
	if hIcon != 0 {
		return windows.Handle(hIcon)
	}

	// 2. Try loading ToothTray.ico from executable's directory or current directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		icoFile := filepath.Join(exeDir, "ToothTray.ico")
		if _, err := os.Stat(icoFile); err == nil {
			h, _, _ := procLoadImage.Call(
				0,
				uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(icoFile))),
				IMAGE_ICON,
				0, 0,
				LR_LOADFROMFILE|LR_DEFAULTSIZE,
			)
			if h != 0 {
				return windows.Handle(h)
			}
		}
	}

	if _, err := os.Stat("ToothTray.ico"); err == nil {
		h, _, _ := procLoadImage.Call(
			0,
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("ToothTray.ico"))),
			IMAGE_ICON,
			0, 0,
			LR_LOADFROMFILE|LR_DEFAULTSIZE,
		)
		if h != 0 {
			return windows.Handle(h)
		}
	}

	// 3. Fallback to standard application icon
	h, _, _ := procLoadIcon.Call(0, uintptr(IDI_APPLICATION))
	return windows.Handle(h)
}

func (a *App) addTrayIcon() error {
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hWnd
	nid.UID = 1
	nid.UFlags = NIF_ICON | NIF_MESSAGE | NIF_TIP
	nid.UCallbackMessage = WM_TRAYICON
	nid.HIcon = a.hIcon
	copy(nid.SzTip[:], windows.StringToUTF16("ToothTray-go (Bluetooth)"))

	ret, _, err := procShell_NotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIcon NIM_ADD failed: %v", err)
	}
	return nil
}

func (a *App) removeTrayIcon() {
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hWnd
	nid.UID = 1
	procShell_NotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
}

func (a *App) ShowNotification(title, message string, isError bool) {
	if a.cfg != nil && !a.cfg.IsNotificationsEnabled() {
		return
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hWnd
	nid.UID = 1
	nid.UFlags = NIF_INFO
	copy(nid.SzInfoTitle[:], windows.StringToUTF16(title))
	copy(nid.SzInfo[:], windows.StringToUTF16(message))
	if isError {
		nid.DwInfoFlags = NIIF_ERROR
	} else {
		nid.DwInfoFlags = NIIF_INFO
	}
	nid.UTimeoutOrVersion = 3000

	procShell_NotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&nid)))
}

func wndProc(hWnd windows.Handle, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_RBUTTONUP, WM_LBUTTONUP, WM_CONTEXTMENU, NIN_SELECT, NIN_KEYSELECT:
			if globalApp != nil {
				globalApp.showPopupMenu()
			}
			return 0
		}
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(uintptr(hWnd), uintptr(msg), wParam, lParam)
	return ret
}

func (a *App) showPopupMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	// Fetch paired devices
	devices, err := a.btManager.GetDevices()

	if err != nil || len(devices) == 0 {
		procAppendMenu.Call(
			hMenu,
			uintptr(MF_STRING|MF_DISABLED),
			0,
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Нет сохраненных устройств"))),
		)
	} else {
		for i, dev := range devices {
			flags := uint32(MF_STRING)
			if dev.Connected {
				flags |= MF_CHECKED
			} else {
				flags |= MF_UNCHECKED
			}

			dispName := dev.Name
			if dev.Connected {
				dispName = "✓ " + dev.Name
			} else {
				dispName = "   " + dev.Name
			}

			cmdID := uintptr(ID_DEVICE_BASE + i)
			procAppendMenu.Call(
				hMenu,
				uintptr(flags),
				cmdID,
				uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(dispName))),
			)
		}
	}

	// Separator
	procAppendMenu.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)

	// Refresh
	procAppendMenu.Call(
		hMenu,
		uintptr(MF_STRING),
		ID_REFRESH,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Обновить список"))),
	)

	// Bluetooth Settings
	procAppendMenu.Call(
		hMenu,
		uintptr(MF_STRING),
		ID_SETTINGS,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Параметры Bluetooth..."))),
	)

	// Notifications toggle
	notifFlags := uint32(MF_STRING)
	if a.cfg.IsNotificationsEnabled() {
		notifFlags |= MF_CHECKED
	}
	procAppendMenu.Call(
		hMenu,
		uintptr(notifFlags),
		ID_NOTIFS,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Всплывающие уведомления"))),
	)

	// Autostart toggle
	autoFlags := uint32(MF_STRING)
	if autostart.IsEnabled() {
		autoFlags |= MF_CHECKED
	}
	procAppendMenu.Call(
		hMenu,
		uintptr(autoFlags),
		ID_AUTOSTART,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Запуск вместе с Windows"))),
	)

	// Separator
	procAppendMenu.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)

	// Exit
	procAppendMenu.Call(
		hMenu,
		uintptr(MF_STRING),
		ID_EXIT,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Выход"))),
	)

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Set foreground window before displaying popup menu so clicking outside closes it
	procSetForegroundWindow.Call(uintptr(a.hWnd))

	cmd, _, _ := procTrackPopupMenuEx.Call(
		hMenu,
		uintptr(TPM_RIGHTBUTTON|TPM_RETURNCMD),
		uintptr(pt.X),
		uintptr(pt.Y),
		uintptr(a.hWnd),
		0,
	)

	procPostMessage.Call(uintptr(a.hWnd), WM_NULL, 0, 0)

	if cmd == 0 {
		return // user dismissed menu
	}

	a.handleMenuCommand(int(cmd), devices)
}

func (a *App) handleMenuCommand(cmd int, devices []*bluetooth.Device) {
	switch {
	case cmd >= ID_DEVICE_BASE && cmd < ID_DEVICE_BASE+len(devices):
		dev := devices[cmd-ID_DEVICE_BASE]
		wasConnected := dev.Connected

		if wasConnected {
			a.ShowNotification("ToothTray-go", fmt.Sprintf("Отключение от %s...", dev.Name), false)
		} else {
			a.ShowNotification("ToothTray-go", fmt.Sprintf("Подключение к %s...", dev.Name), false)
		}

		go func() {
			isNowConnected, err := a.btManager.ToggleConnection(dev)
			if err != nil {
				a.ShowNotification("ToothTray-go", fmt.Sprintf("Ошибка: %v", err), true)
				return
			}

			// Wait up to 3 seconds for state confirmation
			for i := 0; i < 6; i++ {
				time.Sleep(500 * time.Millisecond)
				updatedList, _ := a.btManager.GetDevices()
				for _, ud := range updatedList {
					if ud.Address == dev.Address {
						isNowConnected = ud.Connected
						break
					}
				}
				if isNowConnected != wasConnected {
					break
				}
			}

			if isNowConnected {
				a.ShowNotification("ToothTray-go", fmt.Sprintf("Подключено к %s", dev.Name), false)
			} else {
				a.ShowNotification("ToothTray-go", fmt.Sprintf("Отключено от %s", dev.Name), false)
			}
		}()

	case cmd == ID_REFRESH:
		// Menu will refresh on next click, or show a subtle notification
		a.ShowNotification("ToothTray-go", "Список устройств обновлен", false)

	case cmd == ID_SETTINGS:
		procShellExecute.Call(
			0,
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("open"))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("ms-settings:bluetooth"))),
			0,
			0,
			1, // SW_SHOWNORMAL
		)

	case cmd == ID_NOTIFS:
		newState := !a.cfg.IsNotificationsEnabled()
		if err := a.cfg.SetNotificationsEnabled(newState); err != nil {
			a.ShowNotification("ToothTray-go", fmt.Sprintf("Ошибка сохранения: %v", err), true)
		} else {
			if newState {
				a.ShowNotification("ToothTray-go", "Уведомления включены", false)
			}
		}

	case cmd == ID_AUTOSTART:
		newState := !autostart.IsEnabled()
		if err := autostart.SetEnabled(newState); err != nil {
			a.ShowNotification("ToothTray-go", fmt.Sprintf("Ошибка автозагрузки: %v", err), true)
		} else {
			if newState {
				a.ShowNotification("ToothTray-go", "Автозагрузка включена", false)
			} else {
				a.ShowNotification("ToothTray-go", "Автозагрузка отключена", false)
			}
		}

	case cmd == ID_EXIT:
		procDestroyWindow.Call(uintptr(a.hWnd))
	}
}

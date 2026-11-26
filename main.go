package main

import (
	"log"
	"os"
	"runtime"
	"unsafe"

	"github.com/internetson/ToothTray-go/pkg/tray"

	"golang.org/x/sys/windows"
)

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

func init() {
	// Lock the main goroutine to its OS thread immediately.
	// This is critical for Win32 GUI / message loop applications in Go,
	// ensuring the window and its message queue are never orphaned across
	// Go scheduler migrations or PC sleep/resume cycles.
	runtime.LockOSThread()
}

func main() {
	runtime.LockOSThread()

	mutexName := windows.StringToUTF16Ptr("Local\\ToothTray-go_SingleInstance_Mutex")
	hMutex, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if hMutex == 0 {
		log.Printf("Failed to create mutex: %v\n", err)
	} else if err == windows.ERROR_ALREADY_EXISTS {
		// Another instance is already running
		os.Exit(0)
	}

	app := tray.NewApp()
	if err := app.Run(); err != nil {
		log.Printf("ToothTray-go error: %v\n", err)
		os.Exit(1)
	}
}


package main

import (
	"log"
	"os"
	"unsafe"

	"github.com/internetson/ToothTray-go/pkg/tray"

	"golang.org/x/sys/windows"
)

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

func main() {
	mutexName := windows.StringToUTF16Ptr("Local\\ToothTray_SingleInstance_Mutex")
	hMutex, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if hMutex == 0 {
		log.Printf("Failed to create mutex: %v\n", err)
	} else if err == windows.ERROR_ALREADY_EXISTS {
		// Another instance is already running
		os.Exit(0)
	}

	app := tray.NewApp()
	if err := app.Run(); err != nil {
		log.Printf("ToothTray error: %v\n", err)
		os.Exit(1)
	}
}


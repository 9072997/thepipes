//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func main() {
	var err error
	if len(os.Args) >= 3 && os.Args[1] == "--pack" {
		err = pack(os.Args[2])
	} else {
		err = run()
	}
	if err != nil {
		msg, _ := windows.UTF16PtrFromString(fmt.Sprintf("%v", err))
		title, _ := windows.UTF16PtrFromString("Installer Error")
		windows.MessageBox(0, msg, title, windows.MB_OK|windows.MB_ICONERROR)
		os.Exit(1)
	}
}

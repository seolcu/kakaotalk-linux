// kakaotalk-helper holds the native parts of kakaotalk-linux in one static binary.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kakaotalk-helper tray | send <open|quit> | hide | running | portal <base> | prepare-prefix <prefix> | extract-icon <exe> <png>")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "tray":
		err = runTray()
	case "send":
		err = sendCommand(os.Args[2])
	case "hide":
		err = runCtl("close")
	case "running":
		if !kakaoRunning() {
			os.Exit(1)
		}
	case "portal":
		runPortal(os.Args[2])
	case "prepare-prefix":
		err = preparePrefix(os.Args[2])
	case "extract-icon":
		err = extractIcon(os.Args[2], os.Args[3])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kakaotalk-helper:", err)
		os.Exit(1)
	}
}

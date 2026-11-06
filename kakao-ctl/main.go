// kakao-ctl: send commands to the running KakaoTalk from inside its Wine prefix.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	u   = syscall.NewLazyDLL("user32.dll")
	k   = syscall.NewLazyDLL("kernel32.dll")
	enum = u.NewProc("EnumWindows")
	cls = u.NewProc("GetClassNameW")
	tid = u.NewProc("GetWindowThreadProcessId")
	vis = u.NewProc("IsWindowVisible")
	rect = u.NewProc("GetWindowRect")
	tlen = u.NewProc("GetWindowTextLengthW")
	gtext = u.NewProc("GetWindowTextW")
	post = u.NewProc("PostMessageW")
	open_ = k.NewProc("OpenProcess")
	qname = k.NewProc("QueryFullProcessImageNameW")
	closeh = k.NewProc("CloseHandle")
)

type win struct {
	h             uintptr
	class, exe    string
	pid           uint32
	visible       bool
	l, t, r, b    int32
	titleLen      uintptr
}

func exeOf(pid uint32) string {
	h, _, _ := open_.Call(0x1000, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer closeh.Call(h)
	var b [512]uint16
	n := uint32(len(b))
	qname.Call(h, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&n)))
	s := syscall.UTF16ToString(b[:])
	return s[strings.LastIndex(s, "\\")+1:]
}

func windows() []win {
	var out []win
	cb := syscall.NewCallback(func(h, _ uintptr) uintptr {
		var c [256]uint16
		cls.Call(h, uintptr(unsafe.Pointer(&c[0])), 256)
		var pid uint32
		tid.Call(h, uintptr(unsafe.Pointer(&pid)))
		var r [4]int32
		rect.Call(h, uintptr(unsafe.Pointer(&r[0])))
		v, _, _ := vis.Call(h)
		n, _, _ := tlen.Call(h)
		out = append(out, win{h, syscall.UTF16ToString(c[:]), exeOf(pid), pid, v != 0, r[0], r[1], r[2], r[3], n})
		return 1
	})
	enum.Call(cb, 0)
	return out
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: kakao-ctl list | cmd <id> | close")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "list":
		for _, w := range windows() {
			if strings.EqualFold(w.exe, "KakaoTalk.exe") {
				fmt.Printf("hwnd=%#x class=%q vis=%v rect=%d,%d,%d,%d titlelen=%d\n", w.h, w.class, w.visible, w.l, w.t, w.r, w.b, w.titleLen)
			}
		}
	case "cmd":
		// Post a WM_COMMAND to the main window: class EVA_Window_Dblclk titled "카카오톡".
		id, err := strconv.ParseUint(os.Args[2], 0, 16)
		if err != nil {
			os.Exit(2)
		}
		sendMain(0x111, uintptr(id))
	case "close":
		// SC_CLOSE hides KakaoTalk to the tray. WM_CLOSE would quit it.
		sendMain(0x112, 0xF060)
	case "post-hidden":
		p := func(s string) uintptr { v, _ := strconv.ParseUint(s, 0, 64); return uintptr(v) }
		for _, w := range windows() {
			if strings.EqualFold(w.exe, "KakaoTalk.exe") && w.class == os.Args[2] && !w.visible && w.l == 0 && w.r == 0 {
				post.Call(w.h, p(os.Args[3]), p(os.Args[4]), p(os.Args[5]))
				fmt.Printf("posted hwnd=%#x\n", w.h)
				return
			}
		}
		fmt.Println("window_not_found")
		os.Exit(1)
	case "post":
		p := func(s string) uintptr { v, _ := strconv.ParseUint(s, 0, 64); return uintptr(v) }
		for _, w := range windows() {
			if strings.EqualFold(w.exe, "KakaoTalk.exe") && w.class == os.Args[2] {
				post.Call(w.h, p(os.Args[3]), p(os.Args[4]), p(os.Args[5]))
				fmt.Printf("posted hwnd=%#x\n", w.h)
				return
			}
		}
		fmt.Println("window_not_found")
		os.Exit(1)
	}
}

// sendMain posts a message to the main window: class EVA_Window_Dblclk titled "카카오톡".
func sendMain(msg, wparam uintptr) {
	for _, w := range windows() {
		if !strings.EqualFold(w.exe, "KakaoTalk.exe") || w.class != "EVA_Window_Dblclk" {
			continue
		}
		var t [64]uint16
		gtext.Call(w.h, uintptr(unsafe.Pointer(&t[0])), 64)
		if syscall.UTF16ToString(t[:]) == "카카오톡" {
			post.Call(w.h, msg, wparam, 0)
			fmt.Printf("posted hwnd=%#x msg=%#x wparam=%#x\n", w.h, msg, wparam)
			return
		}
	}
	fmt.Println("main_window_not_found")
	os.Exit(1)
}

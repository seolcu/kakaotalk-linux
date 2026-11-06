package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	trayBusName  = "io.github.seolcu.KakaoTalkLinux.Tray"
	trayObject   = "/io/github/seolcu/KakaoTalkLinux/Tray"
	itemPath     = dbus.ObjectPath("/StatusNotifierItem")
	menuPath     = dbus.ObjectPath("/MenuBar")
	itemIface    = "org.kde.StatusNotifierItem"
	menuIface    = "com.canonical.dbusmenu"
	watcherName  = "org.kde.StatusNotifierWatcher"
	watcherPath  = "/StatusNotifierWatcher"
	startupGrace = 180 * time.Second
)

// WM_COMMAND IDs of KakaoTalk's tray menu, read from a Wine +menu trace of 26.9.2. An update may
// change them. The tray cannot see which items KakaoTalk would grey out, so rarely useful ones
// that act even when not applicable (update, unhide toolbar) are left out.
type menuItem struct {
	id      int32
	label   string
	command uint16
}

var trayMenu = []menuItem{
	{1, "열기", 0x8035},
	{2, "카카오톡 정보", 0x8034},
	{3, "", 0},
	{4, "잠금모드 설정", 0x8073},
	{5, "로그아웃", 0x8033},
	{6, "", 0},
	{7, "종료", 0x8032},
}

const commandOpen, commandQuit = 0x8035, 0x8032

func sendCommand(name string) error {
	switch name {
	case "open":
		return runCtl("cmd", fmt.Sprintf("%#x", commandOpen))
	case "quit":
		return runCtl("cmd", fmt.Sprintf("%#x", commandQuit))
	}
	return fmt.Errorf("unknown command %q", name)
}

// runCtl runs kakao-ctl.exe inside the running prefix. The launcher sets these variables.
func runCtl(args ...string) error {
	cmd := exec.Command(os.Getenv("KAKAO_WINE"), append([]string{os.Getenv("KAKAO_CTL")}, args...)...)
	cmd.Env = append(os.Environ(), "WINEPREFIX="+os.Getenv("KAKAO_PREFIX"), "WINEDEBUG=-all")
	cmd.Dir = os.Getenv("HOME")
	return cmd.Run()
}

func kakaoRunning() bool {
	entries, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		first, _, _ := strings.Cut(string(data), "\x00")
		if !strings.HasSuffix(first, `\KakaoTalk.exe`) {
			continue
		}
		// Ignore a KakaoTalk from another install, such as one running under Steam.
		prefix := os.Getenv("KAKAO_PREFIX")
		if prefix == "" {
			return true
		}
		env, err := os.ReadFile(filepath.Join(filepath.Dir(path), "environ"))
		if err == nil && strings.Contains(string(env), "\x00KAKAO_PREFIX="+prefix+"\x00") {
			return true
		}
	}
	return false
}

type trayState struct {
	sync.Mutex
	seen  bool
	since time.Time
}

// Reset is called by a new launch while this tray is still running.
func (t *trayState) Reset() *dbus.Error {
	t.Lock()
	defer t.Unlock()
	t.seen, t.since = false, time.Now()
	return nil
}

type statusItem struct{}

func (statusItem) Activate(x, y int32) *dbus.Error {
	go sendCommand("open")
	return nil
}
func (statusItem) SecondaryActivate(x, y int32) *dbus.Error {
	go sendCommand("open")
	return nil
}
func (statusItem) ContextMenu(x, y int32) *dbus.Error { return nil }
func (statusItem) Scroll(delta int32, orientation string) *dbus.Error {
	return nil
}

type layoutNode struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

type dbusMenu struct{}

func itemProps(m menuItem) map[string]dbus.Variant {
	if m.label == "" {
		return map[string]dbus.Variant{"type": dbus.MakeVariant("separator")}
	}
	return map[string]dbus.Variant{"label": dbus.MakeVariant(m.label), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true)}
}

func (dbusMenu) GetLayout(parent, depth int32, names []string) (uint32, layoutNode, *dbus.Error) {
	root := layoutNode{ID: 0, Props: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")}}
	for _, m := range trayMenu {
		if parent == m.id {
			return 1, layoutNode{ID: m.id, Props: itemProps(m), Children: []dbus.Variant{}}, nil
		}
		root.Children = append(root.Children, dbus.MakeVariant(layoutNode{ID: m.id, Props: itemProps(m), Children: []dbus.Variant{}}))
	}
	return 1, root, nil
}

type groupProps struct {
	ID    int32
	Props map[string]dbus.Variant
}

func (dbusMenu) GetGroupProperties(ids []int32, names []string) ([]groupProps, *dbus.Error) {
	var out []groupProps
	for _, m := range trayMenu {
		if len(ids) == 0 {
			out = append(out, groupProps{m.id, itemProps(m)})
			continue
		}
		for _, id := range ids {
			if id == m.id {
				out = append(out, groupProps{m.id, itemProps(m)})
			}
		}
	}
	return out, nil
}

func (dbusMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	for _, m := range trayMenu {
		if m.id == id {
			if v, ok := itemProps(m)[name]; ok {
				return v, nil
			}
		}
	}
	return dbus.MakeVariant(""), nil
}

func activate(id int32, event string) {
	if event != "clicked" {
		return
	}
	for _, m := range trayMenu {
		if m.id == id && m.command != 0 {
			go runCtl("cmd", fmt.Sprintf("%#x", m.command))
		}
	}
}

func (dbusMenu) Event(id int32, event string, data dbus.Variant, timestamp uint32) *dbus.Error {
	activate(id, event)
	return nil
}

type menuEvent struct {
	ID        int32
	Event     string
	Data      dbus.Variant
	Timestamp uint32
}

func (dbusMenu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, e := range events {
		activate(e.ID, e.Event)
	}
	return []int32{}, nil
}
func (dbusMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }
func (dbusMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}

type pixmap struct {
	W, H int32
	Data []byte
}
type toolTip struct {
	Icon   string
	Pixmap []pixmap
	Title  string
	Text   string
}

func constProps(values map[string]any) map[string]*prop.Prop {
	out := map[string]*prop.Prop{}
	for k, v := range values {
		out[k] = &prop.Prop{Value: v, Writable: false, Emit: prop.EmitFalse}
	}
	return out
}

func runTray() error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	reply, err := conn.RequestName(trayBusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return conn.Object(trayBusName, trayObject).Call(trayBusName+".Reset", 0).Err
	}
	state := &trayState{since: time.Now()}
	conn.Export(state, trayObject, trayBusName)
	conn.Export(statusItem{}, itemPath, itemIface)
	conn.Export(dbusMenu{}, menuPath, menuIface)
	if _, err := prop.Export(conn, itemPath, prop.Map{itemIface: constProps(map[string]any{
		"Category": "Communications", "Id": "kakaotalk", "Title": "카카오톡", "Status": "Active",
		"WindowId": int32(0), "IconName": "kakaotalk", "IconThemePath": "", "OverlayIconName": "",
		"AttentionIconName": "", "AttentionMovieName": "", "ItemIsMenu": false, "Menu": menuPath,
		"IconPixmap": []pixmap{}, "OverlayIconPixmap": []pixmap{}, "AttentionIconPixmap": []pixmap{},
		"ToolTip": toolTip{"kakaotalk", []pixmap{}, "카카오톡", ""},
	})}); err != nil {
		return err
	}
	if _, err := prop.Export(conn, menuPath, prop.Map{menuIface: constProps(map[string]any{
		"Version": uint32(3), "TextDirection": "ltr", "Status": "normal", "IconThemePath": []string{},
	})}); err != nil {
		return err
	}
	itemName := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	if _, err := conn.RequestName(itemName, dbus.NameFlagDoNotQueue); err != nil {
		return err
	}

	registered := false
	bus := conn.BusObject()
	for {
		var hasWatcher bool
		bus.Call("org.freedesktop.DBus.NameHasOwner", 0, watcherName).Store(&hasWatcher)
		if hasWatcher && !registered {
			registered = conn.Object(watcherName, watcherPath).Call(watcherName+".RegisterStatusNotifierItem", 0, itemName).Err == nil
		} else if !hasWatcher {
			registered = false
		}
		running := kakaoRunning()
		state.Lock()
		state.seen = state.seen || running
		stop := !running && (state.seen || time.Since(state.since) > startupGrace)
		state.Unlock()
		if stop {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
}

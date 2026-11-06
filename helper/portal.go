package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

// The comdlg32 proxy writes <base>.req (UTF-8 lines: mode, title, multiplicity, suggested name,
// then filter name and glob pairs). This creates <base>.started and atomically writes
// <base>.resp as "OK\n<path>...", "CANCEL" or "ERR <reason>".
type filterRule struct {
	Kind uint32
	Glob string
}
type fileFilter struct {
	Name  string
	Rules []filterRule
}

func respond(base, text string) {
	if os.WriteFile(base+".resp.tmp", []byte(text), 0o600) == nil {
		os.Rename(base+".resp.tmp", base+".resp")
	}
}

func choose(req []string) (string, error) {
	for len(req) < 4 {
		req = append(req, "")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	token := fmt.Sprintf("kakao%d", os.Getpid())
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	path := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(path), dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response")); err != nil {
		return "", err
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)

	options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(token)}
	var filters []fileFilter
	for i := 4; i+1 < len(req); i += 2 {
		f := fileFilter{Name: req[i]}
		for _, g := range strings.Split(req[i+1], ";") {
			if g = strings.TrimSpace(g); g != "" {
				f.Rules = append(f.Rules, filterRule{0, g})
			}
		}
		if f.Name != "" && len(f.Rules) > 0 {
			filters = append(filters, f)
		}
	}
	if len(filters) > 0 {
		options["filters"] = dbus.MakeVariant(filters)
	}
	method, title := "OpenFile", req[1]
	if req[0] == "save" {
		method = "SaveFile"
		if title == "" {
			title = "저장"
		}
		if name := req[3]; name != "" {
			if i := strings.LastIndexAny(name, "\\/"); i >= 0 {
				name = name[i+1:]
			}
			options["current_name"] = dbus.MakeVariant(name)
		}
	} else {
		if title == "" {
			title = "열기"
		}
		options["multiple"] = dbus.MakeVariant(req[2] == "multiple")
	}
	portal := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var handle dbus.ObjectPath
	if err := portal.Call("org.freedesktop.portal.FileChooser."+method, 0, "", title, options).Store(&handle); err != nil {
		return "", err
	}
	for s := range signals {
		if s.Name != "org.freedesktop.portal.Request.Response" || s.Path != handle || len(s.Body) < 2 {
			continue
		}
		if status, _ := s.Body[0].(uint32); status != 0 {
			return "CANCEL", nil
		}
		results, _ := s.Body[1].(map[string]dbus.Variant)
		uris, _ := results["uris"].Value().([]string)
		out := []string{"OK"}
		for _, u := range uris {
			if parsed, err := url.Parse(u); err == nil && parsed.Scheme == "file" {
				out = append(out, parsed.Path)
			}
		}
		return strings.Join(out, "\n") + "\n", nil
	}
	return "", fmt.Errorf("portal connection closed")
}

func runPortal(base string) {
	os.WriteFile(base+".started", nil, 0o600)
	data, err := os.ReadFile(base + ".req")
	if err != nil {
		respond(base, "ERR "+err.Error())
		return
	}
	text, err := choose(strings.Split(strings.TrimRight(string(data), "\n"), "\n"))
	if err != nil {
		respond(base, "ERR "+err.Error())
		return
	}
	respond(base, text)
}

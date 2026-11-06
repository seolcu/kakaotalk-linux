package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	imagepng "image/png"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const audioDeviceLimit = 64

var audioSection = regexp.MustCompile(`(?m)^\[Software\\\\Wine\\\\Drivers\\\\winepulse\.drv\\\\devices[^\]]*\][^\[]*`)

// preparePrefix runs while Wine is stopped. Wine keeps every audio device it has ever seen and
// scans the list once per device. On a shared network with AirPlay discovery, this grew to about
// 1,950 entries and stalled login for minutes. It also hides Wine's tray, whose popup menu does
// not close under Wayland.
func preparePrefix(prefix string) error {
	path := filepath.Join(prefix, "user.reg")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	out := data
	if n := len(audioSection.FindAll(data, -1)); n > audioDeviceLimit {
		out = audioSection.ReplaceAll(out, nil)
		fmt.Printf("audio_devices_pruned=%d\n", n)
	}
	const policy = `[Software\\Microsoft\\Windows\\CurrentVersion\\Policies\\Explorer]`
	if !bytes.Contains(out, []byte("NoTrayItemsDisplay")) {
		if i := bytes.Index(out, []byte(policy)); i >= 0 {
			eol := i + bytes.IndexByte(out[i:], '\n') + 1
			out = append(out[:eol:eol], append([]byte("\"NoTrayItemsDisplay\"=dword:00000001\n"), out[eol:]...)...)
		} else {
			out = append(bytes.TrimRight(out, "\n"), []byte(fmt.Sprintf("\n\n%s %d\n\"NoTrayItemsDisplay\"=dword:00000001\n", policy, time.Now().Unix()))...)
		}
	}
	if !bytes.Equal(out, data) {
		if err := os.WriteFile(path+".tmp", out, 0o644); err != nil {
			return err
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			return err
		}
	}
	return linkEmojiFont(prefix)
}

const emojiLink = `NOTOEMOJI-REGULAR.TTF,Noto Emoji\0`

var (
	fontLinkSection = regexp.MustCompile(`(?s)\[Software\\\\Microsoft\\\\Windows NT\\\\CurrentVersion\\\\FontLink\\\\SystemLink\][^\n]*\n.*?\n\n`)
	fontLinkValue   = regexp.MustCompile(`(?m)^("[^"\n]+"=str\(7\):")((?:[^"\\\n]|\\.)*)"$`)
)

// linkEmojiFont makes Noto Emoji a fallback for the fonts KakaoTalk uses. Wine's GDI cannot draw
// colour fonts, so emoji are monochrome. Above the BMP, this needs the patched win32u.so.
func linkEmojiFont(prefix string) error {
	path := filepath.Join(prefix, "system.reg")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	section := fontLinkSection.Find(data)
	if section == nil || bytes.Contains(section, []byte("Noto Emoji")) {
		return nil
	}
	updated := fontLinkValue.ReplaceAll(section, []byte(`${1}${2}`+emojiLink+`"`))
	updated = bytes.TrimRight(updated, "\n")
	for _, name := range []string{"Malgun Gothic", "맑은 고딕", "Noto Sans CJK KR", "Tahoma", "Segoe UI", "System"} {
		if !bytes.Contains(updated, []byte(`"`+name+`"=str(7)`)) {
			updated = append(updated, []byte("\n\""+name+`"=str(7):"`+emojiLink+`"`)...)
		}
	}
	updated = append(updated, "\n\n"...)
	out := bytes.Replace(data, section, updated, 1)
	if err := os.WriteFile(path+".tmp", out, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// extractIcon saves the largest icon in a PE file's resources as a PNG.
func extractIcon(exe, png string) (err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("cannot read icon from %s", exe)
		}
	}()
	d, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	u16 := func(o int) int { return int(binary.LittleEndian.Uint16(d[o:])) }
	u32 := func(o int) int { return int(binary.LittleEndian.Uint32(d[o:])) }
	pe := u32(0x3c)
	sections, optional := u16(pe+6), u16(pe+20)
	rva := u32(pe + 24 + 112 + 8*2)
	offset := func(r int) int {
		for i := 0; i < sections; i++ {
			s := pe + 24 + optional + 40*i
			vs, va, rs, rp := u32(s+8), u32(s+12), u32(s+16), u32(s+20)
			if vs < rs {
				vs = rs
			}
			if va <= r && r < va+vs {
				return r - va + rp
			}
		}
		return -1
	}
	base := offset(rva)
	if base < 0 {
		return fmt.Errorf("no resources in %s", exe)
	}
	var best []byte
	var walk func(dir, depth int, wanted bool)
	walk = func(dir, depth int, wanted bool) {
		n := u16(base+dir+12) + u16(base+dir+14)
		for i := 0; i < n; i++ {
			e := base + dir + 16 + 8*i
			name, target := u32(e), u32(e+4)
			if target&0x80000000 != 0 {
				if depth > 0 || name == 3 { // RT_ICON at the top level
					walk(target&0x7fffffff, depth+1, true)
				}
				continue
			}
			if !wanted {
				continue
			}
			o := offset(u32(base + target))
			size := u32(base + target + 4)
			if o >= 0 && o+size <= len(d) && size > len(best) {
				best = d[o : o+size]
			}
		}
	}
	walk(0, 0, false)
	if best == nil {
		return fmt.Errorf("no icon in %s", exe)
	}
	if bytes.HasPrefix(best, []byte("\x89PNG")) {
		return os.WriteFile(png, best, 0o644)
	}
	// A 32-bit DIB: BGRA rows from the bottom up, with height doubled for the AND mask.
	size := int(binary.LittleEndian.Uint32(best))
	w := int(binary.LittleEndian.Uint32(best[4:]))
	h := int(binary.LittleEndian.Uint32(best[8:])) / 2
	if binary.LittleEndian.Uint16(best[14:]) != 32 || size+w*h*4 > len(best) {
		return fmt.Errorf("unsupported icon format in %s", exe)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := best[size+(h-1-y)*w*4+x*4:]
			img.SetNRGBA(x, y, color.NRGBA{p[2], p[1], p[0], p[3]})
		}
	}
	f, err := os.Create(png)
	if err != nil {
		return err
	}
	defer f.Close()
	return imagepng.Encode(f, img)
}

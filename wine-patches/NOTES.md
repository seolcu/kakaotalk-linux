# Notes on the Wine patch

`kakaotalk.patch` works around two Wine bugs. Neither is fixed on Wine master as of 2026-11-06.
This project does not plan to submit the patch upstream. If Wine fixes them, drop the patch and
the module build.

## 1. winex11: preedit hidden after an XIM commit

Existing bug: none found.

IBus (ibus-hangul) sends the next syllable's preedit before committing the previous one. For
"안녕", typing ㄴ after 안 yields `preedit_draw("ㄴ")`, then the commit of "안".
`xim_set_result_string` calls `post_ime_update( hwnd, 0, NULL, output )`, and the NULL composition
clears the ㄴ that was just drawn. It reappears only with the next preedit draw, when the vowel is
typed. The committed text is correct.

Fix: pass the current preedit with the result,
`post_ime_update( hwnd, ime_comp_cursor_pos, ime_comp_buf, output )`. When there is no preedit,
`ime_comp_buf` is NULL and behaviour is unchanged.

Reproduce: Fedora 44 GNOME, ibus-hangul, any IMM-aware edit control. Type `dkssud` in Hangul
mode; the second ㄴ is invisible until `u` is typed.

## 2. win32u: characters above the BMP draw as two boxes

Existing bug: https://bugs.winehq.org/show_bug.cgi?id=53929 ("ExtTextOut() does not support
surrogate pairs").

Four places stop at 16 bits:

- `select_charmap` takes the last Microsoft cmap, which for most fonts is the (3,1) BMP-only
  table, even when a (3,10) UCS-4 table exists.
- `NtGdiGetGlyphOutline` passes `ch & 0xffff` to the driver.
- The dib driver's glyph cache has 0x10000 / 0x100 pages.
- `render_string` and `font_GetTextExtentExPoint` look up each UTF-16 unit on its own.

The patch combines a surrogate pair into one code point for the lookup, gives the low surrogate
zero advance, and keeps a UCS-4 cmap when present. With a font that has the glyph, such as Noto
Emoji linked through `FontLink\SystemLink`, 😀 draws once instead of as two boxes. GDI+ and
`GetCharABCWidths` are not changed.

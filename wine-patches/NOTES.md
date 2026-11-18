# Notes on the Wine patch

`kakaotalk.patch` works around three Wine bugs (none fixed on Wine master as of 2026-11-12) and turns off one GE-Proton feature (section 4).
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

## 3. riched20: assertion abort when focus returns during an IME composition

Existing bug: none found.

KakaoTalk's message box is a RichEdit control. While a Hangul syllable is still being composed,
clicking the reply button takes focus away and back. `WM_IME_ENDCOMPOSITION` deletes the
composition text with `ME_DeleteSelection`, which marks the paragraph for rewrap, but never
rewraps it. The next `WM_SETFOCUS` calls `create_caret` → `cursor_coords`, which asserts
`~para->nFlags & MEPF_REWRAP` (caret.c:232) and the process aborts. The log shows
`err:msvcrt:_wassert (L"~para->nFlags & MEPF_REWRAP",L"../src-wine/dlls/riched20/caret.c",232)`.

Fix: call `ME_UpdateRepaint` after the deletion in `WM_IME_ENDCOMPOSITION`, and wrap marked
paragraphs at the start of `WM_SETFOCUS` before positioning the caret.

Reproduce: in a chat, type a Hangul syllable and, before finishing it, click 답장 on a message.

## 4. win32u: GE-Proton's SNI tray adds a second tray icon

Not a Wine bug: GE-Proton 11 carries `em-fixups/0001-winewayland-add-SNI-tray-icons-…`, which
makes `Shell_NotifyIconW` register the app's icon as a StatusNotifierItem from inside the process.
That path skips explorer's tray, so the prefix's `NoTrayItemsDisplay=1` no longer hides it, and
next to `kakaotalk-helper`'s icon a second "KakaoTalk" icon appears. GE's `0056` already falls back
to explorer's XEmbed tray on Mutter, which is why GNOME showed one icon and KDE Plasma two
(found 2026-11-18, Fedora 45 KDE).

Fix: make that fallback unconditional in `sni_notify_icon` (`if (TRUE || …Mutter…)`), so every
desktop takes the explorer path and the policy applies again. If a GE update drops the SNI
patches, drop this hunk.

Reproduce: KDE Plasma, start KakaoTalk; `busctl --user get-property org.kde.StatusNotifierWatcher
/StatusNotifierWatcher org.kde.StatusNotifierWatcher RegisteredStatusNotifierItems` lists both the
helper's item and one owned by `KakaoTalk.exe` with an Id like `wine-0x1007e-222`.

/*
 * comdlg32 proxy for KakaoTalk under Proton.
 *
 * GetOpenFileNameW and GetSaveFileNameW go to the desktop's own file chooser through
 * xdg-desktop-portal, using "kakaotalk-helper portal". Everything else is forwarded to Wine's
 * comdlg32, copied beside this DLL as comdlg32_wine.dll. If the helper is unavailable or the
 * call uses a hook or template, the Wine dialog is used instead.
 */
#include <windows.h>
#include <commdlg.h>
#include <stdio.h>

#define MAX_REQ 65536
static HINSTANCE self;

BOOL WINAPI DllMain(HINSTANCE h, DWORD why, LPVOID r) { if (why == DLL_PROCESS_ATTACH) self = h; return TRUE; }

typedef BOOL (WINAPI *ofn_fn)(LPOPENFILENAMEW);

static FARPROC real_proc(const char *name)
{
    static HMODULE mod;
    if (!mod) {
        WCHAR path[MAX_PATH], *slash;
        GetModuleFileNameW(self, path, MAX_PATH);
        if ((slash = wcsrchr(path, L'\\'))) lstrcpyW(slash + 1, L"comdlg32_wine.dll");
        mod = LoadLibraryW(path);
    }
    return mod ? GetProcAddress(mod, name) : NULL;
}

static int put_utf8(char *out, int cap, const WCHAR *in)
{
    return WideCharToMultiByte(CP_UTF8, 0, in, -1, out, cap, NULL, NULL);
}

/* Append one newline-free UTF-8 line. */
static int add_line(char *req, int pos, const WCHAR *text)
{
    char *p;
    int n = put_utf8(req + pos, MAX_REQ - pos - 2, text ? text : L"");
    if (n <= 0) n = 1;
    for (p = req + pos; *p; p++) if (*p == '\n' || *p == '\r') *p = ' ';
    pos += n - 1;
    req[pos++] = '\n';
    req[pos] = 0;
    return pos;
}

static BOOL write_file(const WCHAR *path, const char *data)
{
    DWORD written;
    HANDLE f = CreateFileW(path, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE) return FALSE;
    WriteFile(f, data, strlen(data), &written, NULL);
    CloseHandle(f);
    return TRUE;
}

static char *read_file(const WCHAR *path)
{
    DWORD size, got;
    char *buf;
    HANDLE f = CreateFileW(path, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (f == INVALID_HANDLE_VALUE) return NULL;
    size = GetFileSize(f, NULL);
    buf = HeapAlloc(GetProcessHeap(), 0, size + 1);
    ReadFile(f, buf, size, &got, NULL);
    buf[got] = 0;
    CloseHandle(f);
    return buf;
}

/* Returns the response text, or NULL if the portal path failed and Wine's dialog should run. */
static char *ask_portal(const char *req, HWND owner)
{
    WCHAR helper[MAX_PATH], cmd[MAX_PATH * 2], wreq[64], wresp[64], wstart[64];
    STARTUPINFOW si = {sizeof(si)};
    PROCESS_INFORMATION pi;
    DWORD id = GetCurrentProcessId() ^ GetTickCount(), started = GetTickCount();
    BOOL enabled = FALSE, seen = FALSE;
    char *resp = NULL;
    WCHAR *c;

    if (!GetEnvironmentVariableW(L"KAKAO_PORTAL_HELPER", helper, MAX_PATH)) return NULL;
    swprintf(wreq, 64, L"Z:\\tmp\\kakao-portal-%08lx.req", id);
    swprintf(wresp, 64, L"Z:\\tmp\\kakao-portal-%08lx.resp", id);
    swprintf(wstart, 64, L"Z:\\tmp\\kakao-portal-%08lx.started", id);
    if (!write_file(wreq, req)) return NULL;
    swprintf(cmd, MAX_PATH * 2, L"\"Z:%ls\" portal /tmp/kakao-portal-%08lx", helper, id);
    for (c = cmd; *c; c++) if (*c == L'/' && c < wcschr(cmd + 1, L'"')) *c = L'\\';
    if (!CreateProcessW(NULL, cmd, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi)) goto done;
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);
    if (owner) enabled = !EnableWindow(owner, FALSE);
    for (;;) {
        MSG msg;
        if (GetFileAttributesW(wresp) != INVALID_FILE_ATTRIBUTES) { resp = read_file(wresp); break; }
        seen = seen || GetFileAttributesW(wstart) != INVALID_FILE_ATTRIBUTES;
        if (!seen && GetTickCount() - started > 5000) break;
        MsgWaitForMultipleObjects(0, NULL, FALSE, 50, QS_ALLINPUT);
        while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE)) {
            if (msg.message == WM_QUIT) { PostQuitMessage(msg.wParam); goto restore; }
            TranslateMessage(&msg);
            DispatchMessageW(&msg);
        }
    }
restore:
    if (owner && enabled) { EnableWindow(owner, TRUE); SetForegroundWindow(owner); }
done:
    DeleteFileW(wreq);
    DeleteFileW(wresp);
    DeleteFileW(wstart);
    return resp;
}

/* Convert one UTF-8 Unix path to Z:\... form. */
static int to_win(const char *unix_path, WCHAR *out, int cap)
{
    int n;
    WCHAR *p;
    out[0] = L'Z'; out[1] = L':';
    n = MultiByteToWideChar(CP_UTF8, 0, unix_path, -1, out + 2, cap - 2);
    if (n <= 0) return 0;
    for (p = out; *p; p++) if (*p == L'/') *p = L'\\';
    return n + 1;
}

static BOOL use_portal(LPOPENFILENAMEW ofn, BOOL save, BOOL *handled)
{
    static char req[MAX_REQ];
    const WCHAR *f;
    char *resp, *line, *next;
    WCHAR first[MAX_PATH * 2], *slash;
    int pos = 0, count = 0, used;
    BOOL ok = FALSE;

    *handled = FALSE;
    if (ofn->Flags & (OFN_ENABLEHOOK | OFN_ENABLETEMPLATE | OFN_ENABLETEMPLATEHANDLE)) return FALSE;
    pos = add_line(req, pos, save ? L"save" : L"open");
    pos = add_line(req, pos, ofn->lpstrTitle);
    pos = add_line(req, pos, (ofn->Flags & OFN_ALLOWMULTISELECT) ? L"multiple" : L"single");
    pos = add_line(req, pos, save && ofn->lpstrFile ? ofn->lpstrFile : L"");
    for (f = ofn->lpstrFilter; f && *f; ) {
        const WCHAR *glob = f + lstrlenW(f) + 1;
        pos = add_line(req, pos, f);
        pos = add_line(req, pos, glob);
        f = glob + lstrlenW(glob) + 1;
    }
    if (!(resp = ask_portal(req, ofn->hwndOwner))) return FALSE;
    *handled = TRUE;
    if (strncmp(resp, "OK\n", 3)) {
        if (strncmp(resp, "CANCEL", 6)) *handled = FALSE;
        goto out;
    }
    used = 0;
    for (line = resp + 3; *line; line = next) {
        WCHAR name[MAX_PATH * 2], *base;
        int len;
        if ((next = strchr(line, '\n'))) *next++ = 0; else next = line + strlen(line);
        if (!*line || !to_win(line, name, MAX_PATH * 2)) continue;
        if (!count++) lstrcpyW(first, name);
        base = wcsrchr(name, L'\\') + 1;
        if (count == 2) { /* Multiple files: directory, then each name, double NUL ended. */
            slash = wcsrchr(first, L'\\');
            used = (slash - first) + 1;
            if (used + lstrlenW(slash + 1) + 2 > (int)ofn->nMaxFile) goto small;
            memcpy(ofn->lpstrFile, first, (slash - first) * sizeof(WCHAR));
            ofn->lpstrFile[slash - first] = 0;
            lstrcpyW(ofn->lpstrFile + used, slash + 1);
            used += lstrlenW(slash + 1) + 1;
            ofn->nFileOffset = (slash - first) + 1;
            ofn->nFileExtension = 0;
        }
        if (count >= 2) {
            len = lstrlenW(base) + 1;
            if (used + len + 1 > (int)ofn->nMaxFile) goto small;
            lstrcpyW(ofn->lpstrFile + used, base);
            used += len;
        }
    }
    if (!count) goto out;
    if (count == 1) {
        WCHAR *dot;
        int n = lstrlenW(first);
        slash = wcsrchr(first, L'\\');
        if (save && ofn->lpstrDefExt && !wcschr(slash, L'.') && n + lstrlenW(ofn->lpstrDefExt) + 1 < MAX_PATH * 2) {
            lstrcatW(first, L".");
            lstrcatW(first, ofn->lpstrDefExt);
            n = lstrlenW(first);
        }
        if (n + 1 > (int)ofn->nMaxFile) goto small;
        lstrcpyW(ofn->lpstrFile, first);
        ofn->lpstrFile[n + 1] = 0;
        ofn->nFileOffset = (slash - first) + 1;
        dot = wcsrchr(slash, L'.');
        ofn->nFileExtension = dot ? (dot - first) + 1 : 0;
        if (ofn->lpstrFileTitle && ofn->nMaxFileTitle)
            lstrcpynW(ofn->lpstrFileTitle, slash + 1, ofn->nMaxFileTitle);
    } else {
        ofn->lpstrFile[used] = 0;
    }
    ok = TRUE;
    goto out;
small:
    /* Same contract as Windows: too small a buffer fails. The caller sees no selection. */
    ok = FALSE;
out:
    HeapFree(GetProcessHeap(), 0, resp);
    return ok;
}

BOOL WINAPI GetOpenFileNameW(LPOPENFILENAMEW ofn)
{
    BOOL handled, ok = use_portal(ofn, FALSE, &handled);
    ofn_fn real;
    if (handled) return ok;
    real = (ofn_fn)real_proc("GetOpenFileNameW");
    return real ? real(ofn) : FALSE;
}

BOOL WINAPI GetSaveFileNameW(LPOPENFILENAMEW ofn)
{
    BOOL handled, ok = use_portal(ofn, TRUE, &handled);
    ofn_fn real;
    if (handled) return ok;
    real = (ofn_fn)real_proc("GetSaveFileNameW");
    return real ? real(ofn) : FALSE;
}

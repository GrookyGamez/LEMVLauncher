//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW     = user32.NewProc("RegisterClassExW")
	pCreateWindowExW      = user32.NewProc("CreateWindowExW")
	pDefWindowProcW       = user32.NewProc("DefWindowProcW")
	pShowWindow           = user32.NewProc("ShowWindow")
	pUpdateWindow         = user32.NewProc("UpdateWindow")
	pGetMessageW          = user32.NewProc("GetMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessageW     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pPostMessageW         = user32.NewProc("PostMessageW")
	pSendMessageW         = user32.NewProc("SendMessageW")
	pLoadCursorW          = user32.NewProc("LoadCursorW")
	pLoadIconW            = user32.NewProc("LoadIconW")
	pSetCursor            = user32.NewProc("SetCursor")
	pBeginPaint           = user32.NewProc("BeginPaint")
	pEndPaint             = user32.NewProc("EndPaint")
	pInvalidateRect       = user32.NewProc("InvalidateRect")
	pGetClientRect        = user32.NewProc("GetClientRect")
	pFillRect             = user32.NewProc("FillRect")
	pDrawTextW            = user32.NewProc("DrawTextW")
	pSetWindowTextW       = user32.NewProc("SetWindowTextW")
	pGetWindowTextW       = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	pMessageBoxW          = user32.NewProc("MessageBoxW")
	pSetFocus             = user32.NewProc("SetFocus")
	pAdjustWindowRectEx   = user32.NewProc("AdjustWindowRectEx")
	pGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	pIsZoomed             = user32.NewProc("IsZoomed")
	pSetWindowPos         = user32.NewProc("SetWindowPos")
	pScreenToClient       = user32.NewProc("ScreenToClient")
	pMoveWindow           = user32.NewProc("MoveWindow")
	pSetWindowText        = user32.NewProc("SetWindowTextW")
	pSetTimer             = user32.NewProc("SetTimer")
	pOpenClipboard        = user32.NewProc("OpenClipboard")
	pEmptyClipboard       = user32.NewProc("EmptyClipboard")
	pSetClipboardData     = user32.NewProc("SetClipboardData")
	pCloseClipboard       = user32.NewProc("CloseClipboard")
	pKillTimer            = user32.NewProc("KillTimer")
	pSetViewportOrgEx     = gdi32.NewProc("SetViewportOrgEx")
	pSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	pGetDpiForSystem      = user32.NewProc("GetDpiForSystem")
	pGetDC                = user32.NewProc("GetDC")
	pReleaseDC            = user32.NewProc("ReleaseDC")
	pSetWindowLongPtrW    = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW      = user32.NewProc("CallWindowProcW")
	pTrackMouseEvent      = user32.NewProc("TrackMouseEvent")
	pGetParent            = user32.NewProc("GetParent")

	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pRoundRect              = gdi32.NewProc("RoundRect")
	pMoveToEx               = gdi32.NewProc("MoveToEx")
	pLineTo                 = gdi32.NewProc("LineTo")
	pArc                    = gdi32.NewProc("Arc")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pSetBkColor             = gdi32.NewProc("SetBkColor")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	pGetStockObject         = gdi32.NewProc("GetStockObject")
	pSetDCBrushColor        = gdi32.NewProc("SetDCBrushColor")
	pSaveDC                 = gdi32.NewProc("SaveDC")
	pRestoreDC              = gdi32.NewProc("RestoreDC")
	pIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
	pGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
)

const (
	wsOverlapped   = 0x00000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsVisible      = 0x10000000
	wsChild        = 0x40000000
	wsTabStop      = 0x00010000
	wsClipChildren = 0x02000000
	esAutoHScroll  = 0x0080

	csVRedraw = 0x0001
	csHRedraw = 0x0002
	csDblClks = 0x0008

	wmDestroy       = 0x0002
	wmActivate      = 0x0006
	wmSetFocus      = 0x0007
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmSize          = 0x0005
	wmGetMinMaxInfo = 0x0024
	wmTimer         = 0x0113
	wsThickFrame    = 0x00040000
	wsMaximizeBox   = 0x00010000
	wmNcCalcSize    = 0x0083
	wmNcHitTest     = 0x0084
	wmNcActivate    = 0x0086
	wmSysCommand    = 0x0112

	htCaption     = 2
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17

	scMinimize = 0xF020
	scMaximize = 0xF030
	scRestore  = 0xF120

	smCxSizeFrame    = 32
	smCySizeFrame    = 33
	smCxPaddedBorder = 92

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020

	swHide          = 0
	wmEraseBkgnd    = 0x0014
	wmSetCursor     = 0x0020
	wmSetFont       = 0x0030
	wmKeyDown       = 0x0100
	wmChar          = 0x0102
	wmCommand       = 0x0111
	wmCtlColorEdit  = 0x0133
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmMouseWheel    = 0x020A
	wmMouseLeave    = 0x02A3
	wmApp           = 0x8000

	wmAppRefresh      = wmApp + 1
	wmAppEnter        = wmApp + 2
	wmAppError        = wmApp + 3
	wmAppJarsChanged  = wmApp + 4
	wmAppEntriesReady = wmApp + 5

	enChange       = 0x0300
	wmSetText      = 0x000C
	emSetCueBanner = 0x1501
	emLimitText    = 0x00C5
	emSetSel       = 0x00B1

	idcArrow = 32512
	idcHand  = 32649

	swShowNormal = 1
	swShow       = 5

	dtLeft         = 0x0000
	dtCenter       = 0x0001
	dtRight        = 0x0002
	dtVCenter      = 0x0004
	dtWordBreak    = 0x0010
	dtSingleLine   = 0x0020
	dtNoPrefix     = 0x0800
	dtPathEllipsis = 0x4000
	dtEndEllipsis  = 0x8000

	bkTransparent = 1
	srcCopy       = 0x00CC0020
	nullPen       = 8
	dcBrush       = 18
	logPixelsX    = 88

	fwNormal   = 400
	fwSemibold = 600
	fwBold     = 700

	defaultCharset   = 1
	clearTypeQuality = 5

	mbOK              = 0x00000000
	mbIconError       = 0x00000010
	mbIconInformation = 0x00000040

	vkReturn = 0x0D
	vkEscape = 0x1B
	vkUp     = 0x26
	vkDown   = 0x28
	vkPrior  = 0x21
	vkNext   = 0x22

	htClient = 1

	smCxScreen = 0
	smCyScreen = 1

	tmeLeave = 0x00000002
)

type POINT struct{ X, Y int32 }

type MINMAXINFO struct {
	Reserved     POINT
	MaxSize      POINT
	MaxPosition  POINT
	MinTrackSize POINT
	MaxTrackSize POINT
}

type RECT struct{ Left, Top, Right, Bottom int32 }

type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   uintptr
	DwHoverTime uint32
}

type SIZE struct{ Cx, Cy int32 }

// wstr converts a Go string to a NUL-terminated UTF-16 pointer. Embedded NULs
// are stripped so conversion can't fail.
func wstr(s string) *uint16 {
	if i := indexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func wslice(s string) []uint16 {
	if i := indexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	u, _ := syscall.UTF16FromString(s)
	return u
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func loword(v uintptr) int { return int(uint16(v)) }
func hiword(v uintptr) int { return int(uint16(v >> 16)) }
func xOf(lp uintptr) int   { return int(int16(uint16(lp))) }
func yOf(lp uintptr) int   { return int(int16(uint16(lp >> 16))) }

// rgb builds a COLORREF (0x00BBGGRR).
func rgb(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }

func colR(c uint32) uint32 { return c & 0xff }
func colG(c uint32) uint32 { return (c >> 8) & 0xff }
func colB(c uint32) uint32 { return (c >> 16) & 0xff }

func clamp8(v int) uint32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint32(v)
}

func shade(c uint32, d int) uint32 {
	return rgb(clamp8(int(colR(c))+d), clamp8(int(colG(c))+d), clamp8(int(colB(c))+d))
}

func scaleCol(c uint32, f float64) uint32 {
	return rgb(clamp8(int(float64(colR(c))*f)), clamp8(int(float64(colG(c))*f)), clamp8(int(float64(colB(c))*f)))
}

func getModuleHandle() uintptr {
	h, _, _ := pGetModuleHandleW.Call(0)
	return h
}

func loadCursor(id uintptr) uintptr {
	h, _, _ := pLoadCursorW.Call(0, id)
	return h
}

func loadIcon(hInst, id uintptr) uintptr {
	h, _, _ := pLoadIconW.Call(hInst, id)
	return h
}

func defWindowProc(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func sendMessage(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(hwnd, msg, wp, lp)
	return r
}

func postMessage(hwnd, msg, wp, lp uintptr) {
	pPostMessageW.Call(hwnd, msg, wp, lp)
}

func invalidate(hwnd uintptr) {
	pInvalidateRect.Call(hwnd, 0, 0)
}

func setWindowText(hwnd uintptr, s string) {
	pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(wstr(s))))
}

func getWindowText(hwnd uintptr) string {
	n, _, _ := pGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func messageBox(hwnd uintptr, text, caption string, flags uintptr) {
	pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(wstr(text))), uintptr(unsafe.Pointer(wstr(caption))), flags)
}

func shellOpen(hwnd uintptr, path string) {
	pShellExecuteW.Call(hwnd, uintptr(unsafe.Pointer(wstr("open"))), uintptr(unsafe.Pointer(wstr(path))), 0, 0, swShowNormal)
}

func getClientRect(hwnd uintptr) RECT {
	var r RECT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func createSolidBrush(c uint32) uintptr {
	h, _, _ := pCreateSolidBrush.Call(uintptr(c))
	return h
}

func deleteObject(h uintptr) { pDeleteObject.Call(h) }

func selectObject(hdc, h uintptr) uintptr {
	r, _, _ := pSelectObject.Call(hdc, h)
	return r
}

func stockObject(i uintptr) uintptr {
	r, _, _ := pGetStockObject.Call(i)
	return r
}

func createFont(face string, heightPx int, weight int) uintptr {
	h, _, _ := pCreateFontW.Call(
		uintptr(-heightPx), 0, 0, 0, uintptr(weight), 0, 0, 0,
		defaultCharset, 0, 0, clearTypeQuality, 0,
		uintptr(unsafe.Pointer(wstr(face))))
	return h
}

func fillRect(hdc uintptr, r RECT, c uint32) {
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	pSetDCBrushColor.Call(hdc, uintptr(c))
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), stockObject(dcBrush))
}

func drawText(hdc uintptr, s string, r RECT, flags uintptr, color uint32, font uintptr) {
	if s == "" {
		return
	}
	u := wslice(s)
	selectObject(hdc, font)
	pSetBkMode.Call(hdc, bkTransparent)
	pSetTextColor.Call(hdc, uintptr(color))
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), flags)
}

func textWidth(hdc uintptr, s string, font uintptr) int {
	if s == "" {
		return 0
	}
	u := wslice(s)
	selectObject(hdc, font)
	var sz SIZE
	pGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&sz)))
	return int(sz.Cx)
}

func ptIn(r RECT, x, y int) bool {
	return x >= int(r.Left) && x < int(r.Right) && y >= int(r.Top) && y < int(r.Bottom)
}

func inset(r RECT, d int32) RECT {
	return RECT{r.Left + d, r.Top + d, r.Right - d, r.Bottom - d}
}

func offset(r RECT, dx, dy int32) RECT {
	return RECT{r.Left + dx, r.Top + dy, r.Right + dx, r.Bottom + dy}
}

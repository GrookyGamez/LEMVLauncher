//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

func init() { runtime.LockOSThread() }

// Logical (96 dpi) layout constants. Everything is multiplied by the DPI scale.
const (
	baseW    = 1000
	baseH    = 640
	sidebarW = 236
	rowHBase = 58
	cellBase = 8
)

// Palette: the launcher is drawn like a Minecraft menu screen - a cross-section
// of ground (grass strip, dirt) with the classic bevelled stone buttons.
var (
	colDirt        = []uint32{rgb(0x33, 0x25, 0x1a), rgb(0x3a, 0x2a, 0x1e), rgb(0x2e, 0x21, 0x17), rgb(0x40, 0x2e, 0x20), rgb(0x36, 0x27, 0x1b)}
	colDirtSide    = []uint32{rgb(0x2a, 0x1e, 0x15), rgb(0x30, 0x22, 0x18), rgb(0x26, 0x1b, 0x13), rgb(0x34, 0x25, 0x1a)}
	colDirtDark    = []uint32{rgb(0x17, 0x10, 0x0b), rgb(0x1b, 0x13, 0x0d), rgb(0x14, 0x0e, 0x09), rgb(0x1e, 0x15, 0x0f)}
	colGrass       = []uint32{rgb(0x5f, 0xa0, 0x3c), rgb(0x6b, 0xb0, 0x44), rgb(0x55, 0x92, 0x36), rgb(0x72, 0xba, 0x4b)}
	colGrassAccent = rgb(0x6b, 0xb0, 0x44)

	colText     = rgb(0xff, 0xff, 0xff)
	colTextSoft = rgb(0xd4, 0xd4, 0xd4)
	colTextDim  = rgb(0x9a, 0x9a, 0x9a)
	colYellow   = rgb(0xff, 0xff, 0xa0)
	colGreen    = rgb(0x55, 0xff, 0x55)
	colRed      = rgb(0xff, 0x55, 0x55)

	colBtn         = rgb(0x6f, 0x6f, 0x6f)
	colBtnHover    = rgb(0x7a, 0x8a, 0xc4)
	colBtnActive   = rgb(0x8c, 0x8c, 0x8c)
	colBtnDisabled = rgb(0x3a, 0x3a, 0x3a)

	colBorder       = rgb(0x00, 0x00, 0x00)
	colField        = rgb(0x00, 0x00, 0x00)
	colFieldBorder  = rgb(0xa0, 0xa0, 0xa0)
	colRowSel       = rgb(0x3c, 0x3c, 0x3c)
	colRowHover     = rgb(0x28, 0x28, 0x28)
	colRowSelBorder = rgb(0xa0, 0xa0, 0xa0)
	colProgressBg   = rgb(0x2a, 0x2a, 0x2a)
	colScrollThumb  = rgb(0x8a, 0x8a, 0x8a)
	colBadge        = rgb(0xc8, 0xc8, 0xc8)

	// modern-only surfaces
	colBgMain    = rgb(0x1e, 0x1f, 0x22)
	colSidebarBg = rgb(0x2b, 0x2d, 0x31)
	colListBg    = rgb(0x23, 0x24, 0x28)
	colCaption   = rgb(0x1a, 0x1b, 0x1e)
	colCapHover  = rgb(0x34, 0x36, 0x3c)
	colCloseHov  = rgb(0xed, 0x42, 0x45)
	colAccent    = rgb(0x3d, 0xd6, 0x8c)
	colAccentHov = rgb(0x5c, 0xe0, 0x9e)
	colOnAccent  = rgb(0x0e, 0x1a, 0x13)
	colSeam      = rgb(0x14, 0x15, 0x17)
	colSearchBg  = rgb(0x10, 0x11, 0x13)
)

// frameRound draws a 1px rounded outline.
func frameRound(hdc uintptr, r RECT, col uint32, rad int32) {
	pen, _, _ := pCreatePen.Call(0, 1, uintptr(col))
	nullBr, _, _ := pGetStockObject.Call(5) // NULL_BRUSH
	op := selectObject(hdc, pen)
	ob := selectObject(hdc, nullBr)
	pRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(rad), uintptr(rad))
	selectObject(hdc, op)
	selectObject(hdc, ob)
	deleteObject(pen)
}

// applyPalette points the shared color variables at the active theme.
func applyPalette(modern bool) {
	if modern {
		colText, colTextSoft, colTextDim = rgb(0xf2, 0xf3, 0xf5), rgb(0xb5, 0xba, 0xc1), rgb(0x80, 0x84, 0x8e)
		colYellow, colGreen, colRed = rgb(0xf0, 0xc6, 0x60), colAccent, rgb(0xed, 0x42, 0x45)
		colBtn, colBtnHover, colBtnActive, colBtnDisabled = rgb(0x3a, 0x3c, 0x42), rgb(0x45, 0x48, 0x4f), rgb(0x4e, 0x51, 0x57), rgb(0x2b, 0x2d, 0x31)
		colBorder, colField, colFieldBorder = colSeam, rgb(0x1a, 0x1b, 0x1e), rgb(0x3f, 0x41, 0x47)
		colRowSel, colRowHover, colRowSelBorder = rgb(0x3f, 0x42, 0x48), rgb(0x33, 0x35, 0x3b), colAccent
		colProgressBg, colScrollThumb, colBadge = rgb(0x2b, 0x2d, 0x31), rgb(0x4e, 0x51, 0x57), rgb(0x9a, 0x9f, 0xa6)
		colGrassAccent = colAccent
		return
	}
	colText, colTextSoft, colTextDim = rgb(0xff, 0xff, 0xff), rgb(0xd4, 0xd4, 0xd4), rgb(0x9a, 0x9a, 0x9a)
	colYellow, colGreen, colRed = rgb(0xff, 0xff, 0xa0), rgb(0x55, 0xff, 0x55), rgb(0xff, 0x55, 0x55)
	colBtn, colBtnHover, colBtnActive, colBtnDisabled = rgb(0x6f, 0x6f, 0x6f), rgb(0x7a, 0x8a, 0xc4), rgb(0x8c, 0x8c, 0x8c), rgb(0x3a, 0x3a, 0x3a)
	colBorder, colField, colFieldBorder = rgb(0, 0, 0), rgb(0, 0, 0), rgb(0xa0, 0xa0, 0xa0)
	colRowSel, colRowHover, colRowSelBorder = rgb(0x3c, 0x3c, 0x3c), rgb(0x28, 0x28, 0x28), rgb(0xa0, 0xa0, 0xa0)
	colProgressBg, colScrollThumb, colBadge = rgb(0x2a, 0x2a, 0x2a), rgb(0x8a, 0x8a, 0x8a), rgb(0xc8, 0xc8, 0xc8)
	colGrassAccent = rgb(0x6b, 0xb0, 0x44)
}

type hitKind int

const (
	hitNone hitKind = iota
	hitTab
	hitRow
	hitPlay
	hitOpenFolder
	hitRescan
	hitDownloadAll
	hitCopyLinks
	hitGetJar
	hitCapMin
	hitCapMax
	hitCapClose
	hitRailHome
	hitRailFolder
	hitRailRescan
	hitRailWorlds
	hitRailLogs
	hitRailGear
	hitRailInfo
	hitRailVersions
	hitMemMinus
	hitMemPlus
	hitTglClose
	hitTglName
	hitTglAnim
	hitTglImport
	hitTglRPC
	hitAccount
	hitCatCard
	hitPreCard
	hitReleaseCard
	hitRareCard
	hitBack
	hitHomeCard
)

const (
	viewHome = iota
	viewCats
	viewList
	viewSettings
	viewRareCats
	viewPreCats
	viewReleaseCats
	viewInfo
)

type hit struct {
	kind  hitKind
	index int
}

type layout struct {
	W, H      int
	sidebar   RECT
	title     RECT
	tabs      [TabCount]RECT
	btnOpen   RECT
	btnRescan RECT
	header    RECT
	hint      RECT
	btnDlAll  RECT
	searchBox RECT

	caption  RECT
	capMin   RECT
	capMax   RECT
	capClose RECT

	playBar RECT
	verInfo RECT
	verPill RECT

	rail       RECT
	railHome   RECT
	railCats   RECT
	railFolder RECT
	railWorlds RECT
	railLogs   RECT
	railRescan RECT
	railGear   RECT
	railInfo   RECT
	topBar     RECT
	crumb      RECT

	catCards [TabCount]RECT
	btnBack  RECT
	btnCopy  RECT

	homeCards [3]RECT

	setMem     RECT
	setMemDec  RECT
	setMemVal  RECT
	setMemInc  RECT
	setClose   RECT
	setClsTgl  RECT
	setName    RECT
	setNameTgl RECT
	setAnim    RECT
	setAnimTgl RECT
	setImp     RECT
	setImpTgl  RECT
	setRPC     RECT
	setRPCTgl  RECT
	setAcc     RECT
	setAccBtn  RECT
	setArgs    RECT
	setArgsBox RECT

	list      RECT
	rowH      int
	userLabel RECT
	editFrame RECT
	editCtl   RECT
	play      RECT
	status    RECT
	progress  RECT
}

type app struct {
	L          *Launcher
	hwnd       uintptr
	hEdit      uintptr
	hSearch    uintptr
	hArgs      uintptr
	view       int
	pendAdd    []*Entry          // expansion rows waiting to be merged on the UI thread
	pendDates  map[string]string // manifest release dates for ordering
	rareFilter int               // -1 = all rare rows; otherwise a rareKind
	anim       float64
	hInst      uintptr
	dpi        int
	scale      float64
	fonts      struct {
		title, sub, h1, body, small, mono, monoSmall, btn, play, edit uintptr
	}
	hbrSearch     uintptr
	hbrField      uintptr
	cursorArrow   uintptr
	cursorHand    uintptr
	editOldProc   uintptr
	searchOldProc uintptr

	query   string   // current search text (empty = no search)
	results []*Entry // entries matching query, across all tabs

	bg struct {
		dc, bmp, old uintptr
		w, h         int
	}

	tab      int
	sel      [TabCount + 1]int
	scroll   [TabCount + 1]int
	hover    hit
	tracking bool

	mu           sync.Mutex
	busy         bool
	running      *LaunchResult
	status       string
	statusErr    bool
	progress     float64
	lastPost     time.Time
	pendingError string
}

const idleStatus = "Pick a version, type a username and hit Play."

var ui *app

func (a *app) modern() bool { return true }

// fillRound paints a flat rounded rectangle (the modern theme's basic shape).
func (a *app) fillRound(hdc uintptr, r RECT, col uint32, rad int32) {
	br := createSolidBrush(col)
	pen, _, _ := pCreatePen.Call(0, 1, uintptr(col))
	ob := selectObject(hdc, br)
	op := selectObject(hdc, pen)
	pRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(rad), uintptr(rad))
	selectObject(hdc, ob)
	selectObject(hdc, op)
	deleteObject(br)
	deleteObject(pen)
}

const negOne = ^uintptr(0)

// ---------------------------------------------------------------------------
// entry point

func runUI(L *Launcher) {
	applyPalette(true)
	a := &app{L: L, progress: -1, anim: 1, rareFilter: -1}
	for i := range a.sel {
		a.sel[i] = -1 // nothing selected until the user picks
	}
	ui = a
	a.hInst = getModuleHandle()
	pSetProcessDPIAware.Call()
	a.dpi = systemDPI()
	if a.dpi < 72 {
		a.dpi = 96
	}
	a.scale = float64(a.dpi) / 96
	a.makeFonts()
	a.hbrField = createSolidBrush(colField)
	a.hbrSearch = createSolidBrush(colSearchBg)
	a.cursorArrow = loadCursor(idcArrow)
	a.cursorHand = loadCursor(idcHand)
	icon := loadIcon(a.hInst, 1)

	className := wstr("LEMVLauncherWindow")
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         csHRedraw | csVRedraw | csDblClks,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     a.hInst,
		HIcon:         icon,
		HCursor:       a.cursorArrow,
		LpszClassName: className,
		HIconSm:       icon,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		messageBox(0, "Couldn't register the window class: "+err.Error(), "LEMV Launcher", mbOK|mbIconError)
		return
	}

	l := a.layout()
	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox | wsMaximizeBox | wsThickFrame | wsClipChildren)
	rc := RECT{0, 0, int32(l.W), int32(l.H)}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&rc)), style, 0, 0)
	ww, wh := int(rc.Right-rc.Left), int(rc.Bottom-rc.Top)
	sx, _, _ := pGetSystemMetrics.Call(smCxScreen)
	sy, _, _ := pGetSystemMetrics.Call(smCyScreen)
	x, y := (int(sx)-ww)/2, (int(sy)-wh)/2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wstr("LEMV Launcher"))),
		style, uintptr(x), uintptr(y), uintptr(ww), uintptr(wh), 0, 0, a.hInst, 0)
	if hwnd == 0 {
		messageBox(0, "Couldn't create the launcher window: "+err.Error(), "LEMV Launcher", mbOK|mbIconError)
		return
	}
	a.hwnd = hwnd

	ec := l.editCtl
	a.hEdit, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(wstr("EDIT"))), 0,
		wsChild|wsVisible|wsTabStop|esAutoHScroll, uintptr(ec.Left), uintptr(ec.Top), uintptr(ec.Right-ec.Left), uintptr(ec.Bottom-ec.Top),
		hwnd, 101, a.hInst, 0)
	sendMessage(a.hEdit, wmSetFont, a.fonts.edit, 1)
	sendMessage(a.hEdit, emLimitText, 16, 0)
	sendMessage(a.hEdit, emSetCueBanner, 1, uintptr(unsafe.Pointer(wstr("Username"))))
	if L.Settings.RememberName && L.Settings.Username != "" {
		pSetWindowText.Call(a.hEdit, uintptr(unsafe.Pointer(wstr(L.Settings.Username))))
	}
	a.editOldProc, _, _ = pSetWindowLongPtrW.Call(a.hEdit, ^uintptr(3) /* GWLP_WNDPROC */, syscall.NewCallback(editProc))

	sb := l.searchBox
	pad := a.sc(6)
	a.hSearch, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(wstr("EDIT"))), 0,
		wsChild|wsVisible|wsTabStop|esAutoHScroll, uintptr(sb.Left+pad), uintptr(sb.Top+a.sc(8)), uintptr(sb.Right-sb.Left-2*pad), uintptr(sb.Bottom-sb.Top-a.sc(14)),
		hwnd, 102, a.hInst, 0)
	sendMessage(a.hSearch, wmSetFont, a.fonts.edit, 1)
	sendMessage(a.hSearch, emLimitText, 40, 0)
	sendMessage(a.hSearch, emSetCueBanner, 1, uintptr(unsafe.Pointer(wstr("Search versions"))))
	a.searchOldProc, _, _ = pSetWindowLongPtrW.Call(a.hSearch, ^uintptr(3) /* GWLP_WNDPROC */, syscall.NewCallback(searchProc))

	ab := l.setArgsBox
	a.hArgs, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(wstr("EDIT"))), 0,
		wsChild|wsTabStop|esAutoHScroll, uintptr(ab.Left+pad), uintptr(ab.Top+a.sc(7)), uintptr(ab.Right-ab.Left-2*pad), uintptr(ab.Bottom-ab.Top-a.sc(13)),
		hwnd, 103, a.hInst, 0)
	sendMessage(a.hArgs, wmSetFont, a.fonts.edit, 1)
	sendMessage(a.hArgs, emLimitText, 300, 0)

	// restore where the user left off
	if L.Settings.LastTab >= 0 && L.Settings.LastTab < TabCount {
		a.tab = L.Settings.LastTab
	}
	if L.Settings.LastVersion != "" {
		for i, e := range L.EntriesForTab(a.tab) {
			if e.ID == L.Settings.LastVersion {
				a.sel[a.curTab()] = i
			}
		}
	}
	a.setStatus(idleStatus, -1, false)

	go func() {
		add, dates := a.L.ExpandFromManifest()
		if len(add) == 0 {
			return
		}
		a.mu.Lock()
		a.pendAdd, a.pendDates = add, dates
		a.mu.Unlock()
		postMessage(a.hwnd, wmAppEntriesReady, 0, 0)
	}()

	pSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
	pShowWindow.Call(hwnd, swShow)
	pUpdateWindow.Call(hwnd)
	if L.Settings.Username == "" {
		pSetFocus.Call(a.hEdit)
	} else {
		pSetFocus.Call(hwnd)
	}

	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func systemDPI() int {
	if pGetDpiForSystem.Find() == nil {
		if d, _, _ := pGetDpiForSystem.Call(); d > 0 {
			return int(d)
		}
	}
	hdc, _, _ := pGetDC.Call(0)
	d, _, _ := pGetDeviceCaps.Call(hdc, logPixelsX)
	pReleaseDC.Call(0, hdc)
	if d == 0 {
		return 96
	}
	return int(d)
}

func (a *app) px(pt float64) int { return int(math.Round(pt * float64(a.dpi) / 72)) }

func (a *app) sc(v float64) int32 { return int32(math.Round(v * a.scale)) }

func (a *app) makeFonts() {
	a.fonts.title = createFont("Segoe UI", a.px(17), fwBold)
	a.fonts.sub = createFont("Segoe UI", a.px(9), fwNormal)
	a.fonts.h1 = createFont("Segoe UI", a.px(16), fwBold)
	a.fonts.body = createFont("Segoe UI", a.px(10), fwNormal)
	a.fonts.small = createFont("Segoe UI", a.px(8.5), fwNormal)
	a.fonts.mono = createFont("Consolas", a.px(12), fwBold)
	a.fonts.monoSmall = createFont("Consolas", a.px(8.5), fwNormal)
	a.fonts.btn = createFont("Segoe UI", a.px(10), fwBold)
	a.fonts.play = createFont("Segoe UI", a.px(15), fwBold)
	a.fonts.edit = createFont("Consolas", a.px(11), fwNormal)
}

// ---------------------------------------------------------------------------
// layout

func (a *app) layout() layout {
	var l layout
	s := a.sc
	l.W, l.H = int(s(baseW)), int(s(baseH))
	if a.hwnd != 0 {
		rc := getClientRect(a.hwnd)
		if int(rc.Right) > l.W {
			l.W = int(rc.Right)
		}
		if int(rc.Bottom) > l.H {
			l.H = int(rc.Bottom)
		}
	}
	W, H := int32(l.W), int32(l.H)

	// slim icon rail down the left, Lunar-style
	rw := s(56)
	l.rail = RECT{0, 0, rw, H}
	icon := func(y int32) RECT { return RECT{s(10), y, rw - s(10), y + s(36)} }
	l.railHome = icon(s(12))
	l.railCats = icon(s(64)) // Versions
	l.railFolder = icon(s(112))
	l.railWorlds = icon(s(160))
	l.railLogs = icon(s(208))
	l.railInfo = icon(s(256))
	l.railGear = icon(H - s(96))
	l.railRescan = icon(H - s(48))

	// top bar doubles as the window caption
	top := s(52)
	l.topBar = RECT{rw, 0, W, top}
	l.caption = l.topBar
	bw := s(46)
	l.capClose = RECT{W - bw, 0, W, top}
	l.capMax = RECT{W - 2*bw, 0, W - bw, top}
	l.capMin = RECT{W - 3*bw, 0, W - 2*bw, top}
	l.searchBox = RECT{l.capMin.Left - s(14) - s(220), s(10), l.capMin.Left - s(14), top - s(10)}
	l.verPill = RECT{l.searchBox.Left - s(12) - s(150), s(11), l.searchBox.Left - s(12), top - s(11)}
	l.crumb = RECT{rw + s(24), 0, l.searchBox.Left - s(16), top}

	mx := rw + s(28)

	// ---- categories view -------------------------------------------------
	gx, gy := mx, top+s(64)
	gw := (W - s(28) - gx - 2*s(14)) / 3
	gh := s(108)
	for i := range l.catCards {
		col, row := int32(i%3), int32(i/3)
		x := gx + col*(gw+s(14))
		y := gy + row*(gh+s(14))
		l.catCards[i] = RECT{x, y, x + gw, y + gh}
	}

	// ---- home cards --------------------------------------------------------
	hcw, hch := s(212), s(96)
	hcy := (top+H-s(88))/2 + s(56)
	hcn := int32(len(l.homeCards))
	hcx0 := l.rail.Right + (W-l.rail.Right)/2 - (hcn*hcw+(hcn-1)*s(14))/2
	for i := range l.homeCards {
		x := hcx0 + int32(i)*(hcw+s(14))
		l.homeCards[i] = RECT{x, hcy, x + hcw, hcy + hch}
	}

	// ---- settings view ---------------------------------------------------
	sy := top + s(72)
	rh := s(56)
	row := func() RECT { r := RECT{mx + s(4), sy, W - s(40), sy + rh}; sy += rh + s(10); return r }
	l.setMem = row()
	l.setMemInc = RECT{l.setMem.Right - s(34), l.setMem.Top + s(11), l.setMem.Right, l.setMem.Bottom - s(11)}
	l.setMemVal = RECT{l.setMemInc.Left - s(120), l.setMemInc.Top, l.setMemInc.Left, l.setMemInc.Bottom}
	l.setMemDec = RECT{l.setMemVal.Left - s(34), l.setMemInc.Top, l.setMemVal.Left, l.setMemInc.Bottom}
	l.setClose = row()
	l.setClsTgl = RECT{l.setClose.Right - s(46), l.setClose.Top + s(15), l.setClose.Right, l.setClose.Bottom - s(15)}
	l.setName = row()
	l.setNameTgl = RECT{l.setName.Right - s(46), l.setName.Top + s(15), l.setName.Right, l.setName.Bottom - s(15)}
	l.setAnim = row()
	l.setAnimTgl = RECT{l.setAnim.Right - s(46), l.setAnim.Top + s(15), l.setAnim.Right, l.setAnim.Bottom - s(15)}
	l.setImp = row()
	l.setImpTgl = RECT{l.setImp.Right - s(46), l.setImp.Top + s(15), l.setImp.Right, l.setImp.Bottom - s(15)}
	l.setRPC = row()
	l.setRPCTgl = RECT{l.setRPC.Right - s(46), l.setRPC.Top + s(15), l.setRPC.Right, l.setRPC.Bottom - s(15)}
	if msaEnabled {
		l.setAcc = row()
		l.setAccBtn = RECT{l.setAcc.Right - s(140), l.setAcc.Top + s(10), l.setAcc.Right, l.setAcc.Bottom - s(10)}
	}
	l.setArgs = row()
	l.setArgsBox = RECT{l.setArgs.Left, l.setArgs.Bottom - s(2), l.setArgs.Right, l.setArgs.Bottom + s(34)}

	// ---- list view -------------------------------------------------------
	l.btnBack = RECT{mx, top + s(18), mx + s(40), top + s(52)}
	l.btnDlAll = RECT{W - s(28) - s(160), top + s(16), W - s(28), top + s(54)}
	l.btnCopy = RECT{l.btnDlAll.Left - s(10) - s(150), top + s(16), l.btnDlAll.Left - s(10), top + s(54)}
	l.header = RECT{l.btnBack.Right + s(10), top + s(14), l.btnDlAll.Left - s(16), top + s(46)}
	l.hint = RECT{l.btnBack.Right + s(10), top + s(48), W - s(28), top + s(70)}
	l.list = RECT{rw + s(22), top + s(84), W - s(22), H - s(88) - s(10)}
	l.rowH = int(s(rowHBase))

	l.playBar = RECT{rw + 1, H - s(88), W, H}
	l.verInfo = RECT{rw + s(26), l.playBar.Top + s(14), rw + s(26) + s(250), H - s(12)}
	l.play = RECT{W - s(24) - s(190), l.playBar.Top + s(18), W - s(24), H - s(18)}
	l.editFrame = RECT{l.play.Left - s(14) - s(230), l.playBar.Top + s(22), l.play.Left - s(14), H - s(22)}
	l.status = l.hint
	l.progress = RECT{l.playBar.Left, l.playBar.Top, l.playBar.Right, l.playBar.Top + s(3)}
	l.userLabel = RECT{}

	ef := l.editFrame
	eh := s(22)
	ey := ef.Top + (ef.Bottom-ef.Top-eh)/2
	l.editCtl = RECT{ef.Left + s(10), ey, ef.Right - s(10), ey + eh}
	return l
}

// ---------------------------------------------------------------------------
// state helpers

// curTab is the tab whose rows are on screen; TabCount = search results.
func (a *app) curTab() int {
	if a.query != "" {
		return TabCount
	}
	return a.tab
}

func (a *app) entries() []*Entry {
	if a.query != "" {
		return a.results
	}
	es := a.L.EntriesForTab(a.tab)
	if a.tab == TabLost && a.rareFilter >= 0 {
		var out []*Entry
		for _, e := range es {
			if e.Rare == a.rareFilter {
				out = append(out, e)
			}
		}
		return out
	}
	return es
}

func (a *app) setSearch(q string) {
	a.query = strings.TrimSpace(q)
	if a.query != "" && a.view != viewList {
		a.view = viewList
		a.positionEdits()
		pShowWindow.Call(a.hEdit, swShow)
	}
	a.results = a.L.Search(a.query)
	a.sel[TabCount], a.scroll[TabCount] = 0, 0
	invalidate(a.hwnd)
}

func (a *app) isBusy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.busy
}

// setStatus is safe to call from any goroutine.
func (a *app) setStatus(msg string, frac float64, isErr bool) {
	a.mu.Lock()
	changed := msg != a.status || isErr != a.statusErr
	a.status, a.progress, a.statusErr = msg, frac, isErr
	now := time.Now()
	post := changed || frac >= 1 || now.Sub(a.lastPost) > 50*time.Millisecond
	if post {
		a.lastPost = now
	}
	a.mu.Unlock()
	if post && a.hwnd != 0 {
		postMessage(a.hwnd, wmAppRefresh, 0, 0)
	}
}

func (a *app) clampSel() {
	for t := 0; t < TabCount; t++ {
		n := len(a.L.EntriesForTab(t))
		if a.sel[t] >= n {
			a.sel[t] = n - 1
		}
		if a.sel[t] < 0 {
			a.sel[t] = 0
		}
	}
}

func (a *app) clampScroll(l layout) {
	content := len(a.entries()) * l.rowH
	visible := int(l.list.Bottom - l.list.Top)
	max := content - visible
	if max < 0 {
		max = 0
	}
	if a.scroll[a.curTab()] > max {
		a.scroll[a.curTab()] = max
	}
	if a.scroll[a.curTab()] < 0 {
		a.scroll[a.curTab()] = 0
	}
}

func (a *app) ensureVisible(i int) {
	l := a.layout()
	top := i * l.rowH
	bottom := top + l.rowH
	visible := int(l.list.Bottom - l.list.Top)
	if top < a.scroll[a.curTab()] {
		a.scroll[a.curTab()] = top
	} else if bottom > a.scroll[a.curTab()]+visible {
		a.scroll[a.curTab()] = bottom - visible
	}
	a.clampScroll(l)
}

func (a *app) selectedEntry() *Entry {
	es := a.entries()
	i := a.sel[a.curTab()]
	if i < 0 || i >= len(es) {
		return nil
	}
	return es[i]
}

func (a *app) rescan() {
	var keep [TabCount]string
	for t := 0; t < TabCount; t++ {
		es := a.L.EntriesForTab(t)
		if a.sel[t] >= 0 && a.sel[t] < len(es) {
			keep[t] = es[a.sel[t]].ID
		}
	}
	a.L.Rescan()
	if n := len(a.L.LastImported); n > 0 {
		a.setStatus(fmt.Sprintf("Imported %d jar(s) from Downloads: %s", n, strings.Join(a.L.LastImported, ", ")), -1, false)
	}
	for t := 0; t < TabCount; t++ {
		for i, e := range a.L.EntriesForTab(t) {
			if e.ID == keep[t] {
				a.sel[t] = i
			}
		}
	}
	a.clampSel()
	invalidate(a.hwnd)
}

func (a *app) saveState() {
	a.L.Settings.Username = strings.TrimSpace(getWindowText(a.hEdit))
	a.L.Settings.LastTab = a.tab
	if e := a.selectedEntry(); e != nil {
		a.L.Settings.LastVersion = e.ID
	}
	a.L.SaveSettings()
}

// ---------------------------------------------------------------------------
// input

// rowRect returns the on-screen rectangle of list row i (in the current tab),
// accounting for scroll. It matches the geometry used in drawMain.
func (a *app) rowRect(l layout, i int) RECT {
	pad := a.sc(10)
	top := int32(int(l.list.Top) - a.scroll[a.curTab()] + i*l.rowH)
	return RECT{l.list.Left + pad, top + a.sc(4), l.list.Right - pad - a.sc(14), top + int32(l.rowH) - a.sc(4)}
}

// getJarButtonRect returns the "Get this jar" button rect for a drop-in row,
// or ok=false for rows that don't show the button.
func (a *app) getJarButtonRect(l layout, i int) (RECT, bool) {
	es := a.entries()
	if i < 0 || i >= len(es) {
		return RECT{}, false
	}
	e := es[i]
	if !e.DropInOnly || e.Ready() || e.GetURL == "" {
		return RECT{}, false
	}
	r := a.rowRect(l, i)
	bw, bh := a.sc(104), a.sc(22)
	bx := r.Right - bw
	by := r.Top + (r.Bottom-r.Top-bh)/2
	return RECT{bx, by, bx + bw, by + bh}, true
}

func (a *app) hitTest(x, y int) hit {
	l := a.layout()
	switch {
	case ptIn(l.capMin, x, y):
		return hit{hitCapMin, 0}
	case ptIn(l.capMax, x, y):
		return hit{hitCapMax, 0}
	case ptIn(l.capClose, x, y):
		return hit{hitCapClose, 0}
	case ptIn(l.railHome, x, y):
		return hit{hitRailHome, 0}
	case ptIn(l.railCats, x, y):
		return hit{hitRailVersions, 0}
	case ptIn(l.railFolder, x, y):
		return hit{hitRailFolder, 0}
	case ptIn(l.railRescan, x, y):
		return hit{hitRailRescan, 0}
	case ptIn(l.railWorlds, x, y):
		return hit{hitRailWorlds, 0}
	case ptIn(l.railLogs, x, y):
		return hit{hitRailLogs, 0}
	case ptIn(l.railGear, x, y):
		return hit{hitRailGear, 0}
	case ptIn(l.railInfo, x, y):
		return hit{hitRailInfo, 0}
	}
	if a.view == viewSettings {
		switch {
		case ptIn(l.setMemDec, x, y):
			return hit{hitMemMinus, 0}
		case ptIn(l.setMemInc, x, y):
			return hit{hitMemPlus, 0}
		case ptIn(l.setClsTgl, x, y):
			return hit{hitTglClose, 0}
		case ptIn(l.setNameTgl, x, y):
			return hit{hitTglName, 0}
		case ptIn(l.setAnimTgl, x, y):
			return hit{hitTglAnim, 0}
		case ptIn(l.setImpTgl, x, y):
			return hit{hitTglImport, 0}
		case ptIn(l.setRPCTgl, x, y):
			return hit{hitTglRPC, 0}
		case msaEnabled && ptIn(l.setAccBtn, x, y):
			return hit{hitAccount, 0}
		}
		return hit{hitNone, 0}
	}
	switch a.view {
	case viewHome:
		if ptIn(l.play, x, y) {
			return hit{hitPlay, 0}
		}
		for i := range l.homeCards {
			if ptIn(l.homeCards[i], x, y) {
				return hit{hitHomeCard, i}
			}
		}
	case viewCats:
		for i := range topTabs {
			if ptIn(l.catCards[i], x, y) {
				return hit{hitCatCard, i}
			}
		}
	case viewPreCats:
		if ptIn(l.btnBack, x, y) {
			return hit{hitBack, 0}
		}
		for i := range preTabs {
			if ptIn(l.catCards[i], x, y) {
				return hit{hitPreCard, i}
			}
		}
	case viewReleaseCats:
		if ptIn(l.btnBack, x, y) {
			return hit{hitBack, 0}
		}
		for i := range releaseTabs {
			if ptIn(l.catCards[i], x, y) {
				return hit{hitReleaseCard, i}
			}
		}
	case viewRareCats:
		if ptIn(l.btnBack, x, y) {
			return hit{hitBack, 0}
		}
		for i := range RareKinds {
			if ptIn(l.catCards[i], x, y) {
				return hit{hitRareCard, i}
			}
		}
	case viewList:
		if ptIn(l.btnBack, x, y) {
			return hit{hitBack, 0}
		}
		if ptIn(l.btnDlAll, x, y) {
			return hit{hitDownloadAll, 0}
		}
		if ptIn(l.btnCopy, x, y) && len(a.missingArchived()) > 0 {
			return hit{hitCopyLinks, 0}
		}
		if ptIn(l.play, x, y) {
			return hit{hitPlay, 0}
		}
		if ptIn(l.list, x, y) {
			i := (y - int(l.list.Top) + a.scroll[a.curTab()]) / l.rowH
			if i >= 0 && i < len(a.entries()) {
				// the Download button sits on top of its row
				if br, ok := a.getJarButtonRect(l, i); ok && ptIn(br, x, y) {
					return hit{hitGetJar, i}
				}
				return hit{hitRow, i}
			}
		}
	}
	return hit{hitNone, 0}
}

// selectEntry points the launcher at a specific version from anywhere.
func (a *app) selectEntry(e *Entry) {
	a.tab = e.Tab
	for i, x := range a.L.EntriesForTab(e.Tab) {
		if x == e {
			a.sel[e.Tab] = i
			break
		}
	}
	invalidate(a.hwnd)
}

// onAccountButton signs in (device-code flow) or signs out.
func (a *app) onAccountButton() {
	if a.L.Settings.Account != nil {
		a.L.Settings.Account = nil
		a.L.SaveSettings()
		a.setStatus("Signed out — launches use offline mode again.", -1, false)
		invalidate(a.hwnd)
		return
	}
	cid := strings.TrimSpace(a.L.Settings.MSAClientID)
	if cid == "" {
		a.setStatus("Set msaClientId in launcher-settings.json first — the README walks you through it.", -1, true)
		return
	}
	a.setStatus("Asking Microsoft for a sign-in code…", -1, false)
	go func() {
		dc, err := StartDeviceLogin(cid)
		if err != nil {
			a.setStatus("Sign-in failed: "+err.Error(), -1, true)
			return
		}
		a.setStatus(fmt.Sprintf("Enter code  %s  at %s — waiting for you…", dc.UserCode, dc.VerificationURI), -1, false)
		shellOpen(a.hwnd, dc.VerificationURI)
		acc, err := PollDeviceLogin(cid, dc, func(m string) { a.setStatus(m, -1, false) })
		if err != nil {
			a.setStatus("Sign-in failed: "+err.Error(), -1, true)
			return
		}
		a.L.Settings.Account = acc
		a.L.SaveSettings()
		a.setStatus("Signed in as "+acc.Name+" — launches now use your real account.", -1, false)
		invalidate(a.hwnd)
	}()
}

// setClipboard puts UTF-16 text on the Windows clipboard.
func setClipboard(hwnd uintptr, text string) bool {
	if ok, _, _ := pOpenClipboard.Call(hwnd); ok == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	u16 := utf16.Encode([]rune(text + "\x00"))
	sz := uintptr(len(u16) * 2)
	h, _, _ := pGlobalAlloc.Call(0x0042 /* GMEM_MOVEABLE|GMEM_ZEROINIT */, sz)
	if h == 0 {
		return false
	}
	dst, _, _ := pGlobalLock.Call(h)
	if dst == 0 {
		return false
	}
	for i, v := range u16 {
		*(*uint16)(unsafe.Pointer(dst + uintptr(i*2))) = v
	}
	pGlobalUnlock.Call(h)
	ok, _, _ := pSetClipboardData.Call(13 /* CF_UNICODETEXT */, h)
	return ok != 0
}

// missingArchived lists rows on the current tab whose jar must come from the
// archive (Mojang doesn't host them) and isn't on disk yet.
func (a *app) missingArchived() []*Entry {
	var out []*Entry
	for _, e := range a.entries() {
		if !e.Ready() && e.DropInOnly && e.GetURL != "" {
			out = append(out, e)
		}
	}
	return out
}

// onCopyLinks hands every archive link for this tab to the user at once: the
// clipboard for a download manager, plus a text file they can feed to one.
// The launcher deliberately doesn't fetch these itself.
func (a *app) onCopyLinks() {
	missing := a.missingArchived()
	if len(missing) == 0 {
		return
	}
	urls := make([]string, 0, len(missing))
	for _, e := range missing {
		urls = append(urls, e.GetURL)
	}
	text := strings.Join(urls, "\r\n") + "\r\n"
	path := filepath.Join(a.L.Root, "archive-links.txt")
	os.WriteFile(path, []byte(text), 0o644)
	if setClipboard(a.hwnd, text) {
		a.setStatus(fmt.Sprintf("Copied %d links — paste them into your download manager and I'll import them from Downloads automatically.", len(missing)), -1, false)
	} else {
		a.setStatus(fmt.Sprintf("Wrote %d links to %s — feed that list to your download manager.", len(missing), path), -1, false)
	}
	invalidate(a.hwnd)
}

func (a *app) onOpenFolder() {
	os.MkdirAll(a.L.VersionsDir, 0o755)
	shellOpen(a.hwnd, a.L.VersionsDir)
}

// setView switches between home, categories and the version list.
func (a *app) setView(v int) {
	if a.view == v {
		invalidate(a.hwnd)
		return
	}
	a.view = v
	a.positionEdits()
	show := uintptr(swShow)
	if v == viewCats || v == viewSettings || v == viewRareCats || v == viewPreCats || v == viewReleaseCats || v == viewInfo {
		show = swHide
	}
	pShowWindow.Call(a.hEdit, show)
	if v == viewSettings {
		pSetWindowText.Call(a.hArgs, uintptr(unsafe.Pointer(wstr(strings.Join(a.L.Settings.ExtraJVMArgs, " ")))))
		pShowWindow.Call(a.hArgs, swShow)
	} else {
		pShowWindow.Call(a.hArgs, swHide)
	}
	if a.L.Settings.Animations {
		a.anim = 0
		pSetTimer.Call(a.hwnd, 2, 15, 0)
	} else {
		a.anim = 1
	}
	invalidate(a.hwnd)
}

func (a *app) bumpMemory(delta int) {
	m := a.L.Settings.MaxMemoryMB + delta
	if m < 1024 {
		m = 1024
	}
	if m > 16384 {
		m = 16384
	}
	a.L.Settings.MaxMemoryMB = m
	a.L.SaveSettings()
	invalidate(a.hwnd)
}

func (a *app) onMouseMove(x, y int) {
	if !a.tracking {
		tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: tmeLeave, HwndTrack: a.hwnd}
		pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		a.tracking = true
	}
	h := a.hitTest(x, y)
	if h != a.hover {
		a.hover = h
		invalidate(a.hwnd)
	}
}

func (a *app) onClick(x, y int, double bool) {
	pSetFocus.Call(a.hwnd)
	h := a.hitTest(x, y)
	switch h.kind {
	case hitRailHome:
		if a.query != "" {
			sendMessage(a.hSearch, wmSetText, 0, uintptr(unsafe.Pointer(wstr(""))))
		}
		a.setView(viewHome)
	case hitRailVersions:
		if a.query != "" {
			sendMessage(a.hSearch, wmSetText, 0, uintptr(unsafe.Pointer(wstr(""))))
		}
		a.setView(viewCats)
	case hitRailFolder:
		a.onOpenFolder()
	case hitRailRescan:
		a.rescan()
		if len(a.L.LastImported) == 0 {
			a.setStatus("Rescanned the versions folder.", -1, false)
		}
	case hitRailWorlds:
		os.MkdirAll(a.L.InstancesDir, 0o755)
		shellOpen(a.hwnd, a.L.InstancesDir)
	case hitRailLogs:
		os.MkdirAll(a.L.LogsDir, 0o755)
		shellOpen(a.hwnd, a.L.LogsDir)
	case hitRailGear:
		a.setView(viewSettings)
	case hitRailInfo:
		a.setView(viewInfo)
	case hitMemMinus:
		a.bumpMemory(-512)
	case hitMemPlus:
		a.bumpMemory(512)
	case hitTglClose:
		a.L.Settings.CloseOnLaunch = !a.L.Settings.CloseOnLaunch
		a.L.SaveSettings()
		invalidate(a.hwnd)
	case hitTglName:
		a.L.Settings.RememberName = !a.L.Settings.RememberName
		a.L.SaveSettings()
		invalidate(a.hwnd)
	case hitTglAnim:
		a.L.Settings.Animations = !a.L.Settings.Animations
		a.L.SaveSettings()
		invalidate(a.hwnd)
	case hitTglImport:
		a.L.Settings.AutoImport = !a.L.Settings.AutoImport
		a.L.SaveSettings()
		invalidate(a.hwnd)
	case hitTglRPC:
		a.L.Settings.DiscordRPC = !a.L.Settings.DiscordRPC
		a.L.SaveSettings()
		invalidate(a.hwnd)
	case hitAccount:
		a.onAccountButton()
	case hitBack:
		if a.query != "" {
			sendMessage(a.hSearch, wmSetText, 0, uintptr(unsafe.Pointer(wstr(""))))
		}
		switch {
		case a.view == viewList && a.tab == TabLost && a.rareFilter >= 0:
			a.setView(viewRareCats)
		case a.view == viewList && isPreTab(a.tab):
			a.setView(viewPreCats)
		case a.view == viewList && (a.tab == TabRelease || a.tab == TabMinor):
			a.setView(viewReleaseCats)
		case a.view == viewReleaseCats:
			a.setView(viewCats)
		default:
			a.setView(viewCats)
		}
	case hitHomeCard:
		switch h.index {
		case 0: // continue where you left off
			if e := a.lastPlayed(); e != nil {
				a.selectEntry(e)
				a.onPlay()
			} else {
				a.setView(viewCats)
				a.setStatus("Pick a version to get started.", -1, false)
			}
		case 1: // browse versions
			a.setView(viewCats)
		case 2: // surprise me
			a.surpriseMe()
		}
	case hitCatCard:
		if h.index < 0 || h.index >= len(topTabs) {
			break
		}
		switch t := topTabs[h.index]; t {
		case tabReleaseGroup:
			a.setView(viewReleaseCats)
		case tabPreRelease:
			a.setView(viewPreCats)
		case TabLost:
			a.tab = TabLost
			a.setView(viewRareCats)
		default:
			a.tab = t
			a.rareFilter = -1
			a.setView(viewList)
		}
	case hitPreCard:
		if h.index >= 0 && h.index < len(preTabs) {
			a.tab = preTabs[h.index]
			a.rareFilter = -1
			a.setView(viewList)
		}
	case hitReleaseCard:
		if h.index >= 0 && h.index < len(releaseTabs) {
			a.tab = releaseTabs[h.index]
			a.rareFilter = -1
			a.setView(viewList)
		}
	case hitRareCard:
		if h.index >= 0 && h.index < len(RareKinds) {
			a.rareFilter = h.index
			a.tab = TabLost
			a.sel[TabLost], a.scroll[TabLost] = 0, 0
			a.setView(viewList)
		}
	case hitRow:
		a.sel[a.curTab()] = h.index
		invalidate(a.hwnd)
		if double {
			a.onPlay()
		}
	case hitGetJar:
		a.sel[a.curTab()] = h.index
		es := a.entries()
		if h.index >= 0 && h.index < len(es) {
			e := es[h.index]
			if e.GetURL != "" {
				shellOpen(a.hwnd, e.GetURL)
				os.MkdirAll(a.L.VersionsDir, 0o755)
				a.setStatus(fmt.Sprintf("Downloading %s in your browser — I'll pull it in from Downloads automatically.", e.JarName()), -1, false)
			}
		}
		invalidate(a.hwnd)
	case hitPlay:
		a.onPlay()
	case hitDownloadAll:
		a.onDownloadAll()
	case hitCopyLinks:
		a.onCopyLinks()
	case hitCapMin:
		sendMessage(a.hwnd, wmSysCommand, scMinimize, 0)
	case hitCapMax:
		if z, _, _ := pIsZoomed.Call(a.hwnd); z != 0 {
			sendMessage(a.hwnd, wmSysCommand, scRestore, 0)
		} else {
			sendMessage(a.hwnd, wmSysCommand, scMaximize, 0)
		}
	case hitCapClose:
		postMessage(a.hwnd, wmClose, 0, 0)
	case hitRescan:
		a.rescan()
		n := 0
		for _, e := range a.L.Entries {
			if e.Ready() {
				n++
			}
		}
		a.setStatus(fmt.Sprintf("Rescanned: %d jars found.", n), -1, false)
	}
}

func (a *app) scrollBy(dy int) {
	a.scroll[a.curTab()] += dy
	a.clampScroll(a.layout())
	invalidate(a.hwnd)
}

func (a *app) moveSel(d int) {
	n := len(a.entries())
	if n == 0 {
		return
	}
	i := a.sel[a.curTab()] + d
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	a.sel[a.curTab()] = i
	a.ensureVisible(i)
	invalidate(a.hwnd)
}

// rollTabs are the only eras Surprise Me can land on. April Fools,
// Pre-Classic, Classic, Indev and Rare Versions are deliberately excluded and
// must never come up.
var rollTabs = []int{TabRelease, TabBeta, TabAlpha, TabInfdev}

// rollPool is every version a roll may pick: the allowed eras, minus anything
// that couldn't actually be launched. A roll you can't play is a wasted roll,
// so drop-in-only versions with no jar on disk are left out.
func (a *app) rollPool(tabs []int) []*Entry {
	var out []*Entry
	for _, t := range tabs {
		for _, e := range a.L.EntriesForTab(t) {
			if e.DropInOnly && !e.Ready() {
				continue
			}
			out = append(out, e)
		}
	}
	return out
}

// surpriseMe picks a random version and reveals it. Every 11th roll is
// guaranteed to be an Alpha.
func (a *app) surpriseMe() {
	pool := a.rollPool(rollTabs)
	if len(pool) == 0 {
		a.setStatus("Nothing to roll yet — download a version first.", -1, true)
		return
	}
	a.L.Settings.RollCount++
	jackpot := a.L.Settings.RollCount%11 == 0
	var pick *Entry
	if jackpot {
		if alphas := a.rollPool([]int{TabAlpha}); len(alphas) > 0 {
			pick = alphas[rand.Intn(len(alphas))]
		}
	}
	if pick == nil {
		jackpot = false
		pick = pool[rand.Intn(len(pool))]
	}
	a.L.SaveSettings()
	a.selectEntry(pick)
	a.rareFilter = -1
	a.setView(viewList)
	msg := "Rolled " + pick.Name + " — hit LAUNCH to play it."
	if jackpot {
		msg = fmt.Sprintf("Roll %d: Alpha guaranteed — %s.", a.L.Settings.RollCount, pick.Name)
	}
	a.setStatus(msg, -1, false)
}

// lastPlayed is the version the launcher last started, if it is still in the
// catalog — what the Continue card on the home screen acts on.
func (a *app) lastPlayed() *Entry {
	if id := a.L.Settings.LastVersion; id != "" {
		return a.L.FindEntry(id)
	}
	return nil
}

func (a *app) onPlay() {
	if a.isBusy() {
		return
	}
	e := a.selectedEntry()
	if e == nil {
		return
	}
	name := strings.TrimSpace(getWindowText(a.hEdit))
	if acc := a.L.Settings.Account; acc != nil && acc.Name != "" {
		name = acc.Name
	}
	if !validUsername(name) {
		a.setStatus("Type a username first: 1-16 letters, numbers or underscores.", -1, true)
		pSetFocus.Call(a.hEdit)
		return
	}
	if !e.Ready() && e.DropInOnly {
		a.setStatus(fmt.Sprintf("Mojang doesn't host %s — put your own %s in the versions folder first.", e.Name, e.JarName()), -1, true)
		return
	}
	a.L.Settings.Username = name
	a.L.Settings.LastTab = a.tab
	a.L.Settings.LastVersion = e.ID
	a.L.SaveSettings()

	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()
	a.setStatus("Getting "+e.Name+" ready…", -1, false)
	go a.launch(e, name)
}

func (a *app) launch(e *Entry, name string) {
	res, err := a.L.Launch(e, name, func(msg string, frac float64) { a.setStatus(msg, frac, false) })
	postMessage(a.hwnd, wmAppJarsChanged, 0, 0)
	if err == nil && a.L.Settings.CloseOnLaunch {
		postMessage(a.hwnd, wmClose, 0, 0)
	}
	if err != nil {
		a.L.Logf("launch of %s failed: %v", e.ID, err)
		a.mu.Lock()
		a.busy = false
		a.pendingError = fmt.Sprintf("Couldn't start %s.\n\n%s\n\nDetails are in logs\\launcher.log.", e.Name, err.Error())
		a.mu.Unlock()
		a.setStatus("Couldn't start "+e.Name+": "+err.Error(), -1, true)
		postMessage(a.hwnd, wmAppError, 0, 0)
		return
	}
	a.mu.Lock()
	a.running = res
	a.busy = false // the game is up: the launcher is free for another launch
	a.mu.Unlock()
	logName := filepath.Base(res.LogPath)
	a.setStatus(fmt.Sprintf("%s is running (pid %d). Launch another version any time.", e.Name, res.Cmd.Process.Pid), -1, false)
	a.presenceStart(e)
	started := time.Now()
	code, werr := res.Wait()
	a.mu.Lock()
	if a.running == res {
		a.running = nil
	}
	a.mu.Unlock()
	a.addPlaytime(e.ID, time.Since(started))
	a.presenceStop()
	switch {
	case werr != nil:
		a.setStatus(fmt.Sprintf("%s: %v", e.Name, werr), -1, true)
	case code != 0 && code != 1 && code != -1:
		// 1 and -1 say nothing useful: old versions routinely exit 1 on a
		// normal quit, and -1 just means the exit code couldn't be read. Only
		// report codes that actually mean something.
		a.setStatus(fmt.Sprintf("%s closed with exit code %d. Check logs\\%s for the reason.", e.Name, code, logName), -1, true)
	default:
		a.setStatus(fmt.Sprintf("%s closed after %s.", e.Name, fmtPlaytime(time.Since(started))), -1, false)
	}
}

func (a *app) presenceStart(e *Entry) {
	if !a.L.Settings.DiscordRPC {
		return
	}
	go discord.Start(a.L.Settings.DiscordAppID, "Playing "+e.Name, Tabs[e.Tab].Name+" · LEMV Launcher")
}

func (a *app) presenceStop() {
	if !a.L.Settings.DiscordRPC {
		return
	}
	go discord.Stop()
}

// addPlaytime accumulates seconds played per version id.
func (a *app) addPlaytime(id string, d time.Duration) {
	if d < time.Second {
		return
	}
	if a.L.Settings.Playtime == nil {
		a.L.Settings.Playtime = map[string]int64{}
	}
	a.L.Settings.Playtime[id] += int64(d.Seconds())
	a.L.SaveSettings()
	invalidate(a.hwnd)
}

// fmtPlaytime renders a duration like "2h 14m" or "3m" or "41s".
func fmtPlaytime(d time.Duration) string {
	sec := int64(d.Seconds())
	switch {
	case sec >= 3600:
		return fmt.Sprintf("%dh %dm", sec/3600, (sec%3600)/60)
	case sec >= 60:
		return fmt.Sprintf("%dm", sec/60)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

// missingDownloadable lists the entries on a tab that Mojang can supply.
func (a *app) missingDownloadable(tab int) []*Entry {
	var out []*Entry
	for _, e := range a.L.EntriesForTab(tab) {
		if !e.Ready() && !e.DropInOnly {
			out = append(out, e)
		}
	}
	return out
}

func (a *app) onDownloadAll() {
	if a.isBusy() {
		return
	}
	todo := a.missingDownloadable(a.tab)
	if len(todo) == 0 {
		a.setStatus("Everything on this tab that Mojang hosts is already here.", -1, false)
		return
	}
	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()
	a.setStatus(fmt.Sprintf("Downloading %d jars from Mojang…", len(todo)), 0, false)
	go a.downloadAll(todo)
}

func (a *app) downloadAll(todo []*Entry) {
	ok, failed := 0, 0
	for i, e := range todo {
		base := float64(i) / float64(len(todo))
		span := 1 / float64(len(todo))
		prefix := fmt.Sprintf("[%d/%d] ", i+1, len(todo))
		v, err := a.L.resolveVersion(e)
		if err == nil {
			_, err = a.L.ensureClientJar(e, v, func(msg string, frac float64) {
				f := base
				if frac >= 0 {
					f = base + frac*span
				}
				a.setStatus(prefix+msg, f, false)
			})
		}
		if err != nil {
			a.L.Logf("download all: %s: %v", e.ID, err)
			failed++
		} else {
			ok++
		}
		postMessage(a.hwnd, wmAppJarsChanged, 0, 0)
	}
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
	if failed > 0 {
		a.setStatus(fmt.Sprintf("Downloaded %d jars from Mojang; %d failed — see logs\\launcher.log.", ok, failed), -1, true)
	} else {
		a.setStatus(fmt.Sprintf("Downloaded %d jars from Mojang. All set!", ok), -1, false)
	}
	postMessage(a.hwnd, wmAppJarsChanged, 0, 0)
}

func searchProc(hwnd, msg, wp, lp uintptr) uintptr {
	a := ui
	if a != nil && msg == wmKeyDown {
		switch wp {
		case vkEscape:
			sendMessage(hwnd, wmSetText, 0, uintptr(unsafe.Pointer(wstr(""))))
			pSetFocus.Call(a.hwnd)
			return 0
		case vkReturn:
			a.onPlay()
			return 0
		}
	}
	if a == nil {
		return 0
	}
	r, _, _ := pCallWindowProcW.Call(a.searchOldProc, hwnd, msg, wp, lp)
	return r
}

// positionEdits moves the child EDIT controls to match the current layout
// (the caption bar shifts everything when the theme changes).
func (a *app) positionEdits() {
	l := a.layout()
	ec := l.editCtl
	pMoveWindow.Call(a.hEdit, uintptr(ec.Left), uintptr(ec.Top), uintptr(ec.Right-ec.Left), uintptr(ec.Bottom-ec.Top), 1)
	sb := l.searchBox
	pad := a.sc(6)
	pMoveWindow.Call(a.hSearch, uintptr(sb.Left+pad), uintptr(sb.Top+a.sc(8)), uintptr(sb.Right-sb.Left-2*pad), uintptr(sb.Bottom-sb.Top-a.sc(14)), 1)
	ab := l.setArgsBox
	pMoveWindow.Call(a.hArgs, uintptr(ab.Left+pad), uintptr(ab.Top+a.sc(7)), uintptr(ab.Right-ab.Left-2*pad), uintptr(ab.Bottom-ab.Top-a.sc(13)), 1)
}

// drawSettings paints the settings page in place of the version list.

func editProc(hwnd, msg, wp, lp uintptr) uintptr {
	a := ui
	switch msg {
	case wmKeyDown:
		if wp == vkReturn {
			postMessage(a.hwnd, wmAppEnter, 0, 0)
			return 0
		}
	case wmChar:
		if wp == vkReturn {
			return 0
		}
	case wmMouseWheel:
		return sendMessage(a.hwnd, msg, wp, lp)
	}
	r, _, _ := pCallWindowProcW.Call(a.editOldProc, hwnd, msg, wp, lp)
	return r
}

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	a := ui
	switch msg {
	case wmPaint:
		a.paint()
		return 0
	case wmEraseBkgnd:
		return 1
	case wmLButtonDown:
		a.onClick(xOf(lp), yOf(lp), false)
		return 0
	case wmLButtonDblClk:
		a.onClick(xOf(lp), yOf(lp), true)
		return 0
	case wmMouseMove:
		a.onMouseMove(xOf(lp), yOf(lp))
		return 0
	case wmMouseLeave:
		a.tracking = false
		if a.hover.kind != hitNone {
			a.hover = hit{}
			invalidate(hwnd)
		}
		return 0
	case wmMouseWheel:
		delta := int(int16(uint16(wp >> 16)))
		if delta != 0 {
			a.scrollBy(-delta / 120 * a.layout().rowH)
		}
		return 0
	case wmSetCursor:
		if loword(lp) == htClient && a.hover.kind != hitNone {
			pSetCursor.Call(a.cursorHand)
			return 1
		}
	case wmKeyDown:
		switch wp {
		case vkDown:
			a.moveSel(1)
		case vkUp:
			a.moveSel(-1)
		case vkNext:
			a.moveSel(5)
		case vkPrior:
			a.moveSel(-5)
		case vkReturn:
			a.onPlay()
		}
		return 0
	case wmCommand:
		if loword(wp) == 102 && hiword(wp) == enChange {
			a.setSearch(getWindowText(a.hSearch))
		}
		if loword(wp) == 103 && hiword(wp) == enChange {
			a.L.Settings.ExtraJVMArgs = strings.Fields(getWindowText(a.hArgs))
			a.L.SaveSettings()
		}
		return 0
	case wmTimer:
		if a != nil && wp == 2 {
			a.anim += 0.14
			if a.anim >= 1 {
				a.anim = 1
				pKillTimer.Call(hwnd, 2)
			}
			invalidate(hwnd)
			return 0
		}
	case wmSize:
		if a != nil {
			a.bg.w = 0
			a.positionEdits()
			invalidate(hwnd)
		}
		return 0
	case wmGetMinMaxInfo:
		if a != nil {
			mm := (*MINMAXINFO)(unsafe.Pointer(lp))
			mm.MinTrackSize = POINT{int32(a.sc(baseW)), int32(a.sc(baseH))}
			return 0
		}
	case wmNcCalcSize:
		if a != nil && a.modern() {
			if z, _, _ := pIsZoomed.Call(hwnd); wp != 0 && z != 0 {
				fx, _, _ := pGetSystemMetrics.Call(smCxSizeFrame)
				pb, _, _ := pGetSystemMetrics.Call(smCxPaddedBorder)
				fy, _, _ := pGetSystemMetrics.Call(smCySizeFrame)
				r := (*RECT)(unsafe.Pointer(lp))
				r.Left += int32(fx + pb)
				r.Right -= int32(fx + pb)
				r.Top += int32(fy + pb)
				r.Bottom -= int32(fy + pb)
			}
			return 0
		}
	case wmNcActivate:
		if a != nil && a.modern() {
			r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, negOne)
			return r
		}
	case wmNcHitTest:
		if a != nil && a.modern() {
			pt := POINT{X: int32(int16(loword(lp))), Y: int32(int16(hiword(lp)))}
			pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
			l := a.layout()
			g := a.sc(6)
			cw := int32(l.W)
			ch := int32(l.H)
			zoomed, _, _ := pIsZoomed.Call(hwnd)
			if zoomed == 0 {
				left, right := pt.X < g, pt.X >= cw-g
				top, bottom := pt.Y < g, pt.Y >= ch-g
				switch {
				case top && left:
					return htTopLeft
				case top && right:
					return htTopRight
				case bottom && left:
					return htBottomLeft
				case bottom && right:
					return htBottomRight
				case left:
					return htLeft
				case right:
					return htRight
				case top:
					return htTop
				case bottom:
					return htBottom
				}
			}
			if ptIn(l.caption, int(pt.X), int(pt.Y)) {
				if ptIn(l.capMin, int(pt.X), int(pt.Y)) || ptIn(l.capMax, int(pt.X), int(pt.Y)) || ptIn(l.capClose, int(pt.X), int(pt.Y)) || ptIn(l.searchBox, int(pt.X), int(pt.Y)) {
					return htClient
				}
				return htCaption
			}
			return htClient
		}
	case wmCtlColorEdit:
		pSetTextColor.Call(wp, uintptr(colText))
		if lp == a.hSearch {
			pSetBkColor.Call(wp, uintptr(colSearchBg))
			return a.hbrSearch
		}
		pSetBkColor.Call(wp, uintptr(colField))
		return a.hbrField
	case wmActivate:
		if loword(wp) != 0 && a.hwnd != 0 {
			a.rescan()
		}
		return 0
	case wmAppRefresh:
		invalidate(hwnd)
		return 0
	case wmAppJarsChanged:
		a.rescan()
		return 0
	case wmAppEntriesReady:
		a.mu.Lock()
		add, dates := a.pendAdd, a.pendDates
		a.pendAdd, a.pendDates = nil, nil
		a.mu.Unlock()
		if len(add) > 0 {
			a.L.AddEntries(add, dates)
			a.rescan()
			a.setStatus(fmt.Sprintf("Loaded %d more versions from Mojang's list.", len(add)), -1, false)
			invalidate(hwnd)
		}
		return 0
	case wmAppEnter:
		a.onPlay()
		return 0
	case wmAppError:
		a.mu.Lock()
		e := a.pendingError
		a.pendingError = ""
		a.mu.Unlock()
		if e != "" {
			messageBox(hwnd, e, "LEMV Launcher", mbOK|mbIconError)
		}
		return 0
	case wmDestroy:
		a.saveState()
		pPostQuitMessage.Call(0)
		return 0
	}
	return defWindowProc(hwnd, msg, wp, lp)
}

// ---------------------------------------------------------------------------
// painting

func (a *app) paint() {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
	rc := getClientRect(a.hwnd)
	w, h := int(rc.Right), int(rc.Bottom)
	if w > 0 && h > 0 {
		mem, _, _ := pCreateCompatibleDC.Call(hdc)
		bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
		old := selectObject(mem, bmp)
		a.draw(mem, w, h)
		pBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, srcCopy)
		selectObject(mem, old)
		deleteObject(bmp)
		pDeleteDC.Call(mem)
	}
	pEndPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
}

func noise(i, j int) int {
	x := uint32(i)*374761393 + uint32(j)*668265263
	x = (x ^ (x >> 13)) * 1274126177
	return int((x ^ (x >> 16)) & 0x7fffffff)
}

// ensureBackground renders the pixel-art ground once per window size.
func (a *app) ensureBackground(hdc uintptr, w, h int, l layout) {
	if a.bg.dc != 0 && a.bg.w == w && a.bg.h == h {
		return
	}
	if a.bg.dc != 0 {
		selectObject(a.bg.dc, a.bg.old)
		deleteObject(a.bg.bmp)
		pDeleteDC.Call(a.bg.dc)
	}
	dc, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	old := selectObject(dc, bmp)

	cell := int(a.sc(cellBase))
	if cell < 4 {
		cell = 4
	}
	cols, rows := w/cell+1, h/cell+1
	grassRows := 2
	for j := 0; j < rows; j++ {
		for i := 0; i < cols; i++ {
			n := noise(i, j)
			x0, y0 := int32(i*cell), int32(j*cell)
			var c uint32
			switch {
			case j < grassRows:
				c = colGrass[n%len(colGrass)]
			case j == grassRows && n%3 == 0:
				c = colGrass[(n/3)%len(colGrass)]
			case x0 < l.sidebar.Right:
				c = colDirtSide[n%len(colDirtSide)]
			default:
				c = colDirt[n%len(colDirt)]
			}
			fillRect(dc, RECT{x0, y0, x0 + int32(cell), y0 + int32(cell)}, c)
		}
	}
	// the list sits in a darker band, like the world-select screen
	pSaveDC.Call(dc)
	pIntersectClipRect.Call(dc, uintptr(l.list.Left), uintptr(l.list.Top), uintptr(l.list.Right), uintptr(l.list.Bottom))
	for j := int(l.list.Top) / cell; j <= int(l.list.Bottom)/cell; j++ {
		for i := int(l.list.Left) / cell; i <= int(l.list.Right)/cell; i++ {
			n := noise(i+7, j+3)
			x0, y0 := int32(i*cell), int32(j*cell)
			fillRect(dc, RECT{x0, y0, x0 + int32(cell), y0 + int32(cell)}, colDirtDark[n%len(colDirtDark)])
		}
	}
	pRestoreDC.Call(dc, negOne)
	fillRect(dc, RECT{l.list.Left, l.list.Top, l.list.Right, l.list.Top + 1}, colBorder)
	fillRect(dc, RECT{l.list.Left, l.list.Bottom - 1, l.list.Right, l.list.Bottom}, colBorder)
	// a dark seam between the sidebar and the main area
	fillRect(dc, RECT{l.sidebar.Right - a.sc(2), 0, l.sidebar.Right, int32(h)}, rgb(0x10, 0x0b, 0x07))

	a.bg.dc, a.bg.bmp, a.bg.old, a.bg.w, a.bg.h = dc, bmp, old, w, h
}

func (a *app) shadowText(hdc uintptr, s string, r RECT, flags uintptr, color uint32, font uintptr, sh float64) {
	if a.modern() {
		drawText(hdc, s, r, flags|dtNoPrefix, color, font)
		return
	}
	d := a.sc(sh)
	if d < 1 {
		d = 1
	}
	drawText(hdc, s, offset(r, d, d), flags|dtNoPrefix, scaleCol(color, 0.25), font)
	drawText(hdc, s, r, flags|dtNoPrefix, color, font)
}

// drawButton paints a button in the active theme's style.
func (a *app) drawButton(hdc uintptr, r RECT, label string, active, hovered, disabled bool, font uintptr) {
	if a.modern() {
		fill, text := colBtn, colText
		switch {
		case disabled:
			fill, text = colBtnDisabled, colTextDim
		case active:
			fill, text = colAccent, colOnAccent
			if hovered {
				fill = colAccentHov
			}
		case hovered:
			fill = colBtnHover
		}
		a.fillRound(hdc, r, fill, a.sc(8))
		drawText(hdc, label, r, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, text, font)
		return
	}
	fill, text := colBtn, colText
	switch {
	case disabled:
		fill, text = colBtnDisabled, colTextDim
	case active:
		fill = colBtnActive
	case hovered:
		fill, text = colBtnHover, colYellow
	}
	fillRect(hdc, r, colBorder)
	in := inset(r, 1)
	fillRect(hdc, in, fill)
	b := a.sc(2)
	light, dark := shade(fill, 0x38), scaleCol(fill, 0.42)
	if disabled {
		light, dark = shade(fill, 0x12), scaleCol(fill, 0.6)
	}
	fillRect(hdc, RECT{in.Left, in.Top, in.Right, in.Top + b}, light)
	fillRect(hdc, RECT{in.Left, in.Top, in.Left + b, in.Bottom}, light)
	fillRect(hdc, RECT{in.Left, in.Bottom - b, in.Right, in.Bottom}, dark)
	fillRect(hdc, RECT{in.Right - b, in.Top, in.Right, in.Bottom}, dark)
	if active {
		fillRect(hdc, RECT{in.Left + b, in.Top + b, in.Left + b + a.sc(5), in.Bottom - b}, colGrassAccent)
	}
	a.shadowText(hdc, label, r, dtCenter|dtSingleLine|dtVCenter, text, font, 1.5)
}

func (a *app) draw(hdc uintptr, w, h int) {
	l := a.layout()
	fillRect(hdc, RECT{0, 0, int32(w), int32(h)}, colBgMain)
	if a.anim < 1 {
		t := a.anim
		off := int32((1 - t*(2-t)) * float64(a.sc(26))) // ease-out slide
		pSetViewportOrgEx.Call(hdc, uintptr(off), 0, 0)
		defer pSetViewportOrgEx.Call(hdc, 0, 0, 0)
	}
	switch a.view {
	case viewHome:
		a.drawHome(hdc, l)
		a.drawBottom(hdc, l)
	case viewCats:
		a.drawCats(hdc, l)
	case viewSettings:
		a.drawSettings(hdc, l)
	case viewInfo:
		a.drawInfo(hdc, l)
	case viewRareCats:
		a.drawRareCats(hdc, l)
	case viewPreCats:
		a.drawPreCats(hdc, l)
	case viewReleaseCats:
		a.drawReleaseCats(hdc, l)
	default:
		a.fillRound(hdc, inset(l.list, -a.sc(6)), colListBg, a.sc(12))
		a.drawMain(hdc, l)
		a.drawBottom(hdc, l)
	}
	a.drawTopBar(hdc, l)
	a.drawRail(hdc, l)
}

// drawRail paints the slim icon rail: home, categories, jars folder, rescan.
func (a *app) drawRail(hdc uintptr, l layout) {
	fillRect(hdc, l.rail, colCaption)
	fillRect(hdc, RECT{l.rail.Right - 1, 0, l.rail.Right, l.rail.Bottom}, colSeam)

	type railItem struct {
		r      RECT
		kind   hitKind
		label  string
		active bool
	}
	items := []railItem{
		{l.railHome, hitRailHome, "Home", a.view == viewHome},
		{l.railCats, hitRailVersions, "Versions", a.view == viewCats || a.view == viewList || a.view == viewRareCats},
		{l.railFolder, hitRailFolder, "Open jars folder", false},
		{l.railWorlds, hitRailWorlds, "Your worlds", false},
		{l.railLogs, hitRailLogs, "Logs", false},
		{l.railInfo, hitRailInfo, "About", a.view == viewInfo},
		{l.railGear, hitRailGear, "Settings", a.view == viewSettings},
		{l.railRescan, hitRailRescan, "Rescan jars", false},
	}
	var fly *railItem
	for i := range items {
		it := &items[i]
		hov := a.hover.kind == it.kind
		if hov || it.active {
			c := colCapHover
			if it.active {
				c = colRowSel
			}
			a.fillRound(hdc, it.r, c, a.sc(9))
		}
		if it.active {
			bar := RECT{0, it.r.Top + a.sc(8), a.sc(3), it.r.Bottom - a.sc(8)}
			a.fillRound(hdc, bar, colAccent, a.sc(3))
		}
		a.drawRailIcon(hdc, it.r, it.kind, hov || it.active)
		if hov {
			fly = it
		}
	}
	if fly != nil {
		w := int32(textWidth(hdc, fly.label, a.fonts.small)) + a.sc(20)
		fr := RECT{l.rail.Right + a.sc(8), fly.r.Top + a.sc(4), l.rail.Right + a.sc(8) + w, fly.r.Bottom - a.sc(4)}
		a.fillRound(hdc, fr, colField, a.sc(6))
		drawText(hdc, fly.label, fr, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.small)
	}
}

// drawRailIcon draws each icon from plain rectangles so no font glyphs are needed.
func (a *app) drawRailIcon(hdc uintptr, r RECT, kind hitKind, lit bool) {
	c := colTextDim
	if lit {
		c = colText
	}
	cx := (r.Left + r.Right) / 2
	cy := (r.Top + r.Bottom) / 2
	switch kind {
	case hitRailHome: // the LEMV block logo
		b := RECT{cx - a.sc(9), cy - a.sc(9), cx + a.sc(9), cy + a.sc(9)}
		a.fillRound(hdc, b, colAccent, a.sc(5))
		a.fillRound(hdc, inset(b, a.sc(5)), colOnAccent, a.sc(2))
	case hitRailVersions: // stacked plates
		for i := int32(0); i < 3; i++ {
			y := cy - a.sc(7) + i*a.sc(6)
			a.fillRound(hdc, RECT{cx - a.sc(9), y, cx + a.sc(9), y + a.sc(4)}, c, a.sc(2))
		}
	case hitRailFolder: // folder with a tab
		body := RECT{cx - a.sc(9), cy - a.sc(4), cx + a.sc(9), cy + a.sc(7)}
		tab := RECT{cx - a.sc(9), cy - a.sc(7), cx - a.sc(1), cy - a.sc(2)}
		a.fillRound(hdc, tab, c, a.sc(2))
		a.fillRound(hdc, body, c, a.sc(2))
	case hitRailWorlds: // a tiny "world": rounded square with a dot grid
		b := RECT{cx - a.sc(9), cy - a.sc(9), cx + a.sc(9), cy + a.sc(9)}
		a.fillRound(hdc, b, c, a.sc(4))
		for i := int32(0); i < 2; i++ {
			for j := int32(0); j < 2; j++ {
				d := RECT{cx - a.sc(5) + i*a.sc(6), cy - a.sc(5) + j*a.sc(6), cx - a.sc(1) + i*a.sc(6), cy - a.sc(1) + j*a.sc(6)}
				fillRect(hdc, d, colCaption)
			}
		}
	case hitRailLogs: // a document with lines
		b := RECT{cx - a.sc(7), cy - a.sc(9), cx + a.sc(7), cy + a.sc(9)}
		a.fillRound(hdc, b, c, a.sc(3))
		for i := int32(0); i < 3; i++ {
			ln := RECT{cx - a.sc(4), cy - a.sc(5) + i*a.sc(5), cx + a.sc(4), cy - a.sc(3) + i*a.sc(5)}
			fillRect(hdc, ln, colCaption)
		}
	case hitRailInfo: // circle-i
		ring := RECT{cx - a.sc(9), cy - a.sc(9), cx + a.sc(9), cy + a.sc(9)}
		a.fillRound(hdc, ring, c, a.sc(9))
		a.fillRound(hdc, inset(ring, a.sc(2)), colCaption, a.sc(7))
		fillRect(hdc, RECT{cx - a.sc(1), cy - a.sc(5), cx + a.sc(1), cy - a.sc(3)}, c)
		fillRect(hdc, RECT{cx - a.sc(1), cy - a.sc(1), cx + a.sc(1), cy + a.sc(5)}, c)
	case hitRailGear: // a proper gear: ring, hub hole, eight teeth
		outer := RECT{cx - a.sc(7), cy - a.sc(7), cx + a.sc(7), cy + a.sc(7)}
		a.fillRound(hdc, outer, c, a.sc(7))
		a.fillRound(hdc, inset(outer, a.sc(3)), colCaption, a.sc(4))
		t := a.sc(2)
		// cardinal teeth
		fillRect(hdc, RECT{cx - t, cy - a.sc(10), cx + t, cy - a.sc(6)}, c)
		fillRect(hdc, RECT{cx - t, cy + a.sc(6), cx + t, cy + a.sc(10)}, c)
		fillRect(hdc, RECT{cx - a.sc(10), cy - t, cx - a.sc(6), cy + t}, c)
		fillRect(hdc, RECT{cx + a.sc(6), cy - t, cx + a.sc(10), cy + t}, c)
		// diagonal teeth
		d := a.sc(6)
		for _, q := range [][2]int32{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			fillRect(hdc, RECT{cx + q[0]*d - t, cy + q[1]*d - t, cx + q[0]*d + t, cy + q[1]*d + t}, c)
		}
	case hitRailRescan: // a themed java cup: body, handle, accent steam
		body := RECT{cx - a.sc(8), cy - a.sc(2), cx + a.sc(5), cy + a.sc(9)}
		a.fillRound(hdc, body, c, a.sc(4))
		handle := RECT{cx + a.sc(4), cy - a.sc(1), cx + a.sc(10), cy + a.sc(6)}
		a.fillRound(hdc, handle, c, a.sc(6))
		a.fillRound(hdc, inset(handle, a.sc(2)), colCaption, a.sc(3))
		// steam wisps
		fillRect(hdc, RECT{cx - a.sc(5), cy - a.sc(9), cx - a.sc(3), cy - a.sc(4)}, colAccent)
		fillRect(hdc, RECT{cx - a.sc(1), cy - a.sc(11), cx + a.sc(1), cy - a.sc(4)}, colAccent)
	}
}

// drawTopBar paints the caption/top bar: crumb, search, window buttons.
func (a *app) drawTopBar(hdc uintptr, l layout) {
	fillRect(hdc, l.topBar, colCaption)
	fillRect(hdc, RECT{l.topBar.Left, l.topBar.Bottom - 1, l.topBar.Right, l.topBar.Bottom}, colSeam)

	crumb := "Home"
	switch a.view {
	case viewCats:
		crumb = "Versions"
	case viewSettings:
		crumb = "Settings"
	case viewInfo:
		crumb = "About"
	case viewRareCats:
		crumb = "Versions  /  Rare Versions"
	case viewPreCats:
		crumb = "Versions  /  Pre-Release"
	case viewReleaseCats:
		crumb = "Versions  /  Release"
	case viewList:
		crumb = "Versions  /  " + Tabs[a.tab].Name
		if a.query != "" {
			crumb = "Search"
		}
	}
	drawText(hdc, crumb, l.crumb, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colTextSoft, a.fonts.btn)

	if e := a.selectedEntry(); e != nil {
		a.fillRound(hdc, l.verPill, colRowSel, a.sc(8))
		dot := RECT{l.verPill.Left + a.sc(10), (l.verPill.Top+l.verPill.Bottom)/2 - a.sc(4), l.verPill.Left + a.sc(18), (l.verPill.Top+l.verPill.Bottom)/2 + a.sc(4)}
		col := colBtn
		if e.Ready() {
			col = colAccent
		}
		a.fillRound(hdc, dot, col, a.sc(4))
		drawText(hdc, e.Name, RECT{dot.Right + a.sc(8), l.verPill.Top, l.verPill.Right - a.sc(8), l.verPill.Bottom}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colText, a.fonts.small)
	}
	a.fillRound(hdc, l.searchBox, colSearchBg, a.sc(8))
	frameRound(hdc, l.searchBox, colFieldBorder, a.sc(8))

	cap := func(r RECT, glyph string, kind hitKind) {
		hov := a.hover.kind == kind
		if hov {
			c := colCapHover
			if kind == hitCapClose {
				c = colCloseHov
			}
			fillRect(hdc, r, c)
		}
		tc := colTextSoft
		if hov {
			tc = colText
		}
		drawText(hdc, glyph, r, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, tc, a.fonts.body)
	}
	cap(l.capMin, "\u2013", hitCapMin)
	cap(l.capMax, "\u25a1", hitCapMax)
	cap(l.capClose, "\u00d7", hitCapClose)
}

// drawSettings paints the settings page.
func (a *app) drawSettings(hdc uintptr, l layout) {
	top := l.topBar.Bottom
	drawText(hdc, "Settings", RECT{l.setMem.Left, top + a.sc(14), int32(l.W), top + a.sc(46)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.h1)
	drawText(hdc, "LEMV Launcher "+launcherVersion+"  \u00b7  more options live in launcher-settings.json", RECT{l.setMem.Left, top + a.sc(46), int32(l.W) - a.sc(30), top + a.sc(66)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colTextDim, a.fonts.small)

	label := func(r RECT, name, sub string) {
		drawText(hdc, name, RECT{r.Left, r.Top + a.sc(4), r.Right - a.sc(200), r.Top + a.sc(28)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.body)
		if sub != "" {
			drawText(hdc, sub, RECT{r.Left, r.Top + a.sc(28), r.Right - a.sc(200), r.Bottom}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colTextDim, a.fonts.small)
		}
	}
	toggle := func(r RECT, on, hov bool) {
		track := colBtn
		if hov {
			track = colBtnHover
		}
		if on {
			track = colAccent
			if hov {
				track = colAccentHov
			}
		}
		rad := r.Bottom - r.Top
		a.fillRound(hdc, r, track, rad)
		kw := rad - a.sc(6)
		kx := r.Left + a.sc(3)
		if on {
			kx = r.Right - a.sc(3) - kw
		}
		knob := colText
		if on {
			knob = colOnAccent
		}
		a.fillRound(hdc, RECT{kx, r.Top + a.sc(3), kx + kw, r.Bottom - a.sc(3)}, knob, kw)
	}

	label(l.setMem, "Memory for the game", "How much RAM Minecraft may use")
	a.drawButton(hdc, l.setMemDec, "\u2013", false, a.hover.kind == hitMemMinus, a.L.Settings.MaxMemoryMB <= 1024, a.fonts.body)
	drawText(hdc, fmt.Sprintf("%d MB", a.L.Settings.MaxMemoryMB), l.setMemVal, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.body)
	a.drawButton(hdc, l.setMemInc, "+", false, a.hover.kind == hitMemPlus, a.L.Settings.MaxMemoryMB >= 16384, a.fonts.body)

	label(l.setClose, "Close launcher when the game starts", "")
	toggle(l.setClsTgl, a.L.Settings.CloseOnLaunch, a.hover.kind == hitTglClose)

	label(l.setName, "Remember my username", "Prefill the name from your last launch")
	toggle(l.setNameTgl, a.L.Settings.RememberName, a.hover.kind == hitTglName)

	label(l.setAnim, "Animations", "Slide transitions between menus")
	toggle(l.setAnimTgl, a.L.Settings.Animations, a.hover.kind == hitTglAnim)

	label(l.setImp, "Auto-import from Downloads", "Move matching jars you download into the versions folder")
	toggle(l.setImpTgl, a.L.Settings.AutoImport, a.hover.kind == hitTglImport)

	label(l.setRPC, "Discord Rich Presence", "Show your friends which version you're playing")
	toggle(l.setRPCTgl, a.L.Settings.DiscordRPC, a.hover.kind == hitTglRPC)

	if msaEnabled {
		accState := "Offline mode (no account)"
		accBtn := "Sign in"
		if acc := a.L.Settings.Account; acc != nil {
			accState = "Signed in as " + acc.Name
			accBtn = "Sign out"
		}
		label(l.setAcc, "Minecraft account", accState)
		a.drawButton(hdc, l.setAccBtn, accBtn, a.L.Settings.Account != nil, a.hover.kind == hitAccount, false, a.fonts.small)
	}

	drawText(hdc, "Extra JVM arguments", RECT{l.setArgs.Left, l.setArgs.Top, l.setArgs.Right, l.setArgs.Top + a.sc(24)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.body)
	a.fillRound(hdc, l.setArgsBox, colField, a.sc(8))
}

// drawInfo is the About page.
func (a *app) drawInfo(hdc uintptr, l layout) {
	top := l.topBar.Bottom
	mx := l.rail.Right + a.sc(28)
	W := int32(l.W)
	b := RECT{mx, top + a.sc(24), mx + a.sc(40), top + a.sc(64)}
	a.fillRound(hdc, b, colAccent, a.sc(10))
	a.fillRound(hdc, inset(b, a.sc(11)), colBgMain, a.sc(4))
	drawText(hdc, "LEMV Launcher", RECT{b.Right + a.sc(14), top + a.sc(24), W, top + a.sc(52)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.h1)
	drawText(hdc, "Literally Every Minecraft Version  \u00b7  "+launcherVersion, RECT{b.Right + a.sc(14), top + a.sc(52), W, top + a.sc(72)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colTextDim, a.fonts.small)

	dn, tot := 0, 0
	for t := 0; t < TabCount; t++ {
		n, x := a.L.ReadyCount(t)
		dn += n
		tot += x
	}
	para := func(y int32, txt string, col uint32) int32 {
		r := RECT{mx, y, W - a.sc(40), y + a.sc(64)}
		drawText(hdc, txt, r, dtLeft|dtWordBreak|dtNoPrefix, col, a.fonts.body)
		return y + a.sc(52)
	}
	y := top + a.sc(96)
	y = para(y, fmt.Sprintf("One portable launcher for %d base versions of Minecraft: Java Edition across nine eras \u2014 %d of them on your disk right now. Offline play, per-version worlds, zero setup.", tot, dn), colTextSoft)
	y = para(y, "Versions Mojang still hosts download straight from Mojang. Rare and archived builds link to the Omniarchive preservation community \u2014 the launcher points at their vault and imports what you download, but never redistributes game files itself.", colTextSoft)
	y = para(y, "Sub to GrookyGamez. Not affiliated with Mojang, Microsoft or Oracle. Minecraft is a trademark of Mojang Synergies AB \u2014 buy the game, it's worth it.", colTextDim)
	drawText(hdc, "Data folder: "+a.L.Root, RECT{mx, int32(l.H) - a.sc(40), W - a.sc(40), int32(l.H) - a.sc(16)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colTextDim, a.fonts.small)
}

// drawHome is the landing view: a quiet wordmark; the play bar does the work.
func (a *app) drawHome(hdc uintptr, l layout) {
	W := int32(l.W)
	midY := (l.topBar.Bottom + l.playBar.Top) / 2
	// soft decorative blocks
	deco := func(x, y, sz int32, col uint32) {
		a.fillRound(hdc, RECT{x, y, x + sz, y + sz}, col, sz/4)
	}
	deco(l.rail.Right+a.sc(80), midY-a.sc(110), a.sc(46), colSidebarBg)
	deco(W-a.sc(160), midY-a.sc(140), a.sc(64), colSidebarBg)
	deco(W-a.sc(230), midY+a.sc(70), a.sc(34), colRowHover)
	deco(l.rail.Right+a.sc(70), midY+a.sc(150), a.sc(26), rgb(0x1f, 0x3a, 0x2c))

	// the block mark, larger, above the wordmark
	b := RECT{(l.rail.Right+W)/2 - a.sc(26), midY - a.sc(86), (l.rail.Right+W)/2 + a.sc(26), midY - a.sc(34)}
	a.fillRound(hdc, b, colAccent, a.sc(12))
	a.fillRound(hdc, inset(b, a.sc(14)), colOnAccent, a.sc(5))
	drawText(hdc, "LEMV", RECT{l.rail.Right, midY - a.sc(24), W, midY + a.sc(24)}, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.title)

	a.mu.Lock()
	st, isErr := a.status, a.statusErr
	a.mu.Unlock()
	col := colTextDim
	if isErr {
		col = colRed
	}
	drawText(hdc, st, RECT{l.rail.Right + a.sc(30), l.playBar.Top - a.sc(34), W - a.sc(30), l.playBar.Top - a.sc(8)}, dtCenter|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, col, a.fonts.small)

	// three useful cards: continue, library, surprise
	card := func(i int, title, big, sub string) RECT {
		r := l.homeCards[i]
		hov := a.hover.kind == hitHomeCard && a.hover.index == i
		fill := colRowHover
		if hov {
			fill = colRowSel
		}
		a.fillRound(hdc, r, fill, a.sc(10))
		if hov {
			bar := RECT{r.Left, r.Top + a.sc(10), r.Left + a.sc(3), r.Bottom - a.sc(10)}
			a.fillRound(hdc, bar, colAccent, a.sc(3))
		}
		drawText(hdc, title, RECT{r.Left + a.sc(16), r.Top + a.sc(10), r.Right - a.sc(12), r.Top + a.sc(28)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colTextDim, a.fonts.small)
		drawText(hdc, big, RECT{r.Left + a.sc(16), r.Top + a.sc(28), r.Right - a.sc(12), r.Top + a.sc(58)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colText, a.fonts.btn)
		drawText(hdc, sub, RECT{r.Left + a.sc(16), r.Bottom - a.sc(32), r.Right - a.sc(12), r.Bottom - a.sc(10)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colTextDim, a.fonts.small)
		return r
	}
	cont, contSub := "\u2014", "Nothing played yet"
	if e := a.lastPlayed(); e != nil {
		cont, contSub = e.Name, "Play it again"
	}
	card(0, "CONTINUE", cont, contSub)
	dn, tot := 0, 0
	for t := 0; t < TabCount; t++ {
		n, x := a.L.ReadyCount(t)
		dn += n
		tot += x
	}
	r1 := card(1, "VERSIONS", fmt.Sprintf("%d downloaded", dn), fmt.Sprintf("%d of %d versions downloaded", dn, tot))
	pbr := RECT{r1.Left + a.sc(16), r1.Bottom - a.sc(40), r1.Right - a.sc(16), r1.Bottom - a.sc(36)}
	a.fillRound(hdc, pbr, colBtn, a.sc(2))
	if tot > 0 {
		fw := int32(float64(pbr.Right-pbr.Left) * float64(dn) / float64(tot))
		if fw > 0 {
			a.fillRound(hdc, RECT{pbr.Left, pbr.Top, pbr.Left + fw, pbr.Bottom}, colAccent, a.sc(2))
		}
	}
	sub := "Release, Beta, Alpha and Infdev"
	if n := a.L.Settings.RollCount % 11; n == 10 {
		sub = "Next roll is a guaranteed Alpha"
	}
	card(2, "SURPRISE ME", "Roll the dice", sub)
}

// drawRareCats is the Rare Versions submenu: recovered / still lost / oddities.
func (a *app) drawRareCats(hdc uintptr, l layout) {
	top := l.topBar.Bottom
	a.drawButton(hdc, l.btnBack, "<", false, a.hover.kind == hitBack, false, a.fonts.body)
	drawText(hdc, "Rare Versions", RECT{l.btnBack.Right + a.sc(10), top + a.sc(14), int32(l.W), top + a.sc(46)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.h1)
	counts := make([]int, len(RareKinds))
	for _, e := range a.L.EntriesForTab(TabLost) {
		if e.Rare >= 0 && e.Rare < len(RareKinds) {
			counts[e.Rare]++
		}
	}
	for i := range RareKinds {
		r := l.catCards[i]
		hov := a.hover.kind == hitRareCard && a.hover.index == i
		fill := colRowHover
		if hov {
			fill = colRowSel
		}
		a.fillRound(hdc, r, fill, a.sc(10))
		if hov {
			bar := RECT{r.Left, r.Top + a.sc(10), r.Left + a.sc(3), r.Bottom - a.sc(10)}
			a.fillRound(hdc, bar, colAccent, a.sc(3))
		}
		drawText(hdc, RareKinds[i].Name, RECT{r.Left + a.sc(16), r.Top + a.sc(14), r.Right - a.sc(12), r.Top + a.sc(40)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.btn)
		cc := colTextDim
		if counts[i] > 0 {
			cc = colAccent
		}
		unit := "builds"
		if counts[i] == 1 {
			unit = "build"
		}
		drawText(hdc, fmt.Sprintf("%d %s", counts[i], unit), RECT{r.Left + a.sc(16), r.Top + a.sc(42), r.Right - a.sc(12), r.Top + a.sc(64)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, cc, a.fonts.small)
		drawText(hdc, RareKinds[i].Desc, RECT{r.Left + a.sc(16), r.Bottom - a.sc(38), r.Right - a.sc(12), r.Bottom - a.sc(12)}, dtLeft|dtWordBreak|dtNoPrefix, colTextDim, a.fonts.small)
	}
}

// drawCats is the category menu: one card per era.
// tabPreRelease is a pseudo-tab: the Pre-Release card is a group, not a real
// catalog tab, so it carries no Tabs[] entry of its own.
const (
	tabPreRelease = -1
	// tabReleaseGroup is the Release card: like Pre-Release it's a group, not
	// a tab, holding Base Updates and Minor Updates.
	tabReleaseGroup = -2
)

// releaseTabs sit behind the Release card.
var releaseTabs = []int{TabRelease, TabMinor}

// preTabs are the eras behind the Pre-Release card, newest first — the same
// reverse-chronological order the rest of the launcher uses.
var preTabs = []int{TabBeta, TabAlpha, TabInfdev, TabIndev, TabClassic, TabPreClassic}

// topTabs is the Versions grid. The six pre-release eras sit one level down
// under Pre-Release instead of crowding the top level.
var topTabs = []int{tabReleaseGroup, tabPreRelease, TabAprilFools, TabLost}

// isPreTab reports whether a tab lives behind the Pre-Release card.
func isPreTab(t int) bool {
	for _, x := range preTabs {
		if x == t {
			return true
		}
	}
	return false
}

// groupCount totals the jars across a set of tabs behind one card.
func (a *app) groupCount(tabs []int) (have, total int) {
	for _, t := range tabs {
		n, x := a.L.ReadyCount(t)
		have += n
		total += x
	}
	return
}

func (a *app) preReleaseCount() (have, total int) { return a.groupCount(preTabs) }

// drawCatCard paints one card in either category grid.
func (a *app) drawCatCard(hdc uintptr, r RECT, hov bool, name, desc string, have, total int) {
	fill := colRowHover
	if hov {
		fill = colRowSel
	}
	a.fillRound(hdc, r, fill, a.sc(10))
	if hov {
		bar := RECT{r.Left, r.Top + a.sc(10), r.Left + a.sc(3), r.Bottom - a.sc(10)}
		a.fillRound(hdc, bar, colAccent, a.sc(3))
	}
	drawText(hdc, name, RECT{r.Left + a.sc(16), r.Top + a.sc(12), r.Right - a.sc(12), r.Top + a.sc(38)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colText, a.fonts.btn)
	cc := colTextDim
	if have == total && total > 0 {
		cc = colAccent
	}
	drawText(hdc, fmt.Sprintf("%d / %d downloaded", have, total), RECT{r.Left + a.sc(16), r.Top + a.sc(38), r.Right - a.sc(12), r.Top + a.sc(58)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, cc, a.fonts.small)
	drawText(hdc, desc, RECT{r.Left + a.sc(16), r.Bottom - a.sc(42), r.Right - a.sc(12), r.Bottom - a.sc(10)}, dtLeft|dtWordBreak|dtNoPrefix, colTextDim, a.fonts.small)
}

// drawReleaseCats is the second level behind the Release card.
func (a *app) drawReleaseCats(hdc uintptr, l layout) {
	top := l.topBar.Bottom
	a.drawButton(hdc, l.btnBack, "<", false, a.hover.kind == hitBack, false, a.fonts.body)
	drawText(hdc, "Release", RECT{l.btnBack.Right + a.sc(10), top + a.sc(14), int32(l.W), top + a.sc(46)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.h1)
	names := []string{"Base Updates", "Minor Updates"}
	descs := []string{
		"One jar per major version \u00b7 1.0 through 26.2",
		"Every point release in between \u00b7 1.0.1, 1.2.5, 1.21.4 and the rest",
	}
	for i, t := range releaseTabs {
		n, total := a.L.ReadyCount(t)
		a.drawCatCard(hdc, l.catCards[i], a.hover.kind == hitReleaseCard && a.hover.index == i, names[i], descs[i], n, total)
	}
}

// drawPreCats is the second level behind the Pre-Release card.
func (a *app) drawPreCats(hdc uintptr, l layout) {
	top := l.topBar.Bottom
	a.drawButton(hdc, l.btnBack, "<", false, a.hover.kind == hitBack, false, a.fonts.body)
	drawText(hdc, "Pre-Release", RECT{l.btnBack.Right + a.sc(10), top + a.sc(14), int32(l.W), top + a.sc(46)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.h1)
	for i, t := range preTabs {
		n, total := a.L.ReadyCount(t)
		a.drawCatCard(hdc, l.catCards[i], a.hover.kind == hitPreCard && a.hover.index == i, Tabs[t].Name, Tabs[t].Desc, n, total)
	}
}

func (a *app) drawCats(hdc uintptr, l layout) {
	top := l.topBar.Bottom
	drawText(hdc, "Choose a category", RECT{l.catCards[0].Left, top + a.sc(14), int32(l.W), top + a.sc(46)}, dtLeft|dtSingleLine|dtVCenter|dtNoPrefix, colText, a.fonts.h1)
	for i, t := range topTabs {
		hov := a.hover.kind == hitCatCard && a.hover.index == i
		if t == tabPreRelease {
			have, total := a.preReleaseCount()
			a.drawCatCard(hdc, l.catCards[i], hov, "Pre-Release", "Every build before 1.0 \u00b7 Beta, Alpha, Infdev, Indev, Classic and Pre-Classic", have, total)
			continue
		}
		if t == tabReleaseGroup {
			have, total := a.groupCount(releaseTabs)
			a.drawCatCard(hdc, l.catCards[i], hov, "Release", "Base versions and every point release, 1.0 onwards", have, total)
			continue
		}
		n, total := a.L.ReadyCount(t)
		a.drawCatCard(hdc, l.catCards[i], hov, Tabs[t].Name, Tabs[t].Desc, n, total)
	}
}

func (a *app) drawMain(hdc uintptr, l layout) {
	es := a.entries()
	n, total := a.L.ReadyCount(a.tab)
	name := Tabs[a.tab].Name
	descText := Tabs[a.tab].Desc
	if a.tab == TabLost && a.rareFilter >= 0 && a.query == "" {
		name = RareKinds[a.rareFilter].Name
		descText = RareKinds[a.rareFilter].Desc
	}
	if a.query != "" {
		name = "Search"
		if len(a.results) == 1 {
			descText = "1 version matches \"" + a.query + "\""
		} else {
			descText = fmt.Sprintf("%d versions match \"%s\"", len(a.results), a.query)
		}
	}
	a.drawButton(hdc, l.btnBack, "<", false, a.hover.kind == hitBack, false, a.fonts.body)
	a.shadowText(hdc, name, l.header, dtLeft|dtSingleLine|dtVCenter, colText, a.fonts.h1, 2)
	tw := textWidth(hdc, name, a.fonts.h1)
	desc := RECT{l.header.Left + int32(tw) + a.sc(16), l.header.Top + a.sc(3), l.header.Right, l.header.Bottom}
	a.shadowText(hdc, descText, desc, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis, colTextSoft, a.fonts.body, 1)
	hint := fmt.Sprintf("%d of %d on disk   ·   missing jars download straight from Mojang   ·   custom jars go in %s", n, total, a.L.VersionsDir)
	hintCol := colTextDim
	if a.modern() {
		hint = fmt.Sprintf("%d of %d downloaded  ·  missing jars come straight from Mojang", n, total)
		a.mu.Lock()
		st, isErr := a.status, a.statusErr
		a.mu.Unlock()
		if st != "" && st != idleStatus {
			hint = st
			hintCol = colTextSoft
			if isErr {
				hintCol = colRed
			}
		}
	}
	a.shadowText(hdc, hint, l.hint, dtLeft|dtSingleLine|dtVCenter|dtPathEllipsis, hintCol, a.fonts.small, 1)
	dlDisabled := a.isBusy() || a.query != "" || len(a.missingDownloadable(a.tab)) == 0
	a.drawButton(hdc, l.btnDlAll, "Download all", false, a.hover.kind == hitDownloadAll && !dlDisabled, dlDisabled, a.fonts.btn)
	if n := len(a.missingArchived()); n > 0 {
		a.drawButton(hdc, l.btnCopy, fmt.Sprintf("Copy %d links", n), false, a.hover.kind == hitCopyLinks, false, a.fonts.btn)
	}

	a.clampScroll(l)
	pSaveDC.Call(hdc)
	pIntersectClipRect.Call(hdc, uintptr(l.list.Left), uintptr(l.list.Top+1), uintptr(l.list.Right), uintptr(l.list.Bottom-1))
	y := int32(int(l.list.Top) - a.scroll[a.curTab()])
	pad := a.sc(10)
	for i, e := range es {
		r := RECT{l.list.Left + pad, y + a.sc(4), l.list.Right - pad - a.sc(14), y + int32(l.rowH) - a.sc(4)}
		y += int32(l.rowH)
		if r.Bottom < l.list.Top || r.Top > l.list.Bottom {
			continue
		}
		selected := i == a.sel[a.curTab()]
		hovered := a.hover.kind == hitRow && a.hover.index == i
		if a.modern() {
			switch {
			case selected:
				a.fillRound(hdc, r, colRowSel, a.sc(8))
				bar := RECT{r.Left, r.Top + a.sc(8), r.Left + a.sc(3), r.Bottom - a.sc(8)}
				a.fillRound(hdc, bar, colAccent, a.sc(3))
			case hovered:
				a.fillRound(hdc, r, colRowHover, a.sc(8))
			}
		} else {
			switch {
			case selected:
				fillRect(hdc, r, colRowSelBorder)
				fillRect(hdc, inset(r, 1), colBorder)
				fillRect(hdc, inset(r, 2), colRowSel)
			case hovered:
				fillRect(hdc, r, colRowHover)
			}
		}
		tx := r.Left + a.sc(14)
		statusW := a.sc(250)
		nameCol := colText
		if !e.Ready() {
			nameCol = colTextSoft
		}
		a.shadowText(hdc, e.Name, RECT{tx, r.Top + a.sc(5), r.Right - statusW, r.Top + a.sc(29)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis, nameCol, a.fonts.mono, 1.5)
		a.shadowText(hdc, e.Note, RECT{tx, r.Top + a.sc(29), r.Right - statusW, r.Bottom - a.sc(4)}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis, colTextDim, a.fonts.small, 1)

		sr := RECT{r.Right - statusW, r.Top + a.sc(5), r.Right - a.sc(14), r.Top + a.sc(29)}
		fr := RECT{sr.Left, r.Top + a.sc(29), sr.Right, r.Bottom - a.sc(4)}
		if a.modern() {
			pill := func(right int32, label string, bg, fg uint32) int32 {
				w := int32(textWidth(hdc, label, a.fonts.small)) + a.sc(22)
				pr := RECT{right - w, r.Top + a.sc(7), right, r.Top + a.sc(27)}
				a.fillRound(hdc, pr, bg, pr.Bottom-pr.Top)
				drawText(hdc, label, pr, dtCenter|dtSingleLine|dtVCenter|dtNoPrefix, fg, a.fonts.small)
				return pr.Left
			}
			sub := func(right int32, txt string) {
				drawText(hdc, txt, RECT{r.Left, r.Bottom - a.sc(25), right, r.Bottom - a.sc(5)}, dtRight|dtSingleLine|dtVCenter|dtEndEllipsis, colTextDim, a.fonts.monoSmall)
			}
			rp := r.Right - a.sc(12)
			switch {
			case e.Ready():
				pill(rp, "READY", colAccent, colOnAccent)
				sub(rp, filepath.Base(e.JarPath))
			case e.DropInOnly:
				endX := rp
				if br, ok := a.getJarButtonRect(l, i); ok {
					a.drawButton(hdc, br, "Download", false, a.hover.kind == hitGetJar && a.hover.index == i, false, a.fonts.small)
					endX = br.Left - a.sc(10)
				}
				pill(endX, "NOT ON MOJANG", rgb(0x35, 0x37, 0x3d), colTextSoft)
				sub(endX, "needs "+e.JarName())
			default:
				pill(rp, "ON MOJANG", rgb(0x3e, 0x37, 0x22), colYellow)
				sub(rp, "Play downloads it")
			}
		} else {
			switch {
			case e.Ready():
				a.shadowText(hdc, "READY", sr, dtRight|dtSingleLine|dtVCenter, colGreen, a.fonts.btn, 1)
				a.shadowText(hdc, filepath.Base(e.JarPath), fr, dtRight|dtSingleLine|dtVCenter|dtEndEllipsis, colTextDim, a.fonts.monoSmall, 1)
			case e.DropInOnly:
				if br, ok := a.getJarButtonRect(l, i); ok {
					// label the status to the left of the button, then draw the button
					lbl := RECT{sr.Left, sr.Top, br.Left - a.sc(10), sr.Bottom}
					a.shadowText(hdc, "NOT ON MOJANG", lbl, dtRight|dtSingleLine|dtVCenter, colTextDim, a.fonts.btn, 1)
					flbl := RECT{fr.Left, fr.Top, br.Left - a.sc(10), fr.Bottom}
					a.shadowText(hdc, "needs "+e.JarName(), flbl, dtRight|dtSingleLine|dtVCenter|dtEndEllipsis, colTextDim, a.fonts.monoSmall, 1)
					a.drawButton(hdc, br, "Download", false, a.hover.kind == hitGetJar && a.hover.index == i, false, a.fonts.small)
				} else {
					a.shadowText(hdc, "NOT ON MOJANG", sr, dtRight|dtSingleLine|dtVCenter, colTextDim, a.fonts.btn, 1)
					a.shadowText(hdc, "needs your "+e.JarName(), fr, dtRight|dtSingleLine|dtVCenter|dtEndEllipsis, colTextDim, a.fonts.monoSmall, 1)
				}
			default:
				a.shadowText(hdc, "ON MOJANG", sr, dtRight|dtSingleLine|dtVCenter, colYellow, a.fonts.btn, 1)
				a.shadowText(hdc, "Play downloads it for you", fr, dtRight|dtSingleLine|dtVCenter|dtEndEllipsis, colTextDim, a.fonts.monoSmall, 1)
			}
		}
	}
	pRestoreDC.Call(hdc, negOne)

	content := len(es) * l.rowH
	visible := int(l.list.Bottom - l.list.Top)
	if content > visible {
		track := RECT{l.list.Right - a.sc(10), l.list.Top + a.sc(4), l.list.Right - a.sc(4), l.list.Bottom - a.sc(4)}
		fillRect(hdc, track, colBorder)
		th := int(track.Bottom - track.Top)
		thumbH := th * visible / content
		if thumbH < int(a.sc(24)) {
			thumbH = int(a.sc(24))
		}
		thumbY := int(track.Top) + (th-thumbH)*a.scroll[a.curTab()]/(content-visible)
		thumb := RECT{track.Left, int32(thumbY), track.Right, int32(thumbY + thumbH)}
		fillRect(hdc, thumb, colScrollThumb)
		fillRect(hdc, RECT{thumb.Left, thumb.Top, thumb.Right - 1, thumb.Bottom - 1}, rgb(0xc0, 0xc0, 0xc0))
		fillRect(hdc, RECT{thumb.Left + 1, thumb.Top + 1, thumb.Right - 1, thumb.Bottom - 1}, colScrollThumb)
	}
}

func (a *app) drawBottom(hdc uintptr, l layout) {
	busy := a.isBusy()
	a.mu.Lock()
	st, isErr, prog := a.status, a.statusErr, a.progress
	a.mu.Unlock()

	if a.modern() {
		fillRect(hdc, l.playBar, colSidebarBg)
		fillRect(hdc, RECT{l.playBar.Left, l.playBar.Top, l.playBar.Right, l.playBar.Top + 1}, colSeam)
		if busy && prog >= 0 {
			fw := int32(math.Round(float64(l.playBar.Right-l.playBar.Left) * math.Min(prog, 1)))
			fillRect(hdc, RECT{l.playBar.Left, l.playBar.Top, l.playBar.Left + fw, l.playBar.Top + a.sc(3)}, colAccent)
		}
		label := "Pick a version to play"
		if e := a.selectedEntry(); e != nil {
			label = e.Name
			if e.Note != "" {
				label += " · " + e.Note
			}
		}
		drawText(hdc, label, RECT{l.verInfo.Left, l.playBar.Top, l.verInfo.Right, l.playBar.Bottom}, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis|dtNoPrefix, colTextDim, a.fonts.small)
		_ = st
		_ = isErr
		a.fillRound(hdc, l.editFrame, colField, a.sc(8))
		none := a.selectedEntry() == nil
		a.drawButton(hdc, l.play, "LAUNCH", !busy && !none, a.hover.kind == hitPlay && !busy && !none, busy || none, a.fonts.play)
		return
	}

	a.shadowText(hdc, "Username", l.userLabel, dtLeft|dtSingleLine|dtVCenter, colTextSoft, a.fonts.body, 1)
	fillRect(hdc, l.editFrame, colFieldBorder)
	fillRect(hdc, inset(l.editFrame, 1), colField)

	a.drawButton(hdc, l.play, "PLAY", false, a.hover.kind == hitPlay && !busy, busy, a.fonts.play)

	col := colTextSoft
	if isErr {
		col = colRed
	}
	a.shadowText(hdc, st, l.status, dtLeft|dtSingleLine|dtVCenter|dtEndEllipsis, col, a.fonts.body, 1)
	if busy && prog >= 0 {
		fillRect(hdc, l.progress, colBorder)
		in := inset(l.progress, 1)
		fillRect(hdc, in, colProgressBg)
		fw := int32(math.Round(float64(in.Right-in.Left) * math.Min(prog, 1)))
		fillRect(hdc, RECT{in.Left, in.Top, in.Left + fw, in.Bottom}, colGrassAccent)
		fillRect(hdc, RECT{in.Left, in.Top, in.Left + fw, in.Top + 1}, shade(colGrassAccent, 0x30))
	}
}

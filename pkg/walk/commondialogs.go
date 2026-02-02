// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

import (
	"fmt"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/miu200521358/win"
)

type FileDialog struct {
	Title          string
	FilePath       string
	FilePaths      []string
	InitialDirPath string
	Filter         string
	FilterIndex    int
	Flags          uint32
	ShowReadOnlyCB bool
	// BrowseRootDirPath はフォルダ選択ダイアログのルートを指定する。
	BrowseRootDirPath string
	// BrowseInitialSelectionPath はフォルダ選択ダイアログの初期選択パスを指定する。
	BrowseInitialSelectionPath string
}

func (dlg *FileDialog) show(owner Form, fun func(ofn *win.OPENFILENAME) bool, flags uint32) (accepted bool, err error) {
	ofn := new(win.OPENFILENAME)

	ofn.LStructSize = uint32(unsafe.Sizeof(*ofn))
	if owner != nil {
		ofn.HwndOwner = owner.Handle()
	}

	filter := make([]uint16, len(dlg.Filter)+2)
	copy(filter, syscall.StringToUTF16(dlg.Filter))
	// Replace '|' with the expected '\0'.
	for i, c := range filter {
		if byte(c) == '|' {
			filter[i] = uint16(0)
		}
	}
	ofn.LpstrFilter = &filter[0]
	ofn.NFilterIndex = uint32(dlg.FilterIndex)

	ofn.LpstrInitialDir = syscall.StringToUTF16Ptr(dlg.InitialDirPath)
	ofn.LpstrTitle = syscall.StringToUTF16Ptr(dlg.Title)
	ofn.Flags = win.OFN_FILEMUSTEXIST | flags | dlg.Flags

	if !dlg.ShowReadOnlyCB {
		ofn.Flags |= win.OFN_HIDEREADONLY
	}

	var fileBuf []uint16
	if flags&win.OFN_ALLOWMULTISELECT > 0 {
		fileBuf = make([]uint16, 65536)
	} else {
		fileBuf = make([]uint16, 1024)
		copy(fileBuf, syscall.StringToUTF16(dlg.FilePath))
	}
	ofn.LpstrFile = &fileBuf[0]
	ofn.NMaxFile = uint32(len(fileBuf))

	if !fun(ofn) {
		errno := win.CommDlgExtendedError()
		if errno != 0 {
			err = newError(fmt.Sprintf("Error %d", errno))
		}
		return
	}

	dlg.FilterIndex = int(ofn.NFilterIndex)

	if flags&win.OFN_ALLOWMULTISELECT > 0 {
		split := func() [][]uint16 {
			var parts [][]uint16

			from := 0
			for i, c := range fileBuf {
				if c == 0 {
					if i == from {
						return parts
					}

					parts = append(parts, fileBuf[from:i])
					from = i + 1
				}
			}

			return parts
		}

		parts := split()

		if len(parts) == 1 {
			dlg.FilePaths = []string{syscall.UTF16ToString(parts[0])}
		} else {
			dirPath := syscall.UTF16ToString(parts[0])
			dlg.FilePaths = make([]string, len(parts)-1)

			for i, fp := range parts[1:] {
				dlg.FilePaths[i] = filepath.Join(dirPath, syscall.UTF16ToString(fp))
			}
		}
	} else {
		dlg.FilePath = syscall.UTF16ToString(fileBuf)
	}

	accepted = true

	return
}

func (dlg *FileDialog) ShowOpen(owner Form) (accepted bool, err error) {
	return dlg.show(owner, win.GetOpenFileName, win.OFN_NOCHANGEDIR)
}

func (dlg *FileDialog) ShowOpenMultiple(owner Form) (accepted bool, err error) {
	return dlg.show(owner, win.GetOpenFileName, win.OFN_ALLOWMULTISELECT|win.OFN_EXPLORER|win.OFN_NOCHANGEDIR)
}

func (dlg *FileDialog) ShowSave(owner Form) (accepted bool, err error) {
	return dlg.show(owner, win.GetSaveFileName, win.OFN_NOCHANGEDIR)
}

func pathFromPIDL(pidl uintptr) (string, error) {
	var path [win.MAX_PATH]uint16
	if !win.SHGetPathFromIDList(pidl, &path[0]) {
		return "", newError("SHGetPathFromIDList failed")
	}

	return syscall.UTF16ToString(path[:]), nil
}

// browseFolderContext はフォルダ選択ダイアログの初期化情報を保持する。
type browseFolderContext struct {
	initialSelection *uint16
}

// We use this callback to disable the OK button in case of "invalid" selections.
func browseFolderCallback(hwnd win.HWND, msg uint32, lp, wp uintptr) uintptr {
	const (
		BFFM_INITIALIZED = 1
		BFFM_SELCHANGED  = 2
	)
	if msg == BFFM_INITIALIZED {
		ctx := (*browseFolderContext)(unsafe.Pointer(wp))
		if ctx != nil && ctx.initialSelection != nil {
			const BFFM_SETSELECTION = win.WM_USER + 102
			const BFFM_SETEXPANDED = win.WM_USER + 106
			// 初期選択の展開を優先し、念のため選択だけの通知も送る。
			win.SendMessage(hwnd, BFFM_SETEXPANDED, 1, uintptr(unsafe.Pointer(ctx.initialSelection)))
			win.SendMessage(hwnd, BFFM_SETSELECTION, 1, uintptr(unsafe.Pointer(ctx.initialSelection)))
		}
		return 0
	}
	if msg == BFFM_SELCHANGED {
		_, err := pathFromPIDL(lp)
		var enabled uintptr
		if err == nil {
			enabled = 1
		}

		const BFFM_ENABLEOK = win.WM_USER + 101

		win.SendMessage(hwnd, BFFM_ENABLEOK, 0, enabled)
	}

	return 0
}

var browseFolderCallbackPtr uintptr

func init() {
	AppendToWalkInit(func() {
		browseFolderCallbackPtr = syscall.NewCallback(browseFolderCallback)
	})
}

func (dlg *FileDialog) ShowBrowseFolder(owner Form) (accepted bool, err error) {
	// Calling OleInitialize (or similar) is required for BIF_NEWDIALOGSTYLE.
	if hr := win.OleInitialize(); hr != win.S_OK && hr != win.S_FALSE {
		return false, newError(fmt.Sprint("OleInitialize Error: ", hr))
	}
	defer win.OleUninitialize()

	var ownerHwnd win.HWND
	if owner != nil {
		ownerHwnd = owner.Handle()
	}

	rootPath := ""
	if dlg.BrowseRootDirPath != "" {
		rootPath = dlg.BrowseRootDirPath
	} else if dlg.BrowseInitialSelectionPath == "" && dlg.InitialDirPath != "" {
		// 互換性維持: 新フィールド未使用時は従来通りルート扱いにする。
		rootPath = dlg.InitialDirPath
	}

	selectionPath := dlg.BrowseInitialSelectionPath
	ctx := &browseFolderContext{}
	var selectionBuf []uint16
	if selectionPath != "" {
		selectionBuf = syscall.StringToUTF16(selectionPath)
		if len(selectionBuf) > 0 {
			ctx.initialSelection = &selectionBuf[0]
		}
	}

	const BIF_NEWDIALOGSTYLE = 0x00000040

	bi := win.BROWSEINFO{
		HwndOwner: ownerHwnd,
		LpszTitle: syscall.StringToUTF16Ptr(dlg.Title),
		UlFlags:   BIF_NEWDIALOGSTYLE,
		Lpfn:      browseFolderCallbackPtr,
		LParam:    uintptr(unsafe.Pointer(ctx)),
	}

	if rootPath != "" {
		// 初期ルートは最大長のバッファを使って安全に渡す。
		var buf [win.MAX_PATH]uint16
		copy(buf[:], syscall.StringToUTF16(rootPath))
		win.SHParseDisplayName(&buf[0], 0, &bi.PidlRoot, 0, nil)
	}

	pidl := win.SHBrowseForFolder(&bi)
	runtime.KeepAlive(ctx)
	runtime.KeepAlive(selectionBuf)
	if pidl == 0 {
		return false, nil
	}
	defer win.CoTaskMemFree(pidl)

	dlg.FilePath, err = pathFromPIDL(pidl)
	accepted = dlg.FilePath != ""
	return
}

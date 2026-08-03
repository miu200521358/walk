// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

import (
	"unsafe"

	"github.com/miu200521358/win"
)

// customColorSlots is the number of "custom colors" slots the Windows color
// dialog keeps at the bottom of its expanded view.
const customColorSlots = 16

// ColorDialog is the standard Windows color picker.
type ColorDialog struct {
	// Color is the initially selected color, and holds the chosen color after
	// Show returns true.
	Color Color

	// CustomColors are the sixteen user-defined slots at the bottom of the
	// dialog. The dialog writes the user's edits back into this array, so
	// passing the same array on every call is what makes those colors survive
	// from one invocation to the next. Leave it nil to discard them.
	CustomColors *[customColorSlots]Color

	// ShowCustom opens the dialog with the "define custom colors" pane already
	// expanded, instead of requiring a click to reach it.
	ShowCustom bool
}

// Show displays the dialog and reports whether the user accepted a color.
// A plain cancel returns false with a nil error; only a dialog failure yields
// an error, so the two cases stay distinguishable.
func (dlg *ColorDialog) Show(owner Form) (accepted bool, err error) {
	cc := new(win.CHOOSECOLOR)
	cc.LStructSize = uint32(unsafe.Sizeof(*cc))
	if owner != nil {
		cc.HwndOwner = owner.Handle()
	}

	cc.RgbResult = win.COLORREF(dlg.Color)
	cc.Flags = win.CC_RGBINIT | win.CC_ANYCOLOR
	if dlg.ShowCustom {
		cc.Flags |= win.CC_FULLOPEN
	}

	// The dialog requires a writable array here even when the caller does not
	// care about the custom slots, so a local one always backs the call.
	var custom [customColorSlots]win.COLORREF
	if dlg.CustomColors != nil {
		for i, color := range dlg.CustomColors {
			custom[i] = win.COLORREF(color)
		}
	} else {
		for i := range custom {
			custom[i] = win.COLORREF(ColorWhite)
		}
	}
	cc.LpCustColors = &custom

	if !win.ChooseColor(cc) {
		if code := win.CommDlgExtendedError(); code != 0 {
			return false, newError("ChooseColor failed")
		}
		return false, nil
	}

	if dlg.CustomColors != nil {
		for i, color := range custom {
			dlg.CustomColors[i] = Color(color)
		}
	}

	dlg.Color = Color(cc.RgbResult)

	return true, nil
}

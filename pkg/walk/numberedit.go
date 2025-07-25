// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/miu200521358/win"
)

const numberEditWindowClass = `\o/ Walk_NumberEdit_Class \o/`

func init() {
	AppendToWalkInit(func() {
		MustRegisterWindowClass(numberEditWindowClass)
	})
}

// NumberEdit is a widget that is suited to edit numeric values.
type NumberEdit struct {
	WidgetBase
	edit                     *numberLineEdit
	hWndUpDown               win.HWND
	maxValueChangedPublisher EventPublisher
	minValueChangedPublisher EventPublisher
	prefixChangedPublisher   EventPublisher
	suffixChangedPublisher   EventPublisher
}

// NewNumberEdit returns a new NumberEdit widget as child of parent.
func NewNumberEdit(parent Container) (*NumberEdit, error) {
	ne := new(NumberEdit)

	if err := InitWidget(
		ne,
		parent,
		numberEditWindowClass,
		win.WS_VISIBLE,
		win.WS_EX_CONTROLPARENT); err != nil {
		return nil, err
	}

	var succeeded bool
	defer func() {
		if !succeeded {
			ne.Dispose()
		}
	}()

	var err error
	if ne.edit, err = newNumberLineEdit(ne); err != nil {
		return nil, err
	}

	ne.edit.applyFont(ne.Font())

	ne.SetRange(-math.MaxFloat64, math.MaxFloat64)

	if err = ne.SetValue(0); err != nil {
		return nil, err
	}

	ne.GraphicsEffects().Add(InteractionEffect)
	ne.GraphicsEffects().Add(FocusEffect)

	ne.MustRegisterProperty("MaxValue", NewProperty(
		func() interface{} {
			return ne.MaxValue()
		},
		func(v interface{}) error {
			return ne.SetRange(ne.MinValue(), assertFloat64Or(v, 0.0))
		},
		ne.minValueChangedPublisher.Event()))

	ne.MustRegisterProperty("MinValue", NewProperty(
		func() interface{} {
			return ne.MinValue()
		},
		func(v interface{}) error {
			return ne.SetRange(assertFloat64Or(v, 0.0), ne.MaxValue())
		},
		ne.maxValueChangedPublisher.Event()))

	ne.MustRegisterProperty("Prefix", NewProperty(
		func() interface{} {
			return ne.Prefix()
		},
		func(v interface{}) error {
			return ne.SetPrefix(assertStringOr(v, ""))
		},
		ne.prefixChangedPublisher.Event()))

	ne.MustRegisterProperty("ReadOnly", NewProperty(
		func() interface{} {
			return ne.ReadOnly()
		},
		func(v interface{}) error {
			return ne.SetReadOnly(v.(bool))
		},
		ne.edit.readOnlyChangedPublisher.Event()))

	ne.MustRegisterProperty("Suffix", NewProperty(
		func() interface{} {
			return ne.Suffix()
		},
		func(v interface{}) error {
			return ne.SetSuffix(assertStringOr(v, ""))
		},
		ne.suffixChangedPublisher.Event()))

	ne.MustRegisterProperty("Value", NewProperty(
		func() interface{} {
			return ne.Value()
		},
		func(v interface{}) error {
			return ne.SetValue(assertFloat64Or(v, ne.MinValue()))
		},
		ne.edit.valueChangedPublisher.Event()))

	succeeded = true

	return ne, nil
}

func (ne *NumberEdit) applyEnabled(enabled bool) {
	ne.WidgetBase.applyEnabled(enabled)

	if ne.edit == nil {
		return
	}

	ne.edit.applyEnabled(enabled)
}

func (ne *NumberEdit) applyFont(font *Font) {
	ne.WidgetBase.applyFont(font)

	if ne.edit == nil {
		return
	}

	ne.edit.applyFont(font)
}

func (ne *NumberEdit) SetEnabled(enabled bool) {
	ne.WidgetBase.SetEnabled(enabled)

	if !enabled {
		bg, err := NewSystemColorBrush(SysColorInactiveBorder)
		if err != nil {
			return
		}
		ne.SetBackground(bg)
	} else {
		bg, err := NewSolidColorBrush(RGB(255, 255, 255))
		if err != nil {
			return
		}
		ne.SetBackground(bg)
	}
}

// Decimals returns the number of decimal places in the NumberEdit.
func (ne *NumberEdit) Decimals() int {
	return ne.edit.decimals
}

// SetDecimals sets the number of decimal places in the NumberEdit.
func (ne *NumberEdit) SetDecimals(decimals int) error {
	if decimals < 0 || decimals > 8 {
		return newError("decimals must >= 0 && <= 8")
	}

	ne.edit.decimals = decimals

	return ne.SetValue(ne.edit.value)
}

// Prefix returns the text that appears in the NumberEdit before the number.
func (ne *NumberEdit) Prefix() string {
	return syscall.UTF16ToString(ne.edit.prefix)
}

// SetPrefix sets the text that appears in the NumberEdit before the number.
func (ne *NumberEdit) SetPrefix(prefix string) error {
	if prefix == ne.Prefix() {
		return nil
	}

	p, err := syscall.UTF16FromString(prefix)
	if err != nil {
		return err
	}

	old := ne.edit.prefix
	ne.edit.prefix = p[:len(p)-1]

	if err := ne.edit.setTextFromValue(ne.edit.value); err != nil {
		ne.edit.prefix = old
		return err
	}

	ne.prefixChangedPublisher.Publish()

	return nil
}

// PrefixChanged returns the event that is published when the prefix changed.
func (ne *NumberEdit) PrefixChanged() *Event {
	return ne.prefixChangedPublisher.Event()
}

// Suffix returns the text that appears in the NumberEdit after the number.
func (ne *NumberEdit) Suffix() string {
	return syscall.UTF16ToString(ne.edit.suffix)
}

// SetSuffix sets the text that appears in the NumberEdit after the number.
func (ne *NumberEdit) SetSuffix(suffix string) error {
	if suffix == ne.Suffix() {
		return nil
	}

	s, err := syscall.UTF16FromString(suffix)
	if err != nil {
		return err
	}

	old := ne.edit.suffix
	ne.edit.suffix = s[:len(s)-1]

	if err := ne.edit.setTextFromValue(ne.edit.value); err != nil {
		ne.edit.suffix = old
		return err
	}

	ne.suffixChangedPublisher.Publish()

	return nil
}

// SuffixChanged returns the event that is published when the suffix changed.
func (ne *NumberEdit) SuffixChanged() *Event {
	return ne.suffixChangedPublisher.Event()
}

// Increment returns the amount by which the NumberEdit increments or decrements
// its value, when the user presses the KeyDown or KeyUp keys, or when the mouse
// wheel is rotated.
func (ne *NumberEdit) Increment() float64 {
	return ne.edit.increment
}

// SetIncrement sets the amount by which the NumberEdit increments or decrements
// its value, when the user presses the KeyDown or KeyUp keys, or when the mouse
// wheel is rotated.
func (ne *NumberEdit) SetIncrement(increment float64) error {
	ne.edit.increment = increment

	return nil
}

// MinValue returns the minimum value the NumberEdit will accept.
func (ne *NumberEdit) MinValue() float64 {
	return ne.edit.minValue
}

// MinValue returns the maximum value the NumberEdit will accept.
func (ne *NumberEdit) MaxValue() float64 {
	return ne.edit.maxValue
}

// SetRange sets the minimum and maximum values the NumberEdit will accept.
//
// If the current value is out of this range, it will be adjusted.
func (ne *NumberEdit) SetRange(min, max float64) error {
	if min > max {
		return newError(fmt.Sprintf("invalid range - min: %f, max: %f", min, max))
	}

	minChanged := min != ne.edit.minValue
	maxChanged := max != ne.edit.maxValue

	ne.edit.minValue = min
	ne.edit.maxValue = max
	if min != max {
		if ne.edit.value < min {
			if err := ne.edit.setValue(min, true); err != nil {
				return err
			}
		} else if ne.edit.value > max {
			if err := ne.edit.setValue(max, true); err != nil {
				return err
			}
		}
	}

	if minChanged {
		ne.minValueChangedPublisher.Publish()
	}
	if maxChanged {
		ne.maxValueChangedPublisher.Publish()
	}

	return nil
}

// Value returns the value of the NumberEdit.
func (ne *NumberEdit) Value() float64 {
	return ne.edit.value
}

// SetValue sets the value of the NumberEdit.
func (ne *NumberEdit) SetValue(value float64) error {
	if ne.edit.minValue != ne.edit.maxValue &&
		(value < ne.edit.minValue || value > ne.edit.maxValue) {

		return newError("value out of range")
	}

	return ne.edit.setValue(value, true)
}

// SetValue sets the value of the NumberEdit.
func (ne *NumberEdit) ChangeValue(value float64) error {
	if ne.edit.minValue != ne.edit.maxValue &&
		(value < ne.edit.minValue || value > ne.edit.maxValue) {

		return newError("value out of range")
	}

	return ne.edit.changeValue(value, true)
}

// ValueChanged returns an Event that can be used to track changes to Value.
func (ne *NumberEdit) ValueChanged() *Event {
	return ne.edit.valueChangedPublisher.Event()
}

// SetFocus sets the keyboard input focus to the NumberEdit.
func (ne *NumberEdit) SetFocus() error {
	if win.SetFocus(ne.edit.hWnd) == 0 {
		return lastError("SetFocus")
	}

	return nil
}

// TextSelection returns the range of the current text selection of the
// NumberEdit.
func (ne *NumberEdit) TextSelection() (start, end int) {
	return ne.edit.TextSelection()
}

// SetTextSelection sets the range of the current text selection of the
// NumberEdit.
func (ne *NumberEdit) SetTextSelection(start, end int) {
	ne.edit.SetTextSelection(start, end)
}

// ReadOnly returns whether the NumberEdit is in read-only mode.
func (ne *NumberEdit) ReadOnly() bool {
	return ne.edit.ReadOnly()
}

// SetReadOnly sets whether the NumberEdit is in read-only mode.
func (ne *NumberEdit) SetReadOnly(readOnly bool) error {
	if readOnly != ne.ReadOnly() {
		ne.invalidateBorderInParent()
	}

	return ne.edit.SetReadOnly(readOnly)
}

// SpinButtonsVisible returns whether the NumberEdit appears with spin buttons.
func (ne *NumberEdit) SpinButtonsVisible() bool {
	return ne.hWndUpDown != 0
}

// SetSpinButtonsVisible sets whether the NumberEdit appears with spin buttons.
func (ne *NumberEdit) SetSpinButtonsVisible(visible bool) error {
	if visible == ne.SpinButtonsVisible() {
		return nil
	}

	if visible {
		ne.hWndUpDown = win.CreateWindowEx(
			0,
			syscall.StringToUTF16Ptr("msctls_updown32"),
			nil,
			win.WS_CHILD|win.WS_VISIBLE|win.UDS_ALIGNRIGHT|win.UDS_ARROWKEYS|win.UDS_HOTTRACK,
			0,
			0,
			16,
			20,
			ne.hWnd,
			0,
			0,
			nil)
		if ne.hWndUpDown == 0 {
			return lastError("CreateWindowEx")
		}

		win.SendMessage(ne.hWndUpDown, win.UDM_SETBUDDY, uintptr(ne.edit.hWnd), 0)
	} else {
		if !win.DestroyWindow(ne.hWndUpDown) {
			return lastError("DestroyWindow")
		}

		ne.hWndUpDown = 0
	}

	return nil
}

// Background returns the background Brush of the NumberEdit.
//
// By default this is nil.
func (ne *NumberEdit) Background() Brush {
	return ne.edit.Background()
}

// SetBackground sets the background Brush of the NumberEdit.
func (ne *NumberEdit) SetBackground(bg Brush) {
	ne.edit.SetBackground(bg)
}

// TextColor returns the Color used to draw the text of the NumberEdit.
func (ne *NumberEdit) TextColor() Color {
	return ne.edit.TextColor()
}

// TextColor sets the Color used to draw the text of the NumberEdit.
func (ne *NumberEdit) SetTextColor(c Color) {
	ne.edit.SetTextColor(c)
}

func (*NumberEdit) NeedsWmSize() bool {
	return true
}

// WndProc is the window procedure of the NumberEdit.
//
// When implementing your own WndProc to add or modify behavior, call the
// WndProc of the embedded NumberEdit for messages you don't handle yourself.
func (ne *NumberEdit) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_NOTIFY:
		switch ((*win.NMHDR)(unsafe.Pointer(lParam))).Code {
		case win.UDN_DELTAPOS:
			nmud := (*win.NMUPDOWN)(unsafe.Pointer(lParam))
			ne.edit.incrementValue(-float64(nmud.IDelta) * ne.edit.increment)
		}

	case win.WM_CTLCOLOREDIT, win.WM_CTLCOLORSTATIC:
		if hBrush := ne.handleWMCTLCOLOR(wParam, lParam); hBrush != 0 {
			return hBrush
		}

	case win.WM_WINDOWPOSCHANGED:
		wp := (*win.WINDOWPOS)(unsafe.Pointer(lParam))

		if wp.Flags&win.SWP_NOSIZE != 0 {
			break
		}

		if ne.edit == nil {
			break
		}

		cb := ne.ClientBoundsPixels()
		if err := ne.edit.SetBoundsPixels(cb); err != nil {
			break
		}

		if ne.hWndUpDown != 0 {
			win.SendMessage(ne.hWndUpDown, win.UDM_SETBUDDY, uintptr(ne.edit.hWnd), 0)
		}
	}

	return ne.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}

func (ne *NumberEdit) CreateLayoutItem(ctx *LayoutContext) LayoutItem {
	return &numberEditLayoutItem{
		idealSize: ne.dialogBaseUnitsToPixels(Size{50, 12}),
		minSize:   ne.dialogBaseUnitsToPixels(Size{20, 12}),
	}
}

type numberEditLayoutItem struct {
	LayoutItemBase
	idealSize Size // in native pixels
	minSize   Size // in native pixels
}

func (*numberEditLayoutItem) LayoutFlags() LayoutFlags {
	return ShrinkableHorz | GrowableHorz
}

func (li *numberEditLayoutItem) IdealSize() Size {
	return li.idealSize
}

func (li *numberEditLayoutItem) MinSize() Size {
	return li.minSize
}

type numberLineEdit struct {
	*LineEdit
	buf                   *bytes.Buffer
	prefix                []uint16
	suffix                []uint16
	value                 float64
	minValue              float64
	maxValue              float64
	increment             float64
	decimals              int
	valueChangedPublisher EventPublisher
	inEditMode            bool
}

func newNumberLineEdit(parent Widget) (*numberLineEdit, error) {
	nle := &numberLineEdit{
		buf:       new(bytes.Buffer),
		increment: 1,
	}

	var err error
	if nle.LineEdit, err = newLineEdit(parent, win.WS_EX_CLIENTEDGE); err != nil {
		return nil, err
	}

	succeeded := false
	defer func() {
		if !succeeded {
			nle.Dispose()
		}
	}()

	if err := nle.LineEdit.setAndClearStyleBits(win.ES_RIGHT, win.ES_LEFT|win.ES_CENTER); err != nil {
		return nil, err
	}

	if err := InitWrapperWindow(nle); err != nil {
		return nil, err
	}

	succeeded = true

	return nle, nil
}

func (nle *numberLineEdit) TextColor() Color {
	return nle.LineEdit.TextColor()
}

func (nle *numberLineEdit) SetTextColor(c Color) {
	nle.LineEdit.SetTextColor(c)
}

func (nle *numberLineEdit) setValue(value float64, setText bool) error {
	if setText {
		if err := nle.setTextFromValue(value); err != nil {
			return err
		}
	}

	if value == nle.value {
		return nil
	}

	nle.value = value

	nle.valueChangedPublisher.Publish()

	return nil
}

func (nle *numberLineEdit) changeValue(value float64, setText bool) error {
	if setText {
		if err := nle.setTextFromValue(value); err != nil {
			return err
		}
	}

	if value == nle.value {
		return nil
	}

	nle.value = value

	return nil
}

func (nle *numberLineEdit) setTextFromValue(value float64) error {
	nle.buf.Reset()

	nle.buf.WriteString(syscall.UTF16ToString(nle.prefix))

	if nle.decimals > 0 {
		nle.buf.WriteString(FormatFloatGrouped(value, nle.decimals))
	} else {
		nle.buf.WriteString(FormatFloat(value, nle.decimals))
	}

	nle.buf.WriteString(syscall.UTF16ToString(nle.suffix))

	return nle.SetText(nle.buf.String())
}

func (nle *numberLineEdit) endEdit() error {
	if err := nle.setTextFromValue(nle.value); err != nil {
		return err
	}

	nle.inEditMode = false

	return nil
}

func (nle *numberLineEdit) processChar(text []uint16, start, end int, key Key, char uint16) {
	hadSelection := start != end

	if !nle.inEditMode {
		var groupSepsBeforeStart int
		if nle.decimals > 0 {
			groupSepsBeforeStart = uint16CountUint16(text[:start], groupSepUint16)
		}

		if hadSelection {
			text = append(text[:start], text[end:]...)
		}

		if nle.decimals > 0 {
			text = uint16RemoveUint16(text, groupSepUint16)
			start -= groupSepsBeforeStart
		}

		nle.inEditMode = true
	} else {
		if hadSelection {
			text = append(text[:start], text[end:]...)
		}
	}

	end = start

	switch key {
	case KeyBack:
		if !hadSelection && start > 0 {
			start -= 1
			text = append(text[:start], text[start+1:]...)
		}

	case KeyDelete:
		if !hadSelection && start < len(text) {
			text = append(text[:start], text[start+1:]...)
		}

	default:
		t := make([]uint16, len(text[:start]), len(text)+1)
		copy(t, text[:start])
		t = append(t, char)
		text = append(t, text[start:]...)
		start += 1
	}

	nle.buf.Reset()

	str := syscall.UTF16ToString(text)

	nle.buf.WriteString(syscall.UTF16ToString(nle.prefix))
	nle.buf.WriteString(str)
	nle.buf.WriteString(syscall.UTF16ToString(nle.suffix))

	nle.SetText(nle.buf.String())

	start += len(nle.prefix)
	nle.SetTextSelection(start, start)

	nle.tryUpdateValue(false)
}

func (nle *numberLineEdit) tryUpdateValue(setText bool) bool {
	t := nle.textUTF16()
	t = t[len(nle.prefix) : len(t)-len(nle.suffix)]

	text := strings.Replace(syscall.UTF16ToString(t), decimalSepS, ".", 1)

	switch text {
	case "", ".":
		text = "0"
	}

	if value, err := strconv.ParseFloat(text, 64); err == nil {
		if nle.minValue == nle.maxValue || value >= nle.minValue && value <= nle.maxValue {
			return nle.setValue(value, setText) == nil
		}
	}

	return false
}

func (nle *numberLineEdit) selectNumber() {
	nle.SetTextSelection(len(nle.prefix), len(nle.textUTF16())-len(nle.suffix))
}

func (nle *numberLineEdit) textUTF16() []uint16 {
	textLength := nle.SendMessage(win.WM_GETTEXTLENGTH, 0, 0)
	buf := make([]uint16, textLength+1)
	nle.SendMessage(win.WM_GETTEXT, uintptr(textLength+1), uintptr(unsafe.Pointer(&buf[0])))

	return buf[:len(buf)-1]
}

func (nle *numberLineEdit) incrementValue(delta float64) {
	value := nle.value + delta

	if nle.minValue != nle.maxValue {
		if value < nle.minValue {
			value = nle.minValue
		} else if value > nle.maxValue {
			value = nle.maxValue
		}
	}

	nle.setValue(value, true)
	nle.selectNumber()
}

func (nle *numberLineEdit) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_CHAR:
		if nle.ReadOnly() {
			break
		}

		if AltDown() {
			return 0
		}

		if ControlDown() {
			if wParam == 1 {
				// Ctrl+A
				return 0
			}
			break
		}

		char := uint16(wParam)

		text := nle.textUTF16()
		text = text[len(nle.prefix) : len(text)-len(nle.suffix)]
		start, end := nle.TextSelection()
		start -= len(nle.prefix)
		end -= len(nle.prefix)

		if Key(wParam) == KeyBack {
			nle.processChar(text, start, end, KeyBack, 0)
			return 0
		}

		switch char {
		case uint16('0'), uint16('1'), uint16('2'), uint16('3'), uint16('4'), uint16('5'), uint16('6'), uint16('7'), uint16('8'), uint16('9'):
			if start == end && nle.decimals > 0 {
				if i := uint16IndexUint16(text, decimalSepUint16); i > -1 && i < len(text)-nle.decimals && start > i {
					return 0
				}
			}

			// 必ず最後に追加する
			start = len(text)
			end = len(text)

			nle.processChar(text, start, end, 0, char)
			return 0

		case uint16('-'):
			if nle.minValue != nle.maxValue && nle.minValue >= 0 {
				return 0
			}

			if start > 0 || uint16ContainsUint16(text, uint16('-')) && end == 0 {
				return 0
			}

			nle.processChar(text, start, end, 0, char)
			return 0

		case decimalSepUint16:
			if nle.decimals == 0 {
				return 0
			}

			if start == 0 && end == 0 && len(text) > 0 && text[0] == '-' {
				return 0
			}

			if end < len(text)-nle.decimals {
				return 0
			}

			if i := uint16IndexUint16(text, decimalSepUint16); i > -1 && i <= start || i > end {
				return 0
			}

			nle.processChar(text, start, end, 0, char)
			return 0

		default:
			return 0
		}

	case win.WM_KEYDOWN:
		switch Key(wParam) {
		case KeyA:
			if ControlDown() {
				nle.selectNumber()
				return 0
			}

		case KeyDelete:
			if nle.ReadOnly() {
				break
			}

			text := nle.textUTF16()
			text = text[len(nle.prefix) : len(text)-len(nle.suffix)]
			start, end := nle.TextSelection()
			start -= len(nle.prefix)
			end -= len(nle.prefix)

			nle.processChar(text, start, end, KeyDelete, 0)
			return 0

		case KeyDown:
			if nle.ReadOnly() || nle.increment <= 0 {
				return 0
			}

			nle.incrementValue(-nle.increment)
			return 0

		case KeyEnd:
			start, end := nle.TextSelection()
			end = len(nle.textUTF16()) - len(nle.suffix)
			if !ShiftDown() {
				start = end
			}
			nle.SetTextSelection(start, end)
			return 0

		case KeyHome:
			start, end := nle.TextSelection()
			start = len(nle.prefix)
			if !ShiftDown() {
				end = start
			}
			nle.SetTextSelection(start, end)
			return 0

		case KeyLeft:
			var pos win.POINT
			win.GetCaretPos(&pos)

			lParam := uintptr(win.MAKELONG(uint16(pos.X), uint16(pos.Y)))
			i := int(win.LOWORD(uint32(nle.SendMessage(win.EM_CHARFROMPOS, 0, lParam))))

			if min := len(nle.prefix); i <= min {
				if !ShiftDown() {
					nle.SetTextSelection(min, min)
				}
				return 0
			}

		case KeyReturn:
			if nle.ReadOnly() {
				break
			}

			if nle.inEditMode {
				nle.endEdit()
				nle.selectNumber()
				return 0
			}

		case KeyRight:
			var pos win.POINT
			win.GetCaretPos(&pos)

			lParam := uintptr(win.MAKELONG(uint16(pos.X), uint16(pos.Y)))
			i := int(win.LOWORD(uint32(nle.SendMessage(win.EM_CHARFROMPOS, 0, lParam))))

			if max := len(nle.textUTF16()) - len(nle.suffix); i >= max {
				if !ShiftDown() {
					nle.SetTextSelection(max, max)
				}
				return 0
			}

		case KeyUp:
			if nle.ReadOnly() || nle.increment <= 0 {
				return 0
			}

			nle.incrementValue(nle.increment)
			return 0
		}

	case win.WM_GETDLGCODE:
		if !nle.inEditMode {
			if form := ancestor(nle); form != nil {
				if dlg, ok := form.(dialogish); ok {
					if dlg.DefaultButton() != nil {
						// If the NumberEdit lives in a Dialog that has a
						// DefaultButton, we won't swallow the return key.
						break
					}
				}
			}
		}

		if wParam == win.VK_RETURN {
			return win.DLGC_WANTALLKEYS
		}

	case win.WM_KILLFOCUS:
		nle.onFocusChanged()
		nle.endEdit()

	case win.WM_LBUTTONDOWN:
		i := int(win.LOWORD(uint32(nle.SendMessage(win.EM_CHARFROMPOS, 0, lParam))))

		if min := len(nle.prefix); i < min {
			nle.SetFocus()
			nle.SetTextSelection(min, min)
			return 0
		}
		if max := len(nle.textUTF16()) - len(nle.suffix); i > max {
			nle.SetFocus()
			nle.SetTextSelection(max, max)
			return 0
		}

	case win.WM_LBUTTONDBLCLK:
		nle.selectNumber()
		return 0

	case win.WM_MOUSEMOVE:
		i := int(win.LOWORD(uint32(nle.SendMessage(win.EM_CHARFROMPOS, 0, lParam))))

		if min := len(nle.prefix); i < min {
			return 0
		}
		if max := len(nle.textUTF16()) - len(nle.suffix); i > max {
			return 0
		}

	case win.WM_MOUSEWHEEL:
		if nle.ReadOnly() || nle.increment <= 0 {
			break
		}

		delta := float64(int16(win.HIWORD(uint32(wParam))))
		nle.incrementValue(delta / 120 * nle.increment)
		return 0

	case win.WM_PASTE:
		if nle.ReadOnly() {
			break
		}

		ret := nle.LineEdit.WndProc(hwnd, msg, wParam, lParam)
		if !nle.tryUpdateValue(true) {
			nle.setTextFromValue(nle.value)
		}
		nle.selectNumber()
		return ret

	case win.WM_SETFOCUS:
		nle.onFocusChanged()
		nle.selectNumber()

	case win.EM_SETSEL:
		start := int(wParam)
		end := int(lParam)
		adjusted := false
		if min := len(nle.prefix); start < min {
			start = min
			adjusted = true
		}
		if max := len(nle.textUTF16()) - len(nle.suffix); end < 0 || end > max {
			end = max
			adjusted = true
		}

		if adjusted {
			nle.SetTextSelection(start, end)
			return 0
		}
	}

	return nle.LineEdit.WndProc(hwnd, msg, wParam, lParam)
}

func (nle *numberLineEdit) onFocusChanged() {
	if ne := windowFromHandle(win.GetParent(nle.hWnd)); ne != nil {
		if wnd := windowFromHandle(win.GetParent(ne.Handle())); wnd != nil {
			if _, ok := wnd.(Container); ok {
				ne.(Widget).AsWidgetBase().invalidateBorderInParent()
			}
		}
	}
}

func (ne *NumberEdit) SetToolTipText(s string) error {
	return ne.edit.SetToolTipText(s)
}

// Copyright 2016 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package declarative

import (
	"github.com/miu200521358/walk/pkg/walk"
)

type Slider struct {
	// Window

	Accessibility      Accessibility
	Background         Brush
	ContextMenuItems   []MenuItem
	DoubleBuffering    bool
	Enabled            Property
	Font               Font
	MaxSize            Size
	MinSize            Size
	Name               string
	OnBoundsChanged    walk.EventHandler
	OnKeyDown          walk.KeyEventHandler
	OnKeyPress         walk.KeyEventHandler
	OnKeyUp            walk.KeyEventHandler
	OnMouseDown        walk.MouseEventHandler
	OnMouseMove        walk.MouseEventHandler
	OnMouseUp          walk.MouseEventHandler
	OnSizeChanged      walk.EventHandler
	Persistent         bool
	RightToLeftReading bool
	ToolTipText        Property
	Visible            Property

	// Widget

	Alignment          Alignment2D
	AlwaysConsumeSpace bool
	Column             int
	ColumnSpan         int
	GraphicsEffects    []walk.WidgetGraphicsEffect
	Row                int
	RowSpan            int
	StretchFactor      int

	// Slider

	AssignTo       **walk.Slider
	LineSize       int
	MaxValue       int
	MinValue       int
	Orientation    Orientation
	OnValueChanged walk.EventHandler
	PageSize       int
	ToolTipsHidden bool
	Tracking       bool
	Increment      int // スライダーのメモリ
	Value          Property
}

func (sl Slider) Create(builder *Builder) error {
	w, err := walk.NewSliderWithCfg(builder.Parent(), &walk.SliderCfg{
		Orientation:    walk.Orientation(sl.Orientation),
		ToolTipsHidden: sl.ToolTipsHidden,
		Increment:      sl.Increment,
	})
	if err != nil {
		return err
	}

	if sl.AssignTo != nil {
		*sl.AssignTo = w
	}

	return builder.InitWidget(sl, w, func() error {
		w.SetPersistent(sl.Persistent)
		if sl.LineSize > 0 {
			w.SetLineSize(sl.LineSize)
		}
		if sl.PageSize > 0 {
			w.SetPageSize(sl.PageSize)
		}
		w.SetTracking(sl.Tracking)

		if sl.MaxValue >= sl.MinValue {
			w.SetRange(sl.MinValue, sl.MaxValue)
		}

		if sl.Increment > 0 {
			w.SetIncrement(sl.Increment)
		}

		if sl.OnValueChanged != nil {
			w.ValueChanged().Attach(sl.OnValueChanged)
		}

		return nil
	})
}

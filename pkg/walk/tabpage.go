// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

import (
	"github.com/miu200521358/win"
)

const tabPageWindowClass = `\o/ Walk_TabPage_Class \o/`

var tabPageBackgroundBrush Brush

func init() {
	AppendToWalkInit(func() {
		MustRegisterWindowClass(tabPageWindowClass)

		tabPageBackgroundBrush, _ = NewSystemColorBrush(win.COLOR_WINDOW)
	})
}

type TabPage struct {
	ContainerBase
	image                 Image
	title                 string
	tabWidget             *TabWidget
	titleChangedPublisher EventPublisher
	imageChangedPublisher EventPublisher
}

func NewTabPage(container Container) (*TabPage, error) {
	tp := new(TabPage)

	if err := InitWindow(
		tp,
		container,
		tabPageWindowClass,
		win.WS_CHILD,
		win.WS_EX_CONTROLPARENT); err != nil {
		return nil, err
	}

	tp.children = newWidgetList(tp)

	tp.MustRegisterProperty("Title", NewProperty(
		func() interface{} {
			return tp.Title()
		},
		func(v interface{}) error {
			return tp.SetTitle(assertStringOr(v, ""))
		},
		tp.titleChangedPublisher.Event()))

	tp.MustRegisterProperty("Image", NewProperty(
		func() interface{} {
			return tp.Image()
		},
		func(v interface{}) error {
			img, err := ImageFrom(v)
			if err != nil {
				return err
			}

			return tp.SetImage(img)
		},
		tp.imageChangedPublisher.Event()))

	return tp, nil
}

func (tp *TabPage) Enabled() bool {
	if tp.tabWidget != nil {
		return tp.tabWidget.Enabled() && tp.enabled
	}

	return tp.enabled
}

func (tp *TabPage) Background() Brush {
	if tp.background != nil {
		return tp.background
	} else if tp.tabWidget != nil && tp.tabWidget.background == nullBrushSingleton {
		return nullBrushSingleton
	}

	if win.IsAppThemed() {
		return tabPageBackgroundBrush
	}

	return nil
}

func (tp *TabPage) Font() *Font {
	if tp.font != nil {
		return tp.font
	} else if tp.tabWidget != nil {
		return tp.tabWidget.Font()
	}

	return defaultFont
}

func (tp *TabPage) Image() Image {
	return tp.image
}

func (tp *TabPage) SetImage(value Image) error {
	tp.image = value

	if tp.tabWidget == nil {
		return nil
	}

	return tp.tabWidget.onPageChanged(tp)
}

func (tp *TabPage) Title() string {
	return tp.title
}

func (tp *TabPage) SetTitle(value string) error {
	tp.title = value

	tp.titleChangedPublisher.Publish()

	if tp.tabWidget == nil {
		return nil
	}

	return tp.tabWidget.onPageChanged(tp)
}

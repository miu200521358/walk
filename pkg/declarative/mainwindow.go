// Copyright 2012 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package declarative

import "github.com/miu200521358/walk/pkg/walk"

type MainWindow struct {
	// Window

	Accessibility    Accessibility
	Background       Brush
	ContextMenuItems []MenuItem
	DoubleBuffering  bool
	Enabled          Property
	Font             Font
	MaxSize          Size
	MinSize          Size
	Name             string
	OnBoundsChanged  walk.EventHandler
	OnKeyDown        walk.KeyEventHandler
	OnKeyPress       walk.KeyEventHandler
	OnKeyUp          walk.KeyEventHandler
	OnMouseDown      walk.MouseEventHandler
	OnMouseMove      walk.MouseEventHandler
	OnMouseUp        walk.MouseEventHandler
	OnSizeChanged    walk.EventHandler
	OnClosing        walk.CloseEventHandler
	OnActivate       walk.EventHandler
	OnClickActivate  walk.EventHandler
	OnDeactivate     walk.EventHandler
	// OnMoving           walk.RectEventHandler
	OnEnterSizeMove    walk.EventHandler
	OnExitSizeMove     walk.EventHandler
	OnRestore          walk.EventHandler
	OnMinimize         walk.EventHandler
	Persistent         bool
	RightToLeftLayout  bool
	RightToLeftReading bool
	ToolTipText        Property
	Visible            Property

	// Container

	Children   []Widget
	DataBinder DataBinder
	Layout     Layout

	// Form

	Icon  Property
	Size  Size
	Title Property

	// MainWindow

	AssignTo          **walk.MainWindow
	Bounds            Rectangle
	Expressions       func() map[string]walk.Expression
	Functions         map[string]func(args ...interface{}) (interface{}, error)
	MenuItems         []MenuItem
	OnDropFiles       walk.DropFilesEventHandler
	StatusBarItems    []StatusBarItem
	SuspendedUntilRun bool
	ToolBar           ToolBar
	ToolBarItems      []MenuItem // Deprecated: use ToolBar instead
}

func (mw MainWindow) Create() error {
	w, err := walk.NewMainWindowWithCfg(&walk.MainWindowCfg{
		Name:   mw.Name,
		Bounds: mw.Bounds.toW(),
	})
	if err != nil {
		return err
	}

	if mw.AssignTo != nil {
		*mw.AssignTo = w
	}

	fi := formInfo{
		// Window
		Background:         mw.Background,
		ContextMenuItems:   mw.ContextMenuItems,
		DoubleBuffering:    mw.DoubleBuffering,
		Enabled:            mw.Enabled,
		Font:               mw.Font,
		MaxSize:            mw.MaxSize,
		MinSize:            mw.MinSize,
		Name:               mw.Name,
		OnBoundsChanged:    mw.OnBoundsChanged,
		OnKeyDown:          mw.OnKeyDown,
		OnKeyPress:         mw.OnKeyPress,
		OnKeyUp:            mw.OnKeyUp,
		OnMouseDown:        mw.OnMouseDown,
		OnMouseMove:        mw.OnMouseMove,
		OnMouseUp:          mw.OnMouseUp,
		OnSizeChanged:      mw.OnSizeChanged,
		RightToLeftReading: mw.RightToLeftReading,
		Visible:            mw.Visible,
		Accessibility:      mw.Accessibility,

		// Container
		Children:   mw.Children,
		DataBinder: mw.DataBinder,
		Layout:     mw.Layout,

		// Form
		Icon:  mw.Icon,
		Title: mw.Title,
	}

	builder := NewBuilder(nil)

	w.SetSuspended(true)
	if !mw.SuspendedUntilRun {
		builder.Defer(func() error {
			w.SetSuspended(false)
			return nil
		})
	}

	builder.deferBuildMenuActions(w.Menu(), mw.MenuItems)

	if err := w.SetRightToLeftLayout(mw.RightToLeftLayout); err != nil {
		return err
	}

	return builder.InitWidget(fi, w, func() error {
		if len(mw.ToolBar.Items) > 0 {
			var tb *walk.ToolBar
			if mw.ToolBar.AssignTo == nil {
				mw.ToolBar.AssignTo = &tb
			}

			if err := mw.ToolBar.Create(builder); err != nil {
				return err
			}

			old := w.ToolBar()
			w.SetToolBar(*mw.ToolBar.AssignTo)
			old.Dispose()
		} else {
			builder.deferBuildActions(w.ToolBar().Actions(), mw.ToolBarItems)
		}

		for _, sbi := range mw.StatusBarItems {
			s := walk.NewStatusBarItem()
			if sbi.AssignTo != nil {
				*sbi.AssignTo = s
			}
			s.SetIcon(sbi.Icon)
			s.SetText(sbi.Text)
			s.SetToolTipText(sbi.ToolTipText)
			if sbi.Width > 0 {
				s.SetWidth(sbi.Width)
			}
			if sbi.OnClicked != nil {
				s.Clicked().Attach(sbi.OnClicked)
			}
			w.StatusBar().Items().Add(s)
		}

		if mw.Size.Width > 0 && mw.Size.Height > 0 {
			if err := w.SetSize(mw.Size.toW()); err != nil {
				return err
			}
		}

		imageList, err := walk.NewImageListForDPI(walk.SizeFrom96DPI(walk.Size{16, 16}, builder.dpi), 0, builder.dpi)
		if err != nil {
			return err
		}
		w.ToolBar().SetImageList(imageList)

		if mw.OnDropFiles != nil {
			w.DropFiles().Attach(mw.OnDropFiles)
		}

		if mw.OnClosing != nil {
			w.Closing().Attach(mw.OnClosing)
		}

		if mw.OnActivate != nil {
			w.Activating().Attach(mw.OnActivate)
		}

		if mw.OnClickActivate != nil {
			w.ClickActivating().Attach(mw.OnClickActivate)
		}

		if mw.OnDeactivate != nil {
			w.Deactivating().Attach(mw.OnDeactivate)
		}

		if mw.OnRestore != nil {
			w.Restored().Attach(mw.OnRestore)
		}

		if mw.OnMinimize != nil {
			w.Minimized().Attach(mw.OnMinimize)
		}

		// if mw.OnMoving != nil {
		// 	w.Moving().Attach(mw.OnMoving)
		// }

		if mw.OnEnterSizeMove != nil {
			w.EnterSizeMove().Attach(mw.OnEnterSizeMove)
		}

		if mw.OnExitSizeMove != nil {
			w.ExitSizeMove().Attach(mw.OnExitSizeMove)
		}

		// if mw.AssignTo != nil {
		// 	*mw.AssignTo = w
		// }

		if mw.Expressions != nil {
			for name, expr := range mw.Expressions() {
				builder.expressions[name] = expr
			}
		}
		if mw.Functions != nil {
			for name, fn := range mw.Functions {
				builder.functions[name] = fn
			}
		}

		builder.Defer(func() error {
			if mw.Visible != false {
				w.Show()
			}

			return nil
		})

		return nil
	})
}

func (mw MainWindow) Run() (int, error) {
	var w *walk.MainWindow

	if mw.AssignTo == nil {
		mw.AssignTo = &w
	}

	if err := mw.Create(); err != nil {
		return 0, err
	}

	return (*mw.AssignTo).Run(), nil
}

type StatusBarItem struct {
	AssignTo    **walk.StatusBarItem
	Icon        *walk.Icon
	Text        string
	ToolTipText string
	Width       int
	OnClicked   walk.EventHandler
}

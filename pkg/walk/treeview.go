// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

import (
	"syscall"
	"unsafe"

	"github.com/miu200521358/win"
)

type treeViewItemInfo struct {
	handle       win.HTREEITEM
	child2Handle map[TreeItem]win.HTREEITEM
}

type TreeView struct {
	WidgetBase
	model                          TreeModel
	lazyPopulation                 bool
	itemsResetEventHandlerHandle   int
	itemChangedEventHandlerHandle  int
	itemInsertedEventHandlerHandle int
	itemRemovedEventHandlerHandle  int
	itemCheckedEventHandlerHandle  int
	item2Info                      map[TreeItem]*treeViewItemInfo
	handle2Item                    map[win.HTREEITEM]TreeItem
	currItem                       TreeItem
	hIml                           win.HIMAGELIST
	usingSysIml                    bool
	imageUintptr2Index             map[uintptr]int32
	filePath2IconIndex             map[string]int32
	expandedChangedPublisher       TreeItemEventPublisher
	currentItemChangedPublisher    EventPublisher
	itemActivatedPublisher         EventPublisher
	itemCheckedPublisher           TreeCheckableItemEventPublisher
}

func NewTreeView(parent Container, checkable bool) (*TreeView, error) {
	tv := new(TreeView)

	style := uint32(win.WS_TABSTOP | win.WS_VISIBLE | win.TVS_HASBUTTONS | win.TVS_LINESATROOT | win.TVS_SHOWSELALWAYS | win.TVS_TRACKSELECT)
	if checkable {
		style |= win.TVS_CHECKBOXES
	}

	if err := InitWidget(
		tv,
		parent,
		"SysTreeView32",
		style,
		win.WS_EX_CLIENTEDGE); err != nil {
		return nil, err
	}

	succeeded := false
	defer func() {
		if !succeeded {
			tv.Dispose()
		}
	}()

	if hr := win.HRESULT(tv.SendMessage(win.TVM_SETEXTENDEDSTYLE, win.TVS_EX_DOUBLEBUFFER, win.TVS_EX_DOUBLEBUFFER)); win.FAILED(hr) {
		return nil, errorFromHRESULT("TVM_SETEXTENDEDSTYLE", hr)
	}

	if err := tv.setTheme("Explorer"); err != nil {
		return nil, err
	}

	tv.GraphicsEffects().Add(InteractionEffect)
	tv.GraphicsEffects().Add(FocusEffect)

	tv.MustRegisterProperty("CurrentItem", NewReadOnlyProperty(
		func() interface{} {
			return tv.CurrentItem()
		},
		tv.CurrentItemChanged()))

	tv.MustRegisterProperty("CurrentItemLevel", NewReadOnlyProperty(
		func() interface{} {
			level := -1
			item := tv.CurrentItem()

			for item != nil {
				level++
				item = item.Parent()
			}

			return level
		},
		tv.CurrentItemChanged()))

	tv.MustRegisterProperty("HasCurrentItem", NewReadOnlyBoolProperty(
		func() bool {
			return tv.CurrentItem() != nil
		},
		tv.CurrentItemChanged()))

	succeeded = true

	return tv, nil
}

func (tv *TreeView) Dispose() {
	tv.WidgetBase.Dispose()

	tv.disposeImageListAndCaches()
}

func (tv *TreeView) SetBackground(bg Brush) {
	tv.WidgetBase.SetBackground(bg)

	color := Color(win.GetSysColor(win.COLOR_WINDOW))

	if bg != nil {
		type Colorer interface {
			Color() Color
		}

		if c, ok := bg.(Colorer); ok {
			color = c.Color()
		}
	}

	tv.SendMessage(win.TVM_SETBKCOLOR, 0, uintptr(color))
}

func (tv *TreeView) Model() TreeModel {
	return tv.model
}

func (tv *TreeView) SetModel(model TreeModel) error {
	if tv.model != nil {
		tv.model.ItemsReset().Detach(tv.itemsResetEventHandlerHandle)
		tv.model.ItemChanged().Detach(tv.itemChangedEventHandlerHandle)
		tv.model.ItemInserted().Detach(tv.itemInsertedEventHandlerHandle)
		tv.model.ItemRemoved().Detach(tv.itemRemovedEventHandlerHandle)
		tv.model.ItemChecked().Detach(tv.itemCheckedEventHandlerHandle)

		tv.disposeImageListAndCaches()
	}

	tv.model = model

	if model != nil {
		tv.lazyPopulation = model.LazyPopulation()

		tv.itemsResetEventHandlerHandle = model.ItemsReset().Attach(func(parent TreeItem) {
			if parent == nil {
				tv.resetItems()
			} else if tv.item2Info[parent] != nil {
				tv.SetSuspended(true)
				defer tv.SetSuspended(false)

				if err := tv.removeDescendants(parent); err != nil {
					return
				}

				if err := tv.insertChildren(parent); err != nil {
					return
				}
			}
		})

		tv.itemChangedEventHandlerHandle = model.ItemChanged().Attach(func(item TreeItem) {
			if item == nil || tv.item2Info[item] == nil {
				return
			}

			if err := tv.updateItem(item); err != nil {
				return
			}
		})

		tv.itemInsertedEventHandlerHandle = model.ItemInserted().Attach(func(item TreeItem) {
			tv.SetSuspended(true)
			defer tv.SetSuspended(false)

			var hInsertAfter win.HTREEITEM
			parent := item.Parent()
			for i := parent.ChildCount() - 1; i >= 0; i-- {
				if parent.ChildAt(i) == item {
					if i > 0 {
						hInsertAfter = tv.item2Info[parent.ChildAt(i-1)].handle
					} else {
						hInsertAfter = win.TVI_FIRST
					}
				}
			}

			if _, err := tv.insertItemAfter(item, hInsertAfter); err != nil {
				return
			}
		})

		tv.itemRemovedEventHandlerHandle = model.ItemRemoved().Attach(func(item TreeItem) {
			if err := tv.removeItem(item); err != nil {
				return
			}
		})

		tv.itemCheckedEventHandlerHandle = model.ItemChecked().Attach(func(item TreeCheckableItem) {
			// モデルからのチェック状態変更をTreeViewに反映
			if checkableItem, ok := item.(interface{ Checked() bool }); ok {
				if err := tv.SetChecked(item, checkableItem.Checked()); err != nil {
					return
				}
			}
			tv.itemCheckedPublisher.Publish(item)
		})
	}

	return tv.resetItems()
}

func (tv *TreeView) CurrentItem() TreeItem {
	return tv.currItem
}

func (tv *TreeView) SetCurrentItem(item TreeItem) error {
	if item == tv.currItem {
		return nil
	}

	if item != nil {
		if err := tv.ensureItemAndAncestorsInserted(item); err != nil {
			return err
		}
	}

	handle, err := tv.handleForItem(item)
	if err != nil {
		return err
	}

	if 0 == tv.SendMessage(win.TVM_SELECTITEM, win.TVGN_CARET, uintptr(handle)) {
		return newError("SendMessage(TVM_SELECTITEM) failed")
	}

	tv.currItem = item

	return nil
}

func (tv *TreeView) EnsureVisible(item TreeItem) error {
	handle, err := tv.handleForItem(item)
	if err != nil {
		return err
	}

	tv.SendMessage(win.TVM_ENSUREVISIBLE, 0, uintptr(handle))

	return nil
}

func (tv *TreeView) handleForItem(item TreeItem) (win.HTREEITEM, error) {
	if item != nil {
		if info := tv.item2Info[item]; info == nil {
			return 0, newError("invalid item")
		} else {
			return info.handle, nil
		}
	}

	return 0, newError("invalid item")
}

// ItemAt determines the location of the specified point in native pixels relative to the client area of a tree-view control.
func (tv *TreeView) ItemAt(x, y int) TreeItem {
	hti := win.TVHITTESTINFO{Pt: Point{x, y}.toPOINT()}

	tv.SendMessage(win.TVM_HITTEST, 0, uintptr(unsafe.Pointer(&hti)))

	if item, ok := tv.handle2Item[hti.HItem]; ok {
		return item
	}

	return nil
}

// ItemHeight returns the height of each item in native pixels.
func (tv *TreeView) ItemHeight() int {
	return int(tv.SendMessage(win.TVM_GETITEMHEIGHT, 0, 0))
}

// SetItemHeight sets the height of the tree-view items in native pixels.
func (tv *TreeView) SetItemHeight(height int) {
	tv.SendMessage(win.TVM_SETITEMHEIGHT, uintptr(height), 0)
}

func (tv *TreeView) resetItems() error {
	tv.SetSuspended(true)
	defer tv.SetSuspended(false)

	if err := tv.clearItems(); err != nil {
		return err
	}

	if tv.model == nil {
		return nil
	}

	if err := tv.insertRoots(); err != nil {
		return err
	}

	return nil
}

func (tv *TreeView) ApplyRootCheckStates() {
	// すべてのルートアイテムから再帰的に適用
	if tv.model != nil {
		tv.SetSuspended(true)
		defer tv.SetSuspended(false)

		for i := 0; i < tv.model.RootCount(); i++ {
			item := tv.model.RootAt(i)
			if item == nil {
				continue
			}
			if checkableItem, ok := item.(interface{ Checked() bool }); ok {
				if err := tv.SetChecked(item, checkableItem.Checked()); err != nil {
					// エラーログを出力（必要に応じて）
				}
			}
		}
	}
}

func (tv *TreeView) ExpandAll() {
	// すべてのルートアイテムから再帰的に適用
	if tv.model != nil {
		tv.SetSuspended(true)
		defer tv.SetSuspended(false)

		for i := 0; i < tv.model.RootCount(); i++ {
			item := tv.model.RootAt(i)
			if item == nil {
				continue
			}

			tv.expandChildren(item)
		}
	}
}

func (tv *TreeView) expandChildren(item TreeItem) {
	// すべての子アイテムを展開
	if tv.model != nil {
		tv.SetExpanded(item, true)

		for i := 0; i < item.ChildCount(); i++ {
			child := item.ChildAt(i)
			if child == nil {
				continue
			}
			if err := tv.SetExpanded(child, true); err != nil {
				// エラーログを出力（必要に応じて）
			}
			for childIndex := range child.ChildCount() {
				grandChild := child.ChildAt(childIndex)
				tv.expandChildren(grandChild)
			}
		}
	}
}

func (tv *TreeView) applyCheckStateRecursive(item TreeItem) {
	// 現在のアイテムのチェック状態を適用
	if checkableItem, ok := item.(interface{ Checked() bool }); ok {
		if err := tv.SetChecked(item, checkableItem.Checked()); err != nil {
			// エラーログを出力（必要に応じて）
		}
	}

	// 子アイテムも再帰的に処理
	for i := 0; i < item.ChildCount(); i++ {
		child := item.ChildAt(i)

		// 子アイテムがTreeViewに存在する場合のみ処理
		if tv.item2Info[child] != nil {
			tv.applyCheckStateRecursive(child)
		}
	}
}

func (tv *TreeView) clearItems() error {
	if 0 == tv.SendMessage(win.TVM_DELETEITEM, 0, 0) {
		return newError("SendMessage(TVM_DELETEITEM) failed")
	}

	tv.item2Info = make(map[TreeItem]*treeViewItemInfo)
	tv.handle2Item = make(map[win.HTREEITEM]TreeItem)

	return nil
}

func (tv *TreeView) insertRoots() error {
	for i := tv.model.RootCount() - 1; i >= 0; i-- {
		if _, err := tv.insertItem(tv.model.RootAt(i)); err != nil {
			return err
		}
	}

	return nil
}

func (tv *TreeView) ApplyDPI(dpi int) {
	tv.WidgetBase.ApplyDPI(dpi)

	tv.disposeImageListAndCaches()
}

func (tv *TreeView) applyImageListForImage(image interface{}) {
	tv.hIml, tv.usingSysIml, _ = imageListForImage(image, tv.DPI())

	tv.SendMessage(win.TVM_SETIMAGELIST, 0, uintptr(tv.hIml))

	tv.imageUintptr2Index = make(map[uintptr]int32)
	tv.filePath2IconIndex = make(map[string]int32)
}

func (tv *TreeView) disposeImageListAndCaches() {
	if tv.hIml != 0 && !tv.usingSysIml {
		win.ImageList_Destroy(tv.hIml)
	}
	tv.hIml = 0

	tv.imageUintptr2Index = nil
	tv.filePath2IconIndex = nil
}

func (tv *TreeView) setTVITEMImageInfo(tvi *win.TVITEM, item TreeItem) {
	if imager, ok := item.(Imager); ok {
		if tv.hIml == 0 {
			tv.applyImageListForImage(imager.Image())
		}

		// FIXME: If not setting TVIF_SELECTEDIMAGE and tvi.ISelectedImage,
		// some default icon will show up, even though we have not asked for it.

		tvi.Mask |= win.TVIF_IMAGE | win.TVIF_SELECTEDIMAGE
		tvi.IImage = imageIndexMaybeAdd(
			imager.Image(),
			tv.hIml,
			tv.usingSysIml,
			tv.imageUintptr2Index,
			tv.filePath2IconIndex,
			tv.DPI())

		tvi.ISelectedImage = tvi.IImage
	}
}

func (tv *TreeView) insertItem(item TreeItem) (win.HTREEITEM, error) {
	return tv.insertItemAfter(item, win.TVI_FIRST)
}

func (tv *TreeView) insertItemAfter(item TreeItem, hInsertAfter win.HTREEITEM) (win.HTREEITEM, error) {
	var tvins win.TVINSERTSTRUCT
	tvi := &tvins.Item

	tvi.Mask = win.TVIF_CHILDREN | win.TVIF_TEXT
	tvi.PszText = win.LPSTR_TEXTCALLBACK
	tvi.CChildren = win.I_CHILDRENCALLBACK

	// チェック状態を設定
	if checkableItem, ok := item.(interface{ Checked() bool }); ok {
		tvi.Mask |= win.TVIF_STATE
		if checkableItem.Checked() {
			tvi.State = 2 << 12 // checked
		} else {
			tvi.State = 1 << 12 // unchecked
		}
		tvi.StateMask = win.TVIS_STATEIMAGEMASK
	}

	tv.setTVITEMImageInfo(tvi, item)

	parent := item.Parent()

	if parent == nil {
		tvins.HParent = win.TVI_ROOT
	} else {
		info := tv.item2Info[parent]
		if info == nil {
			return 0, newError("invalid parent")
		}
		tvins.HParent = info.handle
	}

	tvins.HInsertAfter = hInsertAfter

	hItem := win.HTREEITEM(tv.SendMessage(win.TVM_INSERTITEM, 0, uintptr(unsafe.Pointer(&tvins))))
	if hItem == 0 {
		return 0, newError("TVM_INSERTITEM failed")
	}
	tv.item2Info[item] = &treeViewItemInfo{hItem, make(map[TreeItem]win.HTREEITEM)}
	tv.handle2Item[hItem] = item

	if !tv.lazyPopulation {
		if err := tv.insertChildren(item); err != nil {
			return 0, err
		}
	}

	return hItem, nil
}

func (tv *TreeView) insertChildren(parent TreeItem) error {
	info := tv.item2Info[parent]

	for i := parent.ChildCount() - 1; i >= 0; i-- {
		child := parent.ChildAt(i)

		if handle, err := tv.insertItem(child); err != nil {
			return err
		} else {
			info.child2Handle[child] = handle
		}
	}

	return nil
}

func (tv *TreeView) updateItem(item TreeItem) error {
	tvi := &win.TVITEM{
		Mask:    win.TVIF_TEXT,
		HItem:   tv.item2Info[item].handle,
		PszText: win.LPSTR_TEXTCALLBACK,
	}

	tv.setTVITEMImageInfo(tvi, item)

	if 0 == tv.SendMessage(win.TVM_SETITEM, 0, uintptr(unsafe.Pointer(tvi))) {
		return newError("SendMessage(TVM_SETITEM) failed")
	}

	return nil
}

func (tv *TreeView) removeItem(item TreeItem) error {
	if err := tv.removeDescendants(item); err != nil {
		return err
	}

	info := tv.item2Info[item]
	if info == nil {
		return newError("invalid item")
	}

	if 0 == tv.SendMessage(win.TVM_DELETEITEM, 0, uintptr(info.handle)) {
		return newError("SendMessage(TVM_DELETEITEM) failed")
	}

	if parentInfo := tv.item2Info[item.Parent()]; parentInfo != nil {
		delete(parentInfo.child2Handle, item)
	}
	delete(tv.item2Info, item)
	delete(tv.handle2Item, info.handle)

	return nil
}

func (tv *TreeView) removeDescendants(parent TreeItem) error {
	for item, _ := range tv.item2Info[parent].child2Handle {
		if err := tv.removeItem(item); err != nil {
			return err
		}
	}

	return nil
}

func (tv *TreeView) ensureItemAndAncestorsInserted(item TreeItem) error {
	if item == nil {
		return newError("invalid item")
	}

	tv.SetSuspended(true)
	defer tv.SetSuspended(false)

	var hierarchy []TreeItem

	for item != nil && tv.item2Info[item] == nil {
		item = item.Parent()

		if item != nil {
			hierarchy = append(hierarchy, item)
		} else {
			return newError("invalid item")
		}
	}

	for i := len(hierarchy) - 1; i >= 0; i-- {
		if err := tv.insertChildren(hierarchy[i]); err != nil {
			return err
		}
	}

	return nil
}

func (tv *TreeView) Expanded(item TreeItem) bool {
	if tv.item2Info[item] == nil {
		return false
	}

	tvi := &win.TVITEM{
		HItem:     tv.item2Info[item].handle,
		Mask:      win.TVIF_STATE,
		StateMask: win.TVIS_EXPANDED,
	}

	if 0 == tv.SendMessage(win.TVM_GETITEM, 0, uintptr(unsafe.Pointer(tvi))) {
		newError("SendMessage(TVM_GETITEM) failed")
	}

	return tvi.State&win.TVIS_EXPANDED != 0
}

func (tv *TreeView) SetExpanded(item TreeItem, expanded bool) error {
	if expanded {
		if err := tv.ensureItemAndAncestorsInserted(item); err != nil {
			return err
		}
	}

	info := tv.item2Info[item]
	if info == nil {
		return newError("invalid item")
	}

	var action uintptr
	if expanded {
		action = win.TVE_EXPAND
	} else {
		action = win.TVE_COLLAPSE
	}

	if 0 == tv.SendMessage(win.TVM_EXPAND, action, uintptr(info.handle)) {
		return newError("SendMessage(TVM_EXPAND) failed")
	}

	return nil
}

func (tv *TreeView) ExpandedChanged() *TreeItemEvent {
	return tv.expandedChangedPublisher.Event()
}

func (tv *TreeView) CurrentItemChanged() *Event {
	return tv.currentItemChangedPublisher.Event()
}

func (tv *TreeView) ItemActivated() *Event {
	return tv.itemActivatedPublisher.Event()
}

func (tv *TreeView) ItemChecked() *TreeCheckableItemEvent {
	return tv.itemCheckedPublisher.Event()
}

func (tv *TreeView) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_GETDLGCODE:
		if wParam == win.VK_RETURN {
			return win.DLGC_WANTALLKEYS
		}

	case win.WM_NOTIFY:
		nmhdr := (*win.NMHDR)(unsafe.Pointer(lParam))

		switch nmhdr.Code {
		case win.TVN_GETDISPINFO:
			nmtvdi := (*win.NMTVDISPINFO)(unsafe.Pointer(lParam))
			item := tv.handle2Item[nmtvdi.Item.HItem]

			if nmtvdi.Item.Mask&win.TVIF_TEXT != 0 {
				text := item.Text()
				utf16 := syscall.StringToUTF16(text)
				buf := (*[264]uint16)(unsafe.Pointer(nmtvdi.Item.PszText))
				max := mini(len(utf16), int(nmtvdi.Item.CchTextMax))
				copy((*buf)[:], utf16[:max])
				(*buf)[max-1] = 0
			}
			if nmtvdi.Item.Mask&win.TVIF_CHILDREN != 0 {
				if hc, ok := item.(HasChilder); ok {
					if hc.HasChild() {
						nmtvdi.Item.CChildren = 1
					} else {
						nmtvdi.Item.CChildren = 0
					}
				} else {
					nmtvdi.Item.CChildren = int32(item.ChildCount())
				}
			}

		case win.TVN_ITEMEXPANDING:
			nmtv := (*win.NMTREEVIEW)(unsafe.Pointer(lParam))
			item := tv.handle2Item[nmtv.ItemNew.HItem]

			if nmtv.Action == win.TVE_EXPAND && tv.lazyPopulation {
				info := tv.item2Info[item]
				if len(info.child2Handle) == 0 {
					tv.insertChildren(item)

					// 挿入後にチェック状態を適用
					tv.applyCheckStateRecursive(item)
				}
			}

		case win.TVN_ITEMEXPANDED:
			nmtv := (*win.NMTREEVIEW)(unsafe.Pointer(lParam))
			item := tv.handle2Item[nmtv.ItemNew.HItem]

			switch nmtv.Action {
			case win.TVE_COLLAPSE:
				tv.expandedChangedPublisher.Publish(item)

			case win.TVE_COLLAPSERESET:

			case win.TVE_EXPAND:
				tv.expandedChangedPublisher.Publish(item)

				// 展開時に子アイテムのチェック状態をモデルから反映
				tv.applyCheckStateRecursive(item)

			case win.TVE_EXPANDPARTIAL:

			case win.TVE_TOGGLE:
			}

		case win.NM_DBLCLK:
			tv.itemActivatedPublisher.Publish()

		case win.TVN_KEYDOWN:
			nmtvkd := (*win.NMTVKEYDOWN)(unsafe.Pointer(lParam))
			if nmtvkd.WVKey == uint16(KeyReturn) {
				tv.itemActivatedPublisher.Publish()
			}

		case win.TVN_SELCHANGED:
			nmtv := (*win.NMTREEVIEW)(unsafe.Pointer(lParam))

			tv.currItem = tv.handle2Item[nmtv.ItemNew.HItem]

			tv.currentItemChangedPublisher.Publish()

		case win.NM_CLICK:
			// チェックボックスクリック時の処理
			// マウス位置を取得
			var pt win.POINT
			win.GetCursorPos(&pt)
			win.ScreenToClient(tv.hWnd, &pt)

			// ヒットテスト
			hti := win.TVHITTESTINFO{Pt: pt}
			tv.SendMessage(win.TVM_HITTEST, 0, uintptr(unsafe.Pointer(&hti)))

			// チェックボックス部分がクリックされた場合のみ処理
			if hti.Flags&win.TVHT_ONITEMSTATEICON != 0 {
				if item, ok := tv.handle2Item[hti.HItem]; ok {
					// 現在のチェック状態を取得（クリック前の状態）
					currentChecked := tv.Checked(item)
					// 新しいチェック状態は現在の状態の反転
					newChecked := !currentChecked

					// 親アイテムのチェック状態を更新
					if checkableItem, ok := item.(interface{ SetChecked(bool) }); ok {
						checkableItem.SetChecked(newChecked)
					}

					// 子アイテムのチェック状態も親に合わせる（再帰的に）
					tv.setChildrenChecked(item, newChecked)

					// チェック状態変更イベントを発行
					tv.itemCheckedPublisher.Publish(item.(TreeCheckableItem))
				}
			}
		}
	}

	return tv.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}

// setChildrenChecked sets the check state of all children recursively
func (tv *TreeView) setChildrenChecked(parent TreeItem, checked bool) {
	for i := 0; i < parent.ChildCount(); i++ {
		child := parent.ChildAt(i)

		// 子アイテムのモデルの状態を更新
		if checkableChild, ok := child.(interface{ SetChecked(bool) }); ok {
			checkableChild.SetChecked(checked)
		}

		// TreeViewのUI状態も更新
		if err := tv.SetChecked(child, checked); err == nil {
			// 孫アイテムも再帰的に処理
			tv.setChildrenChecked(child, checked)
		}
	}
}

func (*TreeView) NeedsWmSize() bool {
	return true
}

func (tv *TreeView) CreateLayoutItem(ctx *LayoutContext) LayoutItem {
	return NewGreedyLayoutItem()
}

// Checked returns whether the specified item is checked (for checkable TreeViews)
func (tv *TreeView) Checked(item TreeItem) bool {
	if item == nil {
		return false
	}

	info := tv.item2Info[item]
	if info == nil {
		return false
	}

	// TVM_GETITEMSTATEメッセージを使用してチェック状態を取得
	state := tv.SendMessage(win.TVM_GETITEMSTATE, uintptr(info.handle), win.TVIS_STATEIMAGEMASK)

	// チェック状態は上位4ビットに格納されている
	// 1 = unchecked, 2 = checked
	checkState := (state & win.TVIS_STATEIMAGEMASK) >> 12

	// 2 = checked の場合にtrueを返す
	return checkState == 2
}

// SetChecked sets the check state of the specified item (for checkable TreeViews)
func (tv *TreeView) SetChecked(item TreeItem, checked bool) error {
	if item == nil {
		return newError("invalid item")
	}

	info := tv.item2Info[item]
	if info == nil {
		return newError("invalid item")
	}

	var checkState uint32
	if checked {
		checkState = 2 << 12 // checked
	} else {
		checkState = 1 << 12 // unchecked
	}

	tvi := &win.TVITEM{
		HItem:     info.handle,
		Mask:      win.TVIF_STATE,
		State:     checkState,
		StateMask: win.TVIS_STATEIMAGEMASK,
	}

	if tv.SendMessage(win.TVM_SETITEM, 0, uintptr(unsafe.Pointer(tvi))) == 0 {
		return newError("SendMessage(TVM_SETITEM) failed")
	}

	// アイテムの再描画のみを行う（展開はしない）
	var rect win.RECT
	if tv.SendMessage(win.TVM_GETITEMRECT, uintptr(info.handle), uintptr(unsafe.Pointer(&rect))) != 0 {
		win.InvalidateRect(tv.hWnd, &rect, false)
	}

	return nil
}

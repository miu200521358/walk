// Copyright 2019 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package walk

import (
	"sync"
)

// The global window group manager instance.
var wgm windowGroupManager

// windowGroupManager manages window groups for each thread with one or
// more windows.
type windowGroupManager struct {
	mutex  sync.RWMutex
	groups map[uint32]*WindowGroup
}

// Group returns a window group for the given thread ID.
//
// The group will have its counter incremented as a result of this call.
// It is the caller's responsibility to call Done when finished with the
// group.
func (m *windowGroupManager) Group(threadID uint32) *WindowGroup {
	// Fast path with read lock
	m.mutex.RLock()
	if m.groups != nil {
		if group := m.groups[threadID]; group != nil {
			m.mutex.RUnlock()
			group.Add(1)
			return group
		}
	}
	m.mutex.RUnlock()

	// Slow path with write lock
	m.mutex.Lock()
	if m.groups == nil {
		m.groups = make(map[uint32]*WindowGroup)
	} else {
		if group := m.groups[threadID]; group != nil {
			// Another caller raced with our lock and beat us
			m.mutex.Unlock()
			group.Add(1)
			return group
		}
	}

	group := newWindowGroup(threadID, m.removeGroup)
	group.Add(1)
	m.groups[threadID] = group
	m.mutex.Unlock()

	return group
}

// removeGroup is called by window groups to remove themselves from
// the manager.
func (m *windowGroupManager) removeGroup(threadID uint32) {
	m.mutex.Lock()
	delete(m.groups, threadID)
	m.mutex.Unlock()
}

// WindowGroup holds data common to windows that share a thread.
//
// Each WindowGroup keeps track of the number of references to
// the group. When the number of references reaches zero, the
// group is disposed of.
type WindowGroup struct {
	refs       int // Tracks the number of windows that rely on this group
	ignored    int // Tracks the number of refs created by the group itself
	threadID   uint32
	completion func(uint32) // Used to tell the window group manager to remove this group
	removed    bool         // Has this group been removed from its manager? (used for race detection)
	toolTip    *ToolTip
}

// newWindowGroup returns a new window group for the given thread ID.
//
// The completion function will be called when the group is disposed of.
func newWindowGroup(threadID uint32, completion func(uint32)) *WindowGroup {
	return &WindowGroup{
		threadID:   threadID,
		completion: completion,
	}
}

// ThreadID identifies the thread that the group is affiliated with.
func (g *WindowGroup) ThreadID() uint32 {
	return g.threadID
}

// Refs returns the current number of references to the group.
func (g *WindowGroup) Refs() int {
	return g.refs
}

// Add changes the group's reference counter by delta, which may be negative.
//
// If the reference counter becomes zero the group will be disposed of.
//
// If the reference counter goes negative Add will panic.
func (g *WindowGroup) Add(delta int) {
	if g.removed {
		panic("walk: add() called on a WindowGroup that has been removed from its manager")
	}

	g.refs += delta
	if g.refs < 0 {
		panic("walk: negative WindowGroup refs counter")
	}
	if g.refs-g.ignored == 0 {
		g.dispose()
	}
}

// Done decrements the group's reference counter by one.
func (g *WindowGroup) Done() {
	g.Add(-1)
}

// ToolTip returns the tool tip control for the group, if one exists.
func (g *WindowGroup) ToolTip() *ToolTip {
	return g.toolTip
}

// CreateToolTip returns a tool tip control for the group.
//
// If a control has not already been prepared for the group one will be
// created.
func (g *WindowGroup) CreateToolTip() (*ToolTip, error) {
	if g.toolTip != nil {
		return g.toolTip, nil
	}

	tt, err := NewToolTip() // This must not call group.ToolTip()
	if err != nil {
		return nil, err
	}
	g.toolTip = tt

	// At this point the ToolTip has already added a reference for itself
	// to the group as part of the ToolTip's InitWindow process. We don't
	// want it to count toward the group's liveness, however, because it
	// would keep the group from cleaning up after itself.
	//
	// To solve this problem we also keep track of the number of
	// references that each group should ignore. The ignored references
	// are subtracted from the total number of references when evaluating
	// liveness. The expectation is that ignored references will be
	// removed as part of the group's disposal process.
	g.ignore(1)

	return tt, nil
}

// ignore changes the number of references that the group will ignore.
//
// ignore is used internally by WindowGroup to keep track of the number
// of references created by the group itself. When finished with a group,
// call Done() instead.
func (g *WindowGroup) ignore(delta int) {
	if g.removed {
		panic("walk: ignore() called on a WindowGroup that has been removed from its manager")
	}

	g.ignored += delta
	if g.ignored < 0 {
		panic("walk: negative WindowGroup ignored counter")
	}
	if g.refs-g.ignored == 0 {
		g.dispose()
	}
}

// dispose releases any resources consumed by the group.
func (g *WindowGroup) dispose() {
	if g.toolTip != nil {
		g.toolTip.Dispose()
		g.toolTip = nil
	}
	g.removed = true // race detection only
	g.completion(g.threadID)
}
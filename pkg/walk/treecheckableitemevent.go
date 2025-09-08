// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

type treeCheckableItemEventHandlerInfo struct {
	handler TreeCheckableItemEventHandler
	once    bool
}

type TreeCheckableItemEventHandler func(item TreeCheckableItem)

type TreeCheckableItemEvent struct {
	handlers []treeCheckableItemEventHandlerInfo
}

func (e *TreeCheckableItemEvent) Attach(handler TreeCheckableItemEventHandler) int {
	handlerInfo := treeCheckableItemEventHandlerInfo{handler, false}

	for i, h := range e.handlers {
		if h.handler == nil {
			e.handlers[i] = handlerInfo
			return i
		}
	}

	e.handlers = append(e.handlers, handlerInfo)

	return len(e.handlers) - 1
}

func (e *TreeCheckableItemEvent) Detach(handle int) {
	e.handlers[handle].handler = nil
}

func (e *TreeCheckableItemEvent) Once(handler TreeCheckableItemEventHandler) {
	i := e.Attach(handler)
	e.handlers[i].once = true
}

type TreeCheckableItemEventPublisher struct {
	event TreeCheckableItemEvent
}

func (p *TreeCheckableItemEventPublisher) Event() *TreeCheckableItemEvent {
	return &p.event
}

func (p *TreeCheckableItemEventPublisher) Publish(item TreeCheckableItem) {
	for i, h := range p.event.handlers {
		if h.handler != nil {
			h.handler(item)

			if h.once {
				p.event.Detach(i)
			}
		}
	}
}

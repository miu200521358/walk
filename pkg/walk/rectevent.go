// Copyright 2011 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package walk

type rectEventHandlerInfo struct {
	handler RectEventHandler
	once    bool
}

// RectEventHandler is called for rect events. x and y are measured in native pixels.
type RectEventHandler func(x, y, w, h int)

type RectEvent struct {
	handlers []rectEventHandlerInfo
}

func (e *RectEvent) Attach(handler RectEventHandler) int {
	handlerInfo := rectEventHandlerInfo{handler, false}

	for i, h := range e.handlers {
		if h.handler == nil {
			e.handlers[i] = handlerInfo
			return i
		}
	}

	e.handlers = append(e.handlers, handlerInfo)

	return len(e.handlers) - 1
}

func (e *RectEvent) Detach(handle int) {
	e.handlers[handle].handler = nil
}

func (e *RectEvent) Once(handler RectEventHandler) {
	i := e.Attach(handler)
	e.handlers[i].once = true
}

type RectEventPublisher struct {
	event RectEvent
}

func (p *RectEventPublisher) Event() *RectEvent {
	return &p.event
}

// Publish publishes rect event. x and y are measured in native pixels.
func (p *RectEventPublisher) Publish(x, y, w, h int) {
	for i, handlerInfo := range p.event.handlers {
		if handlerInfo.handler != nil {
			handlerInfo.handler(x, y, w, h)

			if handlerInfo.once {
				p.event.Detach(i)
			}
		}
	}
}

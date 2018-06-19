// Copyright (c) 2018 - Max Persson <liuyuanlin>
//
package fsm

import (
	"errors"
	"sync"
)

// Event is the info that get passed as a reference in the callbacks.
type Event struct {
	// FSM is a reference to the current FSM.
	FSM *FSM

	// Event is the event name.
	mEvent string

	// Src is the state before the transition.
	mState string

	// Err is an optional error that can be returned from a callback.
	Err error

	// Args is a optinal list of arguments passed to the callback.
	Args []interface{}
}

// eKey is a struct key used for storing the transition map.
type eKey struct {
	mState string
	mEvent string
}

// FSM is the state machine that holds the current state.
//
// It has to be created with NewFSM to function properly.
type FSM struct {
	mCurState       string
	mStates         []string
	mEventCallbacks map[eKey]EventCallback
	stateMu         sync.RWMutex
	eventMu         sync.RWMutex
}

// Callback is a function type that callbacks should use. Event is the current
// event info as the callback happens.
type EventCallback func(*Event)

func NewFSM(initState string, allState []string) *FSM {
	f := &FSM{
		mCurState: initState,
		mStates:   allState,
	}
	f.mEventCallbacks = make(map[eKey]EventCallback)
	return f
}

// Current returns the current state of the FSM.
func (f *FSM) CurState() string {
	f.stateMu.RLock()
	defer f.stateMu.RUnlock()
	return f.mCurState
}

// Is returns true if state is the current state.
func (f *FSM) IsState(state string) bool {
	f.stateMu.RLock()
	defer f.stateMu.RUnlock()
	return state == f.mCurState
}

// SetState allows the user to move to the given state from current state.
// The call does not trigger any callbacks, if defined.
func (f *FSM) SetState(state string) {
	f.stateMu.Lock()
	defer f.stateMu.Unlock()
	f.mCurState = state
	return
}

//AddStateEvent
func (f *FSM) AddStateEvent(state string, event string, eventCallback EventCallback) {
	f.eventMu.Lock()
	defer f.eventMu.Unlock()

	f.mEventCallbacks[eKey{state, event}] = eventCallback

}

func (f *FSM) Event(event string, args ...interface{}) error {
	f.eventMu.RLock()
	defer f.eventMu.RUnlock()

	f.stateMu.RLock()
	defer f.stateMu.RUnlock()

	if _, ok := f.mEventCallbacks[eKey{f.mCurState, event}]; !ok {
		//不存在
		return errors.New("NO_REGIST_EVENT")
	}

	fCallback := f.mEventCallbacks[eKey{f.mCurState, event}]

	e := &Event{f, event, f.mCurState, nil, args}

	fCallback(e)

	return e.Err
}

// Transition wraps transitioner.transition.
func (f *FSM) Transition(state string) error {
	f.eventMu.RLock()
	defer f.eventMu.RUnlock()

	f.stateMu.RLock()
	defer f.stateMu.RUnlock()

	if f.mCurState == state {
		//当前状态不用转移
		return nil
	}

	//执行当前状态的离开事件和新状态的进入事件
	if fCallback, ok := f.mEventCallbacks[eKey{f.mCurState, "leave"}]; ok {
		e := &Event{f, "leave", f.mCurState, nil, nil}
		fCallback(e)
	}
	f.stateMu.RUnlock()

	f.stateMu.Lock()
	f.mCurState = state
	f.stateMu.Unlock()

	f.stateMu.RLock()

	if fCallback, ok := f.mEventCallbacks[eKey{f.mCurState, "enter"}]; ok {
		e := &Event{f, "enter", f.mCurState, nil, nil}
		fCallback(e)
	}

	return nil
}

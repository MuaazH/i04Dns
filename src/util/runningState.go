package util

import (
	"sync"
)

type RunningState struct {
	on   bool
	cond *sync.Cond
}

func NewRunningState() *RunningState {
	return &RunningState{
		on:   false,
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (state *RunningState) SetOn() bool {
	state.cond.L.Lock()
	ret := !state.on
	state.on = true
	state.cond.L.Unlock()
	return ret
}

func (state *RunningState) SignalShutdownComplete() {
	state.cond.L.Lock()
	state.on = false
	state.cond.L.Unlock()
	state.cond.Broadcast()
}

func (state *RunningState) SetOff() {
	state.cond.L.Lock()
	if state.on {
		state.on = false
		state.cond.Wait()
	}
	state.cond.L.Unlock()
}

func (state *RunningState) IsOn() bool {
	state.cond.L.Lock()
	on := state.on
	state.cond.L.Unlock()
	return on
}

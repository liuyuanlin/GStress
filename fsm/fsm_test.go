// Copyright (c) 2013 - Max Persson <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fsm

import (
	"fmt"
	"testing"
)

func start_enter(e *Event) {

	fmt.Println("enter state: " + e.FSM.CurState())

}

func TestEnterEvent(t *testing.T) {
	fsm := NewFSM(
		"start",
		[]string{"start", "game", "end"},
	)

	fsm.AddStateEvent("start", "enter", start_enter)
	fsm.Event("enter")
	if fsm.CurState() != "start" {
		t.Error("expected state to be 'start'")
	}

}

func TestTransition(t *testing.T) {
	fsm := NewFSM(
		"start",
		[]string{"start", "game", "end"},
	)

	fsm.AddStateEvent("start", "enter", func(e *Event) {

		fmt.Println("enter state: " + e.FSM.CurState())
	})

	fsm.AddStateEvent("start", "leave", func(e *Event) {

		fmt.Println("leave state: " + e.FSM.CurState())
	})

	fsm.AddStateEvent("game", "enter", func(e *Event) {

		fmt.Println("enter state: " + e.FSM.CurState())
	})
	fsm.AddStateEvent("game", "leave", func(e *Event) {

		fmt.Println("leave state: " + e.FSM.CurState())
	})

	fsm.AddStateEvent("end", "enter", func(e *Event) {

		fmt.Println("enter state: " + e.FSM.CurState())
	})
	fsm.AddStateEvent("end", "leave", func(e *Event) {

		fmt.Println("leave state: " + e.FSM.CurState())
	})

	fsm.Transition("game")
	if fsm.CurState() != "game" {
		t.Error("expected state to be 'game'")
	}
	fsm.Transition("start")
	if fsm.CurState() != "start" {
		t.Error("expected state to be 'start'")
	}

	fsm.Transition("end")
	if fsm.CurState() != "end" {
		t.Error("expected state to be 'end'")
	}

}

package domain

import "testing"

func TestValidateTaskTransition(t *testing.T) {
	for _, test := range []struct {
		name    string
		from    TaskState
		to      TaskState
		wantErr bool
	}{
		{name: "draft to ready", from: TaskStateDraft, to: TaskStateReady},
		{name: "ready back to draft", from: TaskStateReady, to: TaskStateDraft},
		{name: "ready to archived", from: TaskStateReady, to: TaskStateArchived},
		{name: "archived cannot reopen", from: TaskStateArchived, to: TaskStateDraft, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateTaskTransition(test.from, test.to)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateTaskTransition(%s, %s) error = %v", test.from, test.to, err)
			}
		})
	}
}

func TestValidateRunTransition(t *testing.T) {
	for _, test := range []struct {
		name    string
		from    RunState
		to      RunState
		wantErr bool
	}{
		{name: "starting to running", from: RunStateStarting, to: RunStateRunning},
		{name: "running to stopping", from: RunStateRunning, to: RunStateStopping},
		{name: "stopping to stopped", from: RunStateStopping, to: RunStateStopped},
		{name: "unknown can recover", from: RunStateUnknown, to: RunStateRunning},
		{name: "terminal cannot restart", from: RunStateSucceeded, to: RunStateRunning, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateRunTransition(test.from, test.to)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateRunTransition(%s, %s) error = %v", test.from, test.to, err)
			}
		})
	}
}

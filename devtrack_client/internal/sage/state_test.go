package sage

import "testing"

func TestPauseResumeStateIsPersistentAndIdempotent(t *testing.T) {
	root := t.TempDir()
	initial, err := ReadState(root)
	if err != nil || initial.Paused || initial.Ready || initial.Capture != "not_installed" {
		t.Fatalf("unexpected initial state: %+v, %v", initial, err)
	}
	for i := 0; i < 2; i++ {
		state, err := SetPaused(root, true)
		if err != nil || !state.Paused {
			t.Fatalf("pause %d: %+v, %v", i, state, err)
		}
	}
	state, err := ReadState(root)
	if err != nil || !state.Paused {
		t.Fatalf("pause not persistent: %+v, %v", state, err)
	}
	for i := 0; i < 2; i++ {
		state, err = SetPaused(root, false)
		if err != nil || state.Paused {
			t.Fatalf("resume %d: %+v, %v", i, state, err)
		}
	}
	if cutoff, err := CaptureCutoff(root); err != nil || cutoff <= 0 {
		t.Fatalf("capture cutoff: %d, %v", cutoff, err)
	}
}

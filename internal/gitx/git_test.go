package gitx

import (
	"bytes"
	"io"
	"testing"
)

func TestSetOutput_DefaultsToStderr(t *testing.T) {
	prev := SetOutput(io.Discard)
	t.Cleanup(func() { SetOutput(prev) })

	if Output() != io.Discard {
		t.Errorf("Output() did not return the configured writer")
	}
}

func TestSetOutput_ThreadSafe(t *testing.T) {
	prev := SetOutput(&bytes.Buffer{})
	t.Cleanup(func() { SetOutput(prev) })

	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			SetOutput(&bytes.Buffer{})
			_ = Output()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}

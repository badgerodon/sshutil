package sshutil

import (
	"testing"
)

func TestCleanName(t *testing.T) {
	if cleanName(" ") != "\\ " {
		t.Errorf("Expected \" to be escaped")
	}
}

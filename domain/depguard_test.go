package domain

import (
	"os/exec"
	"testing"
)

func TestDepGuard(t *testing.T) {
	cmd := exec.Command("golangci-lint", "run", "--enable=depguard", "./")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("depguard failed:\n%s", string(out))
	}
}

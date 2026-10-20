// Package archtest guards the separation between core and presentation.
package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

// Core packages must not depend on CLI/TUI libraries.
var (
	core      = []string{"../domain", "../portal/...", "../store", "../exporter", "../report"}
	forbidden = []string{
		"github.com/charmbracelet/",
		"github.com/spf13/",
		"github.com/jannes-dailidow/hrc-exporter/internal/ui",
		"github.com/jannes-dailidow/hrc-exporter/internal/cli",
		"github.com/jannes-dailidow/hrc-exporter/internal/i18n",
	}
)

func TestCoreHasNoPresentationDeps(t *testing.T) {
	out, err := exec.Command("go", append([]string{"list", "-deps"}, core...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		for _, f := range forbidden {
			if strings.HasPrefix(dep, f) {
				t.Errorf("core depends on %s", dep)
			}
		}
	}
}

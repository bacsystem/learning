//go:build linux

package main

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureChildSetsPdeathsig(t *testing.T) {
	cmd := exec.Command("true")
	configureChild(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("goose debe morir con migrate (Pdeathsig=SIGKILL), got %+v", cmd.SysProcAttr)
	}
}

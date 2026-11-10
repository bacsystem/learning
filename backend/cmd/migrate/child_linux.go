//go:build linux

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// configureChild hace que el kernel mate a goose con SIGKILL si muere el hilo
// que lo lanzó (H95). Sin esto, un `kill -9` a migrate cierra la conexión del
// advisory lock mientras goose sigue migrando, y otra réplica toma el lock y
// corre goose en paralelo.
//
// Pdeathsig se dispara cuando termina el HILO creador, no el proceso: el
// llamador mantiene runtime.LockOSThread (lockThread) hasta que el hijo
// termina.
func configureChild(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}

func lockThread() (unlock func()) {
	runtime.LockOSThread()
	return runtime.UnlockOSThread
}

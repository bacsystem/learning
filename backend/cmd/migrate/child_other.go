//go:build !linux

package main

import "os/exec"

// Fuera de Linux no hay Pdeathsig: si migrate muere con SIGKILL, goose sigue
// corriendo sin el advisory lock y otra instancia podría migrar en paralelo
// (H95, riesgo residual). La imagen Docker y CI corren en Linux; en macOS o
// Windows (desarrollo local) no se debe matar migrate con -9 a mitad de una
// migración.
func configureChild(cmd *exec.Cmd) {}

func lockThread() (unlock func()) { return func() {} }

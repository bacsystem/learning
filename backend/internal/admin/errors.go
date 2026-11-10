package admin

import "errors"

var (
	errLastAdmin       = errors.New("no se puede cambiar el rol: es el único administrador activo")
	errUsuarioNotFound = errors.New("Usuario no encontrado.")
)

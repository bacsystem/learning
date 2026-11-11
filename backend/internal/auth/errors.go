package auth

import "errors"

var (
	ErrDNINotFound              = errors.New("El DNI no está registrado en el sistema. Contacta al administrador.")
	ErrPersonaAlreadyRegistered = errors.New("Ya existe una cuenta registrada con este DNI.")
	ErrUsernameTaken            = errors.New("El username ya está en uso.")
	ErrEmailTaken               = errors.New("El email ya está en uso.")
	ErrInvalidCredentials       = errors.New("Credenciales inválidas.")
	// ErrServicioNoDisponible: the database failed or timed out, so the
	// request cannot be answered (503, ADR 0005); never a 401.
	ErrServicioNoDisponible  = errors.New("Servicio no disponible.")
	ErrAccountLocked         = errors.New("Demasiados intentos fallidos. Cuenta bloqueada temporalmente. Intente en 15 minutos.")
	ErrAccountPending        = errors.New("Tu cuenta está pendiente de activación por el administrador.")
	ErrAccountRejected       = errors.New("Tu cuenta ha sido rechazada. Contacta al administrador.")
	ErrAccountInactive       = errors.New("Tu cuenta está desactivada.")
	ErrUserNotFound          = errors.New("Usuario no encontrado.")
	ErrInvalidToken          = errors.New("El enlace de recuperación es inválido o ha expirado.")
	ErrInvalidVerifyToken    = errors.New("El enlace de verificación es inválido o ha expirado.")
	ErrEmailMismatch         = errors.New("El correo no coincide con el registrado para este DNI. Escribe el correo que tienes registrado o contacta al administrador.")
	ErrCannotChangeOwnEstado = errors.New("No puedes desactivar ni rechazar tu propia cuenta.")
	ErrLastAdmin             = errors.New("No se puede desactivar al último administrador activo.")
)

package auth

import (
	"context"
	"testing"
)

// All service tests require a live database (via testDB) and skip otherwise.

func TestService_Register_DNINotFound(t *testing.T) {
	_, svc := testSetup(t)
	ctx := context.Background()

	err := svc.Register(ctx, RegisterRequest{
		DNI:   "00000000",
		Email: "test@example.com",
	}, "127.0.0.1", "test-agent")

	if err != ErrDNINotFound {
		t.Errorf("want ErrDNINotFound, got %v", err)
	}
}

func TestService_Login_InvalidCredentials(t *testing.T) {
	_, svc := testSetup(t)
	ctx := context.Background()

	_, err := svc.Login(ctx, LoginRequest{
		Username: "nonexistent_user",
		Password: "wrongpassword",
	}, "127.0.0.1", "test-agent")

	if err != ErrInvalidCredentials {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestService_ForgotPassword_UnknownEmail_Succeeds(t *testing.T) {
	_, svc := testSetup(t)
	ctx := context.Background()

	// ForgotPassword always succeeds silently (anti-enumeration)
	err := svc.ForgotPassword(ctx, "unknown@example.com", "127.0.0.1")
	if err != nil {
		t.Errorf("expected nil error for unknown email, got %v", err)
	}
}

func TestService_ResetPassword_InvalidToken(t *testing.T) {
	_, svc := testSetup(t)
	ctx := context.Background()

	err := svc.ResetPassword(ctx, "invalid-token-value", "NewPassword123!", "127.0.0.1")
	if err != ErrInvalidToken {
		t.Errorf("want ErrInvalidToken, got %v", err)
	}
}

func TestService_UpdateEstado_UserNotFound(t *testing.T) {
	_, svc := testSetup(t)
	ctx := context.Background()

	_, err := svc.UpdateEstado(ctx, 99999999, UpdateUsuarioEstadoRequest{
		Estado:      "activo",
		Observacion: "",
	}, 1, "")

	if err != ErrUserNotFound {
		t.Errorf("want ErrUserNotFound, got %v", err)
	}
}

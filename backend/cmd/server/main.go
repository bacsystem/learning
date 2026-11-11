package main

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cbaciliod/learning/internal/admin"
	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/catalogs"
	"github.com/cbaciliod/learning/internal/config"
	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/cbaciliod/learning/internal/inscripciones"
	"github.com/cbaciliod/learning/internal/logger"
	internalmw "github.com/cbaciliod/learning/internal/middleware"
	"github.com/cbaciliod/learning/internal/notifications"
	"github.com/cbaciliod/learning/internal/notifier"
	"github.com/cbaciliod/learning/internal/perfiles"
	"github.com/cbaciliod/learning/internal/practices"
	"github.com/cbaciliod/learning/internal/questions"
	"github.com/cbaciliod/learning/internal/salud"
	"github.com/cbaciliod/learning/internal/simulations"
	"github.com/cbaciliod/learning/internal/temas"
	"github.com/cbaciliod/learning/internal/testimonios"
	"github.com/rs/zerolog/log"
)

const (
	shutdownTimeout = 10 * time.Second
	// workerDrainTimeout bounds how long we wait for the email worker to
	// finish its in-flight batch after the shutdown signal.
	workerDrainTimeout = 30 * time.Second
)

func main() {
	os.Exit(run())
}

// run wires and runs the server. It returns the process exit code so that
// deferred cleanups (DB pool) always execute before exiting.
func run() int {
	arranque := time.Now() // uptime of GET /admin/salud (PLT-01)
	cfg, err := config.Load()
	if err != nil {
		log.Error().Err(err).Msg("config load failed")
		return 1
	}

	logger.Init(cfg.Env, cfg.LogLevel)

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Error().Err(err).Msg("db connect failed")
		return 1
	}
	defer database.Close()

	gcm, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("crypto init failed")
		return 1
	}

	// NOTE: the same ENCRYPTION_KEY is used both as the AES-256-GCM key and as
	// the HMAC-SHA256 key for the dni_hmac / email_hmac lookup columns. Using
	// separate derived keys would be better practice, but changing it now would
	// invalidate every stored HMAC (and lookups by DNI / e-mail) — it requires a
	// coordinated data migration, so it is intentionally left as is.
	rawKey, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("invalid encryption key: must be a valid hex string")
		return 1
	}
	// One trusted-proxy configuration for auth, audit and the rate limiters.
	internalmw.SetTrustedProxies(cfg.TrustedProxies)
	repo := auth.NewRepository(database, gcm, rawKey)
	// config.Load requires FRONTEND_BASE_URL outside development/test and
	// strips its trailing slash; the localhost fallback is for local work only.
	frontendBase := cfg.FrontendBaseURL
	if frontendBase == "" {
		frontendBase = "http://localhost:3000"
	}
	svc := auth.NewService(repo, cfg.JWTSecret, frontendBase)

	audLog := audit.New(database)

	insRepo := inscripciones.NewRepository(database, gcm, rawKey)
	insSvc := inscripciones.NewService(insRepo)
	tesRepo := testimonios.NewRepository(database, gcm)
	tesSvc := testimonios.NewService(tesRepo)

	catRepo := catalogs.NewRepository(database)
	catSvc := catalogs.NewService(catRepo)

	qRepo := questions.NewRepository(database)
	qSvc := questions.NewService(qRepo)

	pRepo := practices.NewRepository(database)
	pSvc := practices.NewService(pRepo, practices.NewQuestionsAdapter(qSvc))

	simRepo := simulations.NewRepository(database)
	simSvc := simulations.NewService(simRepo, simulations.NewQuestionsAdapter(qSvc))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(database, version))
	// The session checker also returns the user's current rol/estado, so role
	// changes and deactivations apply on the next request (see auth.RequireAuth).
	checkSession := auth.SessionChecker(repo.FindActiveSesion)
	auth.Mount(mux, svc, cfg.JWTSecret, checkSession)
	inscripciones.Mount(mux, insSvc, audLog, cfg.JWTSecret, checkSession)
	testimonios.Mount(mux, tesSvc, audLog, cfg.JWTSecret, checkSession)
	catalogs.Mount(mux, catSvc, cfg.JWTSecret, checkSession)
	questions.Mount(mux, qSvc, cfg.JWTSecret, checkSession)
	temas.Mount(mux, temas.NewService(temas.NewRepository(database)), audLog, cfg.JWTSecret, checkSession)
	perfiles.Mount(mux, perfiles.NewService(perfiles.NewRepository(database), catSvc), cfg.JWTSecret, checkSession)
	fuentes.Mount(mux, fuentes.NewService(fuentes.NewRepository(database), nil), audLog, cfg.JWTSecret, checkSession)
	practices.Mount(mux, pSvc, cfg.JWTSecret, checkSession)
	// Health of the services for admins, in the contract order (PLT-01).
	salud.Mount(mux, salud.NewService([]salud.Comprobacion{
		salud.ComprobarAPI(version, arranque, time.Now),
		salud.ComprobarBD(database),
		// SMTP is only optional in development/test (config.Load).
		salud.ComprobarCorreo(database, notifier.MaxRetries, cfg.SMTPHost != ""),
		salud.ComprobarCopia(database, time.Now),
	}, time.Now), cfg.JWTSecret, checkSession)
	simulations.Mount(mux, simSvc, cfg.JWTSecret, checkSession)
	notifications.Mount(mux, database, audLog, cfg.JWTSecret, checkSession)
	admin.RegisterRoutes(mux, database, gcm, rawKey, cfg.JWTSecret, checkSession, audLog)

	// Body limit: 1 MB for every body (413 above that) except multipart
	// requests to the upload routes listed here, which get a 20 MB ceiling
	// (their handlers apply smaller limits). A new upload route must be
	// added to this list; the client-sent Content-Type alone never raises
	// the limit.
	rutasSubida := append([]string{
		"POST /admin/personas/preview",
		"POST /admin/personas/import",
		"POST /preguntas/preview",
		"POST /preguntas/import",
	}, fuentes.RutasSubida...)
	bodyLimit := internalmw.MaxBodySize(internalmw.DefaultJSONBodyLimit, internalmw.DefaultMultipartBodyLimit, rutasSubida...)
	handler := internalmw.SecurityHeaders(internalmw.CORS(cfg.CORSOrigins)(bodyLimit(mux)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start notification workers. Run returns once ctx is cancelled and the
	// in-flight batch has finished; we wait for it on shutdown.
	var workers sync.WaitGroup
	emailWorker := notifier.NewEmailWorker(database, cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom)
	workers.Add(1)
	go func() {
		defer workers.Done()
		emailWorker.Run(ctx)
	}()

	// Ingest worker (ADR 0003 § Worker de ingesta): normalises and publishes
	// the 'procesando' versions of the fuentes, auditing as the system.
	ingestaWorker := fuentes.NewIngestaWorker(fuentes.NewPgCola(database), fuentes.NewPgSource(database), fuentes.AuditorSistema(audLog))
	workers.Add(1)
	go func() {
		defer workers.Done()
		ingestaWorker.Run(ctx)
	}()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", srv.Addr).Str("version", version).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	exitCode := 0
	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	case err, ok := <-serverErr:
		if ok && err != nil {
			log.Error().Err(err).Msg("server error")
			exitCode = 1
		}
	}
	stop() // cancel ctx so the workers stop picking up new work
	log.Info().Msg("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
		exitCode = 1
	}

	workersDone := make(chan struct{})
	go func() {
		workers.Wait()
		close(workersDone)
	}()
	select {
	case <-workersDone:
		log.Info().Msg("workers stopped")
	case <-time.After(workerDrainTimeout):
		log.Error().Dur("timeout", workerDrainTimeout).Msg("timed out waiting for workers to stop")
		exitCode = 1
	}
	return exitCode
}

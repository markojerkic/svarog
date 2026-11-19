package embedded

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
)

type Config struct {
	Host        string `env:"NATS_LISTEN_HOST" envDefault:"0.0.0.0"`
	Port        int    `env:"NATS_LISTEN_PORT" envDefault:"4222"` // Zero lets the OS assign a port.
	StoreDir    string `env:"NATS_STORE_DIR" envDefault:"/var/lib/svarog/nats"`
	AccountSeed string
}

type Embedded struct {
	config   Config
	ns       *server.Server
	stop     context.CancelFunc
	stopped  <-chan struct{}
	userJWT  string
	userSeed string
}

func New(cfg Config) *Embedded {
	return &Embedded{config: cfg}
}

// Start blocks until the broker accepts connections. JetStream recovers the
// persisted store inside this call, so only ctx bounds it. Start is not safe
// for concurrent use.
func (e *Embedded) Start(ctx context.Context) error {
	if e.ns != nil {
		return errors.New("embedded NATS is already started or shutting down")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(e.config.StoreDir) == "" {
		return errors.New("NATS_STORE_DIR is required for embedded JetStream")
	}
	port := e.config.Port
	if port == 0 {
		port = server.RANDOM_PORT
	}
	auth, err := newBrokerAuth(e.config.AccountSeed)
	if err != nil {
		return err
	}
	// nats-server does not lock its store; two brokers on one directory corrupt it.
	unlock, err := lockStore(e.config.StoreDir)
	if err != nil {
		return err
	}
	ns, err := server.NewServer(&server.Options{
		ServerName:       "svarog-embedded",
		Host:             e.config.Host,
		Port:             port,
		NoSigs:           true,
		JetStream:        true,
		StoreDir:         e.config.StoreDir,
		MaxPayload:       max(server.MAX_PAYLOAD_SIZE, logevent.MaxMessageBytes),
		TrustedOperators: []*jwt.OperatorClaims{auth.operator},
		AccountResolver:  auth.resolver,
		SystemAccount:    auth.operator.SystemAccount,
	})
	if err != nil {
		unlock()
		return fmt.Errorf("configure embedded NATS server: %w", err)
	}

	// Keep broker diagnostics on the console even if the app forwards later logs.
	logger := &serverLogger{Logger: slog.With("component", "nats"), fatal: make(chan error, 1)}
	ns.SetLogger(logger, false, false)
	lifetime, stop := context.WithCancel(context.Background())
	started, stopped := make(chan struct{}), make(chan struct{})
	e.ns, e.stop, e.stopped = ns, stop, stopped
	go func() {
		defer close(stopped)
		defer unlock()
		ns.Start()
		close(started)
		// Finish startup before stopping, even if the caller has timed out.
		<-lifetime.Done()
		ns.Shutdown()
		ns.WaitForShutdown()
	}()

	select {
	case <-started:
		select {
		case err = <-logger.fatal:
		default:
			// Start has returned, so the client listener should already be open.
			if !ns.ReadyForConnections(100 * time.Millisecond) {
				err = errors.New("embedded NATS is not ready for connections")
			}
		}
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		// Join the broker so the process never exits during store recovery.
		_ = e.Shutdown(context.Background())
		return fmt.Errorf("start embedded NATS server: %w", err)
	}
	e.userJWT, e.userSeed = auth.userJWT, auth.userSeed
	return nil
}

func (e *Embedded) Connect(opts natsconn.ConnectOptions) (*natsconn.NatsConnection, error) {
	if e.userJWT == "" {
		return nil, errors.New("embedded NATS is not running")
	}
	return natsconn.Connect(e.ns.ClientURL(), opts,
		nats.InProcessServer(e.ns), nats.UserJWTAndSeed(e.userJWT, e.userSeed))
}

func (e *Embedded) Shutdown(ctx context.Context) error {
	if e.ns == nil {
		return nil
	}
	e.userJWT, e.userSeed = "", ""
	e.stop()
	select {
	case <-e.stopped:
		e.ns, e.stop, e.stopped = nil, nil, nil
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type serverLogger struct {
	*slog.Logger
	fatal chan error
}

func (l *serverLogger) Noticef(format string, args ...any) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *serverLogger) Warnf(format string, args ...any) {
	l.Warn(fmt.Sprintf(format, args...))
}

func (l *serverLogger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *serverLogger) Fatalf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	l.Error(err.Error())
	select {
	case l.fatal <- err:
	default:
	}
}

func (*serverLogger) Debugf(string, ...any) {}
func (*serverLogger) Tracef(string, ...any) {}

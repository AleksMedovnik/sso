package grpcapp

import (
    "context"
    "log/slog"
	"net"
	"fmt"

	authgrpc "github.com/AleksMedovnik/sso/internal/grpc/auth"

    "google.golang.org/grpc"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

type App struct {
    log        *slog.Logger
    gRPCServer *grpc.Server
    port       int // Порт, на котором будет работать grpc-сервер
}


// New creates new gRPC server app.
func New(
    log *slog.Logger,
    authService authgrpc.Auth,
    port int,
) *App {
    loggingOpts := []logging.Option{
        logging.WithLogOnEvents(
            logging.PayloadReceived, logging.PayloadSent,
        ),
    }

    recoveryOpts := []recovery.Option{
        recovery.WithRecoveryHandler(func(p interface{}) (err error) {
            log.Error("Recovered from panic", slog.Any("panic", p))

            return status.Errorf(codes.Internal, "internal error")
        }),
    }

    gRPCServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
        recovery.UnaryServerInterceptor(recoveryOpts...),
        logging.UnaryServerInterceptor(InterceptorLogger(log), loggingOpts...),
    ))

    authgrpc.Register(gRPCServer, authService)

    return &App{
        log:        log,
        gRPCServer: gRPCServer,
        port:       port,
    }
}

// InterceptorLogger adapts slog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func InterceptorLogger(l *slog.Logger) logging.Logger {
    return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
        l.Log(ctx, slog.Level(lvl), msg, fields...)
    })
}

// MustRun runs gRPC server and panics if any error occurs.
func (a *App) MustRun() {
    if err := a.Run(); err != nil {
        panic(err)
    }
}

// Run runs gRPC server.
func (a *App) Run() error {
    const op = "grpcapp.Run"

    // Создаём listener, который будет слушить TCP-сообщения, адресованные
    // Нашему gRPC-серверу
    l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
    if err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }

    a.log.Info("grpc server started", slog.String("addr", l.Addr().String()))

    // Запускаем обработчик gRPC-сообщений
    if err := a.gRPCServer.Serve(l); err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }

    return nil
}

// Stop stops gRPC server.
func (a *App) Stop() {
    const op = "grpcapp.Stop"

    a.log.With(slog.String("op", op)).
        Info("stopping gRPC server", slog.Int("port", a.port))

    // Используем встроенный в gRPCServer механизм graceful shutdown
    a.gRPCServer.GracefulStop()
}
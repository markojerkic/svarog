package grpcserver

import (
	"context"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type contextKey string

const ClientIDKey contextKey = "clientID"

// wrappedServerStream wraps grpc.ServerStream to modify the context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

type AuthInterceptor interface {
	withInterceptor() grpc.UnaryServerInterceptor
	withStreamInterceptor() grpc.StreamServerInterceptor
}

type AuthInterceptorImpl struct {
}

// withStreamInterceptor implements AuthInterceptor.
func (a *AuthInterceptorImpl) withStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Wrap the server stream to inject the authenticated context
		ctx, err := a.interceptor(ss.Context())
		if err != nil {
			log.Error("Failed to authenticate client", "error", err)
			return status.Error(codes.Unauthenticated, "could not authenticate client")
		}

		// Create a wrapper stream that uses the new context
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Continue with the handler using the wrapped stream
		return handler(srv, wrappedStream)
	}
}

// withInterceptor implements AuthInterceptor.
func (a *AuthInterceptorImpl) withInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx, err = a.interceptor(ctx)

		if err != nil {
			log.Error("Failed to authenticate client", "error", err)
			return nil, status.Error(codes.Unauthenticated, "could not authenticate client")
		}

		return handler(ctx, req)
	}
}

func (a *AuthInterceptorImpl) interceptor(ctx context.Context) (context.Context, error) {
	// Get client certificate info from context
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no peer found")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no TLS info found")
	}

	if len(tlsInfo.State.VerifiedChains) == 0 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return nil, status.Error(codes.Unauthenticated, "could not verify client certificate")
	}

	// Extract client ID from certificate's Common Name
	clientID := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName

	log.Debug("Client connected", "clientID", clientID)

	ctx = context.WithValue(ctx, ClientIDKey, clientID)

	return ctx, nil
}

var _ AuthInterceptor = &AuthInterceptorImpl{}

func NewAuthInterceptor() AuthInterceptor {
	return &AuthInterceptorImpl{}
}

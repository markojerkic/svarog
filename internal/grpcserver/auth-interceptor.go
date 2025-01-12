package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type AuthInterceptor interface {
	withInterceptor() grpc.UnaryServerInterceptor
}

type AuthInterceptorImpl struct {
}

// withInterceptor implements AuthInterceptor.
func (a *AuthInterceptorImpl) withInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
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

		ctx = context.WithValue(ctx, "clientID", clientID)

		return handler(ctx, req)
	}
}

var _ AuthInterceptor = &AuthInterceptorImpl{}

func NewAuthInterceptor() AuthInterceptor {
	return &AuthInterceptorImpl{}
}

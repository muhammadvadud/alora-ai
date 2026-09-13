// Package backend — Django backend'ining ichki gRPC klientlari.
package backend

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

type requestIDKey struct{}

// WithRequestID gRPC metadata interceptoriga request ID berish uchun ishlatiladi.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func requestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		return firstMetadata(md, "x-request-id")
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		return firstMetadata(md, "x-request-id")
	}
	return ""
}

func firstMetadata(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func metadataContext(ctx context.Context, token string) context.Context {
	var pairs []string
	if token != "" {
		pairs = append(pairs, "x-service-token", token)
	}
	if id := requestID(ctx); id != "" {
		pairs = append(pairs, "x-request-id", id)
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// Dial ochiq ichki tarmoqdagi Django gRPC ulanishini yaratadi. mTLS keyingi
// bosqichga qoldirilgani uchun hozir insecure transport ishlatiladi (rules §8.2).
func Dial(ctx context.Context, addr, serviceToken string) (*grpc.ClientConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(metadataContext(ctx, serviceToken), method, req, reply, cc, opts...)
		}),
		grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(metadataContext(ctx, serviceToken), desc, cc, method, opts...)
		}),
	)
}

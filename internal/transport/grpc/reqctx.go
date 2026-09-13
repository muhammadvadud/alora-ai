package grpc

import "context"

// RequestInfo transportdan domen tool'lariga o'tadigan so'rov kontekstidir.
// curtains.WithUser hali mavjud emasligi sababli shu qatlamda saqlanadi.
type RequestInfo struct {
	UserID         string
	TenantID       string
	Role           string
	Locale         string
	CustomerMode   bool
	ConversationID string
}

type requestInfoKey struct{}

func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey{}, info)
}

func RequestInfoFrom(ctx context.Context) (RequestInfo, bool) {
	info, ok := ctx.Value(requestInfoKey{}).(RequestInfo)
	return info, ok
}

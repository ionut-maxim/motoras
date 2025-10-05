package server

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

type logContextKey struct{}

func newLogInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			var err error
			level := slog.LevelInfo
			start := time.Now()

			res, err := next(ctx, req)

			defer func() {
				end := time.Now()
				latency := time.Since(start)

				requestAttrs := []slog.Attr{
					slog.String("procedure", req.Spec().Procedure),
					slog.String("peer", req.Peer().Protocol+"://"+req.Peer().Addr+req.Peer().Query.Encode()),
					slog.Time("time", start),
				}
				responseAttrs := []slog.Attr{
					slog.Duration("latency", latency),
					slog.Time("time", end),
				}
				if slog.Default().Enabled(ctx, slog.LevelDebug) {
					// TODO: Do something with the log level
					requestAttrs = append(requestAttrs, slog.Any("payload", req.Any()))
					responseAttrs = append(responseAttrs, slog.Any("payload", res.Any()))
				}
				if err != nil {
					level = slog.LevelError
					responseAttrs = append(responseAttrs, slog.String("error", err.Error()))
				}

				attributes := []slog.Attr{
					{
						Key:   "request",
						Value: slog.GroupValue(requestAttrs...),
					},
					{
						Key:   "response",
						Value: slog.GroupValue(responseAttrs...),
					},
				}

				logger.LogAttrs(ctx, level, "Connect RPC call", attributes...)
			}()

			return res, err
		}
	}
}

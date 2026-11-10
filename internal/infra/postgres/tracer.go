package postgres

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

var collectionPattern = regexp.MustCompile(`(?i)\b(?:FROM|INTO|UPDATE|JOIN)\s+([a-z_][a-z0-9_]*)`)

type queryInfoKey struct{}

type queryInfo struct {
	started    time.Time
	operation  string
	collection string
}

// queryTracer keeps otelpgx's connection and query spans while enriching queries
// with the bounded operation and table labels required by the service.
type queryTracer struct {
	*otelpgx.Tracer
	duration metric.Float64Histogram
}

func newQueryTracer() (*queryTracer, error) {
	duration, err := otel.Meter("tripgo/postgres").Float64Histogram("db.client.operation.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	return &queryTracer{
		Tracer: otelpgx.NewTracer(
			otelpgx.WithDisableSQLStatementInAttributes(),
			otelpgx.WithMeterProvider(metricnoop.NewMeterProvider()),
		),
		duration: duration,
	}, nil
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	info := queryInfo{started: time.Now(), operation: sqlOperation(data.SQL), collection: sqlCollection(data.SQL)}
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("db.operation.name", info.operation),
		attribute.String("db.collection.name", info.collection),
	)
	return context.WithValue(ctx, queryInfoKey{}, info)
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if info, ok := ctx.Value(queryInfoKey{}).(queryInfo); ok {
		errorType := ""
		if data.Err != nil {
			errorType = databaseErrorType(data.Err)
			span := trace.SpanFromContext(ctx)
			span.SetAttributes(attribute.String("error.type", errorType))
			span.SetStatus(codes.Error, data.Err.Error())
		}
		t.duration.Record(ctx, time.Since(info.started).Seconds(), metric.WithAttributes(
			attribute.String("db.operation.name", info.operation),
			attribute.String("db.collection.name", info.collection),
			attribute.String("error.type", errorType),
		))
	}
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

func sqlOperation(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return "UNKNOWN"
	}
	return strings.ToUpper(fields[0])
}

func sqlCollection(sql string) string {
	if match := collectionPattern.FindStringSubmatch(sql); len(match) == 2 {
		return strings.ToLower(match[1])
	}
	return "unknown"
}

func databaseErrorType(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return reflect.TypeOf(err).String()
}

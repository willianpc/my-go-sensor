package instatrace

import (
	"context"
	"sync"

	instana "github.com/instana/go-sensor"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"go.opencensus.io/trace"
)

type ocSpanContextKey struct {
	TraceID trace.TraceID
	SpanID  trace.SpanID
}

// Exporter is an go.opencensus.io/trace.Exporter that listens for OpenCensus spans
// and attaches them to Instana traces
type Exporter struct {
	sensor   *instana.Sensor
	mu       sync.RWMutex
	ocTraces map[ocSpanContextKey]instana.SpanContext
}

// NewExporter initializes a new opencensus.Exporter
func NewExporter(sensor *instana.Sensor) *Exporter {
	return &Exporter{
		sensor:   sensor,
		ocTraces: make(map[ocSpanContextKey]instana.SpanContext),
	}
}

// ExportSpan implements go.opencensus.io/trace.Exporter for Exporter
func (exp *Exporter) ExportSpan(s *trace.SpanData) {
	k := ocSpanContextKey{s.TraceID, s.ParentSpanID}

	exp.mu.RLock()
	spCtx, ok := exp.ocTraces[k]
	exp.mu.RUnlock()

	if !ok {
		return
	}

	exp.sensor.Logger().Debug(
		"mapping OpenCensus span ", s.Name,
		" (traceID: ", s.TraceID.String(), ", ",
		"spanID: ", s.SpanID.String(), ") ",
		" to Instana trace ", instana.FormatID(spCtx.TraceID),
	)

	exp.mu.Lock()
	delete(exp.ocTraces, k)
	exp.mu.Unlock()

	exp.sensor.Tracer().StartSpan(
		s.Name,
		ext.SpanKindRPCClient,
		opentracing.ChildOf(spCtx),
		opentracing.StartTime(s.StartTime),
		opentracing.Tags(s.Attributes),
	).FinishWithOptions(
		opentracing.FinishOptions{
			FinishTime: s.EndTime,
		},
	)
}

// Context starts a new OpenCensus span and injects it into provided context. This
// span is than used to correlate the OpenCensus trace with Instana
func (exp *Exporter) Context(ctx context.Context) (context.Context, *trace.Span) {
	ctx, ocSpan := trace.StartSpan(
		ctx,
		"github.com/instana/go-sensor/instrumentation/gcp/instastorage.Context",
		trace.WithSampler(trace.AlwaysSample()),
	)

	if sp, ok := instana.SpanFromContext(ctx); ok {
		exp.mu.Lock()
		exp.ocTraces[ocSpanContextKey{ocSpan.SpanContext().TraceID, ocSpan.SpanContext().SpanID}] = sp.Context().(instana.SpanContext)
		exp.mu.Unlock()
	}

	return ctx, ocSpan
}

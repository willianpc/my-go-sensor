package instatrace

import (
	"context"
	"sync"
	"time"

	instana "github.com/instana/go-sensor"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"go.opencensus.io/trace"
)

type ocSpanContextKey struct {
	TraceID trace.TraceID
	SpanID  trace.SpanID
}

// ExporterOptions contains configuration for Exporter
type MapperOptions struct {
	// The maximum time to keep OpenCensus spans that cannot be mapped to Instana trace in cache.
	// If an exit span for a trace does not arrive within this period of time, it will be discarded.
	// By default the exported does not discard any spans.
	MaxTraceDuration time.Duration
}

// Mapper is an go.opencensus.io/trace.Exporter that listens for OpenCensus spans
// and maps them to Instana traces
type Mapper struct {
	sensor   *instana.Sensor
	mu       sync.RWMutex
	ocTraces map[ocSpanContextKey]instana.SpanContext
	unmapped *ttlSpanDataCache
}

// NewMapper initializes a new opencensus.Mapper
func NewMapper(sensor *instana.Sensor, opts MapperOptions) *Mapper {
	ttlCache := newTTLSpanDataCache(opts.MaxTraceDuration)
	if opts.MaxTraceDuration > 0 {
		go ttlCache.Cleanup(context.Background())
	}

	return &Mapper{
		sensor:   sensor,
		ocTraces: make(map[ocSpanContextKey]instana.SpanContext),
		unmapped: ttlCache,
	}
}

// ExportSpan implements go.opencensus.io/trace.Exporter for Exporter
func (exp *Mapper) ExportSpan(s *trace.SpanData) {
	k := ocSpanContextKey{s.TraceID, s.ParentSpanID}

	exp.mu.RLock()
	spCtx, ok := exp.ocTraces[k]
	exp.mu.RUnlock()

	if !ok {
		exp.sensor.Logger().Debug("enqueueing ", spanDataToString(s), " to consider it later")
		exp.unmapped.Put(k, s)

		return
	}

	exp.sensor.Logger().Debug(
		"mapping OpenCensus span ", spanDataToString(s), " to Instana trace ", instana.FormatID(spCtx.TraceID),
	)

	exp.mu.Lock()
	delete(exp.ocTraces, k)
	exp.mu.Unlock()

	tags := make(opentracing.Tags, len(s.Attributes)+1)
	for k, v := range s.Attributes {
		tags[k] = v
	}
	tags[string(ext.SpanKind)] = convertSpanKind(s.SpanKind)

	sp := exp.sensor.Tracer().StartSpan(
		s.Name,
		opentracing.ChildOf(spCtx),
		opentracing.StartTime(s.StartTime),
		tags,
	)
	sp.FinishWithOptions(opentracing.FinishOptions{FinishTime: s.EndTime})

	// store the non-exit span mapping and wait for the exit span
	if s.SpanKind != trace.SpanKindClient {
		exp.mapOCTrace(s.TraceID, s.SpanID, sp.Context().(instana.SpanContext))
	}

	if queued := exp.unmapped.Fetch(ocSpanContextKey{s.TraceID, s.SpanID}); len(queued) > 0 {
		exp.sensor.Logger().Debug("found ", len(queued), " OpenCensus span(s) stored for later consideration")
		for _, s := range queued {
			exp.ExportSpan(s)
		}
	}
}

// Context starts a new OpenCensus span and injects it into provided context. This
// span is than used to correlate the OpenCensus trace with Instana
func (exp *Mapper) Context(ctx context.Context) (context.Context, *trace.Span) {
	ctx, ocSpan := trace.StartSpan(
		ctx,
		"github.com/instana/go-sensor/instrumentation/gcp/instastorage.Context",
		trace.WithSampler(trace.AlwaysSample()),
	)

	if sp, ok := instana.SpanFromContext(ctx); ok {
		exp.mapOCTrace(ocSpan.SpanContext().TraceID, ocSpan.SpanContext().SpanID, sp.Context().(instana.SpanContext))
	}

	return ctx, ocSpan
}

func (exp *Mapper) mapOCTrace(traceID trace.TraceID, spanID trace.SpanID, spCtx instana.SpanContext) {
	exp.mu.Lock()
	defer exp.mu.Unlock()

	exp.ocTraces[ocSpanContextKey{traceID, spanID}] = spCtx
}

func convertSpanKind(ocSpanKind int) ext.SpanKindEnum {
	switch ocSpanKind {
	case trace.SpanKindClient:
		return ext.SpanKindRPCClientEnum
	case trace.SpanKindServer:
		return ext.SpanKindRPCServerEnum
	default:
		return "intermediate"
	}
}

func spanDataToString(s *trace.SpanData) string {
	return s.Name + " (traceID: " + s.TraceID.String() + ", spanID: " + s.SpanID.String() + ")"
}

type spanDataCacheEntry struct {
	items     []*trace.SpanData
	expiresAt time.Time
}

type ttlSpanDataCache struct {
	ttl     time.Duration
	mu      sync.Mutex
	entries map[ocSpanContextKey]spanDataCacheEntry
}

func newTTLSpanDataCache(ttl time.Duration) *ttlSpanDataCache {
	return &ttlSpanDataCache{
		ttl:     ttl,
		entries: make(map[ocSpanContextKey]spanDataCacheEntry),
	}
}

func (c *ttlSpanDataCache) Put(k ocSpanContextKey, s *trace.SpanData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[k] = spanDataCacheEntry{
		items:     append(c.entries[k].items, s),
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *ttlSpanDataCache) Fetch(k ocSpanContextKey) []*trace.SpanData {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entries, ok := c.entries[k]; ok {
		delete(c.entries, k)
		return entries.items
	}

	return nil
}

func (c *ttlSpanDataCache) Cleanup(ctx context.Context) {
	t := time.NewTicker(c.ttl / 2)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			c.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

func (c *ttlSpanDataCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, v := range c.entries {
		if !v.expiresAt.After(now) {
			delete(c.entries, k)
		}
	}
}

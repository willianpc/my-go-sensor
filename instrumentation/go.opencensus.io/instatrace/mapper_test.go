package instatrace_test

import (
	"context"
	"testing"
	"time"

	instana "github.com/instana/go-sensor"
	"github.com/instana/go-sensor/instrumentation/go.opencensus.io/instatrace"
	"github.com/instana/testify/assert"
	"github.com/instana/testify/require"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"go.opencensus.io/trace"
)

func TestMapper_Context(t *testing.T) {
	s := instana.NewSensor("testing")
	exp := instatrace.NewMapper(s)

	ctx, sp := exp.Context(context.Background())

	assert.True(t, sp.SpanContext().IsSampled())
	assert.Equal(t, sp, trace.FromContext(ctx))
}

func TestMapper_ExportSpan(t *testing.T) {
	rec := instana.NewTestRecorder()

	s := instana.NewSensorWithTracer(instana.NewTracerWithEverything(instana.DefaultOptions(), rec))
	exp := instatrace.NewMapper(s)

	trace.RegisterExporter(exp)
	defer trace.UnregisterExporter(exp)

	parent := s.Tracer().StartSpan("entry")
	ctx, _ := exp.Context(instana.ContextWithSpan(context.Background(), parent))

	_, ocSpan := trace.StartSpan(ctx, "opencensus", trace.WithSpanKind(trace.SpanKindClient))
	ocSpan.AddAttributes(
		trace.StringAttribute("stringKey", "value"),
		trace.Int64Attribute("answer", 42),
	)
	ocSpan.End()

	parent.Finish()

	spans := rec.GetQueuedSpans()
	require.Len(t, spans, 2)

	sp, parentSp := spans[0], spans[1]

	assert.Equal(t, parentSp.TraceID, sp.TraceID)
	assert.Equal(t, parentSp.SpanID, sp.ParentID)

	assert.Equal(t, "sdk", sp.Name)
	assert.Equal(t, 2, sp.Kind)
	assert.Equal(t, 0, sp.Ec)

	require.IsType(t, instana.SDKSpanData{}, sp.Data)
	spanData := sp.Data.(instana.SDKSpanData)

	assert.Equal(t, map[string]interface{}{
		"tags": opentracing.Tags{
			"span.kind": ext.SpanKindRPCClientEnum,
			"stringKey": "value",
			"answer":    int64(42),
		},
	}, spanData.Tags.Custom)
}

func TestMapper_ExportSpan_MultiSpanTrace(t *testing.T) {
	rec := instana.NewTestRecorder()

	s := instana.NewSensorWithTracer(instana.NewTracerWithEverything(instana.DefaultOptions(), rec))
	exp := instatrace.NewMapper(s)

	trace.RegisterExporter(exp)
	defer trace.UnregisterExporter(exp)

	parent := s.Tracer().StartSpan("entry")
	ctx, _ := exp.Context(instana.ContextWithSpan(context.Background(), parent))

	ctx, intermOCSpan := trace.StartSpan(ctx, "opencensus_intermediate")
	_, ocSpan := trace.StartSpan(ctx, "opencensus_exit", trace.WithSpanKind(trace.SpanKindClient))

	ocSpan.End()
	intermOCSpan.End()
	parent.Finish()

	spans := rec.GetQueuedSpans()
	require.Len(t, spans, 3)

	entrySpan, intermSpan, exitSpan := spans[2], spans[0], spans[1]

	assert.Equal(t, entrySpan.TraceID, intermSpan.TraceID)
	assert.Equal(t, intermSpan.TraceID, exitSpan.TraceID)

	assert.Equal(t, entrySpan.SpanID, intermSpan.ParentID)
	assert.Equal(t, intermSpan.SpanID, exitSpan.ParentID)
}

func TestMapper_ExportSpan_MultiSpanTrace_LongRunningTrace(t *testing.T) {
	rec := instana.NewTestRecorder()

	s := instana.NewSensorWithTracer(instana.NewTracerWithEverything(instana.DefaultOptions(), rec))
	exp := instatrace.NewMapperWithOptions(s, instatrace.MapperOptions{
		MaxTraceDuration: 10 * time.Millisecond,
	})

	trace.RegisterExporter(exp)
	defer trace.UnregisterExporter(exp)

	parent := s.Tracer().StartSpan("entry")
	ctx, _ := exp.Context(instana.ContextWithSpan(context.Background(), parent))

	ctx, intermOCSpan := trace.StartSpan(ctx, "opencensus_intermediate")
	_, ocSpan := trace.StartSpan(ctx, "opencensus_exit", trace.WithSpanKind(trace.SpanKindClient))

	ocSpan.End()

	time.Sleep(50 * time.Millisecond) // let the exported cache expire
	intermOCSpan.End()

	parent.Finish()

	spans := rec.GetQueuedSpans()
	require.Len(t, spans, 2)

	entrySpan, intermSpan := spans[1], spans[0]

	assert.Equal(t, entrySpan.TraceID, intermSpan.TraceID)
	assert.Equal(t, entrySpan.SpanID, intermSpan.ParentID)
}

func TestMapper_ExportSpan_NoInstanaTrace(t *testing.T) {
	rec := instana.NewTestRecorder()

	s := instana.NewSensorWithTracer(instana.NewTracerWithEverything(instana.DefaultOptions(), rec))
	exp := instatrace.NewMapper(s)

	trace.RegisterExporter(exp)
	defer trace.UnregisterExporter(exp)

	_, ocSpan := trace.StartSpan(context.Background(), "opencensus")
	ocSpan.End()

	assert.Empty(t, rec.GetQueuedSpans())
}

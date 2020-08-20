package instatrace_test

import (
	"context"
	"testing"

	instana "github.com/instana/go-sensor"
	"github.com/instana/go-sensor/instrumentation/go.opencensus.io/instatrace"
	"github.com/instana/testify/assert"
	"github.com/instana/testify/require"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"go.opencensus.io/trace"
)

func TestExporter_Context(t *testing.T) {
	s := instana.NewSensor("testing")
	exp := instatrace.NewExporter(s)

	ctx, sp := exp.Context(context.Background())

	assert.True(t, sp.SpanContext().IsSampled())
	assert.Equal(t, sp, trace.FromContext(ctx))
}

func TestExporter_ExportSpan(t *testing.T) {
	rec := instana.NewTestRecorder()

	s := instana.NewSensorWithTracer(instana.NewTracerWithEverything(instana.DefaultOptions(), rec))
	exp := instatrace.NewExporter(s)

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

func TestExporter_ExportSpan_NoInstanaTrace(t *testing.T) {
	rec := instana.NewTestRecorder()

	s := instana.NewSensorWithTracer(instana.NewTracerWithEverything(instana.DefaultOptions(), rec))
	exp := instatrace.NewExporter(s)

	trace.RegisterExporter(exp)
	defer trace.UnregisterExporter(exp)

	_, ocSpan := trace.StartSpan(context.Background(), "opencensus")
	ocSpan.End()

	assert.Empty(t, rec.GetQueuedSpans())
}

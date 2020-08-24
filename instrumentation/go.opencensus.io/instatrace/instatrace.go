/*
Package instatrace provides Instana trace continuation by mapping the spans created within code instrumented with OpenCensus
onto ongoing Instana traces.

To map OpenCensus traces to Instana ones first register an exporter (this is usually done in your main() function, close to
where Instana sensor is initialized:

    sensor := instana.NewSensor("my-opencensus-service")
    trace.RegisterExporter(instatrace.NewMapper(sensor))

To correlate traces, the instatrace.Mapper needs an entry point to the code instrumented with OpenCensus. (*instatrace.Mapper).Context()
checks whether there is an active OpenCensus trace present in provided context and maps this trace to an ongoing Instana
trace stored within the same context. If there is no active OpenCensus trace yet, (*instatrace.Mapper).Context() initiates one:

    func InstanaInstrumentedMethod(exporter *instatrace.Mapper) {
        // Initialize and inject an Instana trace into context.Context. This is usually already done for you
	// by an instrumentation wrapper provided with github.com/instana/go-sensor
        sp := sensor.Tracer().StartSpan("entry")
        defer sp.Finish()

        ctx := instana.ContextWithSpan(context.Background(), sp)

        // Mark the entry point into the OpenCensus-instrumented code block to correlate its spans with Instana trace
	ctx, _ := exporter.Context(ctx)
	OpenCensusInstrumentedMethod(ctx)
        // ...
    }

Once this is done, the exporter will send an Instana span for every OpenCensus span created from within this context, and attribute
them to the same Instana trace.
*/
package instatrace

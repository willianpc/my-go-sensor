Instana Go sensor integration for OpenCensus
============================================

[![GoDoc](https://img.shields.io/static/v1?label=godoc&message=reference&color=blue)][godoc]

This module provides following instrumentation libraries that integrate existing [OpenCensus](https://opencensus.io/) instrumentations
that use [`go.opencensus.io/...`](https://pkg.go.dev/go.opencensus.io/trace?tab=doc) with Instana:

* [`instatrace`](./instatrace) provides a trace exporter that maps [OpenCensus](https://opencensus.io/) spans to Instana traces.

[godoc]: https://pkg.go.dev/github.com/instana/go-sensor/instrumentation/go.opencensus.io

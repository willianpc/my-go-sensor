// (c) Copyright IBM Corp. 2022
// (c) Copyright Instana Inc. 2022

package instaazurefunction

import (
	"fmt"
	instana "github.com/instana/go-sensor"
	"github.com/instana/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBasicHttpGet(t *testing.T) {
	recorder := instana.NewTestRecorder()
	sensor := instana.NewSensorWithTracer(
		instana.NewTracerWithEverything(instana.DefaultOptions(), recorder))

	h := WrapFunctionHandler(sensor, func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprintln(writer, "Ok")
	})

	bodyReader := strings.NewReader(`{
	  "Metadata": {
		"Headers": {
		  "User-Agent": "curl/7.79.1"
		},
		"sys": {
		  "MethodName": "products"
		}
	  }
	}`)
	req := httptest.NewRequest(http.MethodGet, "/test", bodyReader)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	spans := recorder.GetQueuedSpans()

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Ok\n", rec.Body.String())
	assert.Equal(t, 1, len(spans))

	azSpan := spans[0]
	data := azSpan.Data.(instana.AzureFunctionSpanData)

	assert.Equal(t, "products", data.Tags.MethodName)
	assert.Equal(t, "custom", data.Tags.Runtime)
	assert.Equal(t, httpTrigger, data.Tags.Trigger)

}

package instaazurefunction

import (
	"context"

	instana "github.com/instana/go-sensor"
)

type azureTracer struct {
	sensor *instana.Sensor
}

func (a *azureTracer) DetectSpanDataFromContext(c *context.Context) {

}

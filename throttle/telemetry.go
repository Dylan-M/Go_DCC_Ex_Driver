package throttle

import "go.opentelemetry.io/otel/attribute"

// operation keeps the trace context inside the single-owner controller. Delayed
// speed writes retain it separately so their spans link to the original action.
func (c *Controller) operation(name string, attrs ...attribute.KeyValue) func(error) {
	previous := c.operationContext
	// Explicit target attributes win over the selected tab's default address.
	attributes := attribute.NewSet(append([]attribute.KeyValue{attribute.Int("loco.address", c.state.Cab)}, attrs...)...)
	attrs = attributes.ToSlice()
	ctx, finish := c.telemetry.Start(previous, "throttle."+name, attrs...)
	c.operationContext = ctx
	c.telemetry.Event(ctx, "throttle.action", append(attrs, attribute.String("action", name))...)
	return func(err error) { finish(err); c.operationContext = previous }
}

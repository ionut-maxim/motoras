# Trigger Service Metrics

The trigger service exposes OpenTelemetry metrics to help monitor the health and performance of the event processing
pipeline.

## Available Metrics

### `trigger.event_channel.size` (Gauge)

**Description**: Current number of events buffered in the trigger event channel.

**Unit**: `{events}`

**Usage**: This gauge provides real-time visibility into the event channel backlog. High values indicate back pressure -
the workflow service is not consuming events as fast as they are being produced.

**Load Balancing Implications**:

- **Low values (< 30% of buffer)**: System is healthy, events are being processed quickly
- **Medium values (30-70% of buffer)**: Monitor closely, consider scaling workflow processing
- **High values (> 70% of buffer)**: Back pressure detected, scale out or optimize workflow processing

The default event channel buffer size is 100 events (configurable via `WithEventBufferSize` option).

### `trigger.events.processed` (Counter)

**Description**: Total number of events sent to the event channel for processing.

**Unit**: `{events}`

**Usage**: This counter tracks the total throughput of the trigger service. Use it to:

- Monitor event processing rate over time
- Detect anomalies in trigger activity
- Calculate average events per second

## Configuration

### Using Default Global Meter

By default, the trigger service uses the global OpenTelemetry meter:

```go
triggerService := trigger.New(
    trigger.WithLogger(logger),
    trigger.WithStore(store),
)
```

### Using Custom Meter Provider

To use a specific meter provider (e.g., for testing or custom exporters):

```go
import (
    "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/exporters/prometheus"
)

// Create a Prometheus exporter
exporter, err := prometheus.New()
if err != nil {
    log.Fatal(err)
}

// Create meter provider
meterProvider := metric.NewMeterProvider(
    metric.WithReader(exporter),
)

// Pass to trigger service
triggerService := trigger.New(
    trigger.WithLogger(logger),
    trigger.WithStore(store),
    trigger.WithMeterProvider(meterProvider),
)
```

## Example: Prometheus Scraping

With a Prometheus exporter configured, the metrics will be available at your metrics endpoint:

```
# HELP trigger_event_channel_size Current number of events in the trigger event channel
# TYPE trigger_event_channel_size gauge
trigger_event_channel_size{} 42

# HELP trigger_events_processed Total number of events processed by the trigger service
# TYPE trigger_events_processed counter
trigger_events_processed{} 1547
```

## Alerting Recommendations

### High Back Pressure Alert

```yaml
- alert: TriggerServiceBackPressure
  expr: trigger_event_channel_size > 70
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Trigger service experiencing back pressure"
    description: "Event channel has {{ $value }} events buffered (>70% of capacity)"
```

### Channel Full Alert

```yaml
- alert: TriggerServiceChannelFull
  expr: trigger_event_channel_size >= 100
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Trigger service event channel is full"
    description: "Event channel is at maximum capacity, events may be blocked"
```

## Troubleshooting

### High Channel Size

If `trigger.event_channel.size` is consistently high:

1. **Check workflow processing time**: Slow workflows will cause back pressure
2. **Scale horizontally**: Add more instances to distribute load
3. **Increase buffer size**: Use `WithEventBufferSize()` to increase capacity (temporary solution)
4. **Optimize workflows**: Profile and optimize slow workflow steps

### Low Event Processing Rate

If `trigger.events.processed` is lower than expected:

1. **Check trigger configuration**: Verify triggers are configured correctly
2. **Check subscriber logs**: Look for errors in trigger subscriber implementations
3. **Check distributed locks**: Ensure locks are being acquired successfully

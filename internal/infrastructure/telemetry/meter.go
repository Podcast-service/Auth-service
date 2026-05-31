package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const runtimeMetricsInterval = 15 * time.Second

func InitMeter(ctx context.Context) (*sdkmetric.MeterProvider, error) {
	endpoint := endpointFromEnv()
	serviceName := serviceNameFromEnv()

	exporterCtx, cancel := context.WithTimeout(ctx, exporterTimeout)
	defer cancel()

	exporter, err := otlpmetricgrpc.New(exporterCtx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create otlp metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)

	otel.SetMeterProvider(mp)

	if err := runtime.Start(
		runtime.WithMeterProvider(mp),
		runtime.WithMinimumReadMemStatsInterval(runtimeMetricsInterval),
	); err != nil {
		return nil, fmt.Errorf("start runtime metrics: %w", err)
	}

	return mp, nil
}

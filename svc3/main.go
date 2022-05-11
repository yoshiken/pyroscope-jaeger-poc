package main

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/pyroscope-io/client/pyroscope"
	"github.com/pyroscope-io/otelpyroscope"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"log"
	"net/http"
	"github.com/google/go-github/github"
	//"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	//"os"
	"fmt"
)

var tracer trace.Tracer

func callGhOrgs(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "callGhOrgs")
	defer span.End()
	client := github.NewClient(nil)
	orgs, _, err := client.Organizations.List(context.Background(), "yoshiken", nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	for i, organization := range orgs {
		fmt.Printf("%v. %v\n", i+1, organization.GetLogin())
	}
	primeNumber(ctx)
}

func primeNumber(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "primeNumber")
	defer span.End()
	for i := 2; i < 100; i++ {
		if isPrime(i) {
			fmt.Printf("%v is prime\n", i)
		}
	}
}

func isPrime(n int) bool {
	for i := 2; i < n; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}

func main() {
	/*
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint(),
	stdouttrace.WithWriter(os.Stderr),
	)
	*/
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint("http://localhost:14268/api/traces"),
	))
	if err != nil {
		return
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(otelpyroscope.NewTracerProvider(tp,
		otelpyroscope.WithAppName("svc4"),
		otelpyroscope.WithPyroscopeURL("http://localhost:4040"),
		otelpyroscope.WithRootSpanOnly(true),
		otelpyroscope.WithAddSpanName(true),
		otelpyroscope.WithProfileURL(true),
		otelpyroscope.WithProfileBaselineURL(true),
	))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()
	tracer = tp.Tracer("svc4")

	p, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "svc4",
		ServerAddress:   "http://localhost:4040",
		Logger:          pyroscope.StandardLogger,
	})
	if err != nil {
		pyroscope.StandardLogger.Errorf(err.Error())
	}
	defer func(p *pyroscope.Profiler) {
		_ = p.Stop()
	}(p)

	e := echo.New()
	e.Use(otelecho.Middleware("svc4"))
	e.GET("/greeting", func(c echo.Context) error {
		_, span := tracer.Start(c.Request().Context(), "handler")

		callGhOrgs(c.Request().Context())
		defer span.End()
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.Logger.Fatal(e.Start(":1324"))
}

// Package telemetry wires OpenTelemetry tracing for gitte and exports spans to
// an OTLP/HTTP endpoint (e.g. Elastic APM). It is config-driven and degrades to
// a no-op whenever telemetry is disabled or setup fails, so it never blocks or
// slows gitte.
package telemetry

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/cego/gitte/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlplog "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/cego/gitte"

// flushTimeout bounds how long exit can block flushing spans. Kept short so an
// enabled-but-unreachable endpoint (e.g. laptop with the VPN off) adds at most
// this delay to every command.
const flushTimeout = 1 * time.Second

// Resolved is the outcome of resolving telemetry settings from config + env.
type Resolved struct {
	Enabled   bool
	Endpoint  string            // explicit endpoint to export to; empty when UseSDKEnv
	Headers   map[string]string // export headers (auth, etc.)
	UseSDKEnv bool              // enable from standard OTEL_* env; let the SDK read its own config
}

// Resolve computes telemetry settings. Precedence:
// GITTE_TELEMETRY=off > GITTE_TELEMETRY_URL > config endpoint > OTEL_EXPORTER_OTLP_* env.
func Resolve(cfg *config.GitteConfig) Resolved {
	if strings.EqualFold(os.Getenv("GITTE_TELEMETRY"), "off") {
		return Resolved{}
	}

	endpoint := os.Getenv("GITTE_TELEMETRY_URL")
	if endpoint == "" && cfg != nil {
		endpoint = cfg.Telemetry.Endpoint
	}
	if endpoint != "" {
		// Headers always come from config, even when GITTE_TELEMETRY_URL overrides
		// the endpoint. This is a known v1 limitation: if you need different headers
		// for an override endpoint, use the standard OTEL_EXPORTER_OTLP_* env path
		// instead (which lets the SDK read its own config independently).
		headers := map[string]string{}
		if cfg != nil {
			for k, v := range cfg.Telemetry.Headers {
				headers[k] = v
			}
		}
		return Resolved{Enabled: true, Endpoint: endpoint, Headers: headers}
	}

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" {
		return Resolved{Enabled: true, UseSDKEnv: true}
	}

	return Resolved{}
}

// noopErrorHandler swallows OTEL-internal errors (e.g. export failures) so they
// never reach the user or interfere with gitte.
type noopErrorHandler struct{}

func (noopErrorHandler) Handle(error) {}

// debugErrorHandler logs OTEL-internal errors to stderr. Enabled via
// GITTE_TELEMETRY_DEBUG so export failures (auth, redirects, connectivity) are
// visible when diagnosing why traces aren't arriving — otherwise they are
// silently swallowed.
type debugErrorHandler struct{}

func (debugErrorHandler) Handle(err error) {
	fmt.Fprintf(os.Stderr, "[telemetry] %v\n", err)
}

// resourceAttributes builds the resource attributes attached to every span.
// username and hostname identify which developer and machine produced the
// trace (the primary signal for debugging machine-specific failures); both are
// best-effort and omitted when they cannot be resolved.
func resourceAttributes(version, username, hostname string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", "gitte"),
		attribute.String("service.version", version),
		attribute.String("os.type", runtime.GOOS),
		attribute.String("os.arch", runtime.GOARCH),
	}
	if username != "" {
		attrs = append(attrs, attribute.String("user.name", username))
	}
	if hostname != "" {
		attrs = append(attrs, attribute.String("host.name", hostname))
	}
	return attrs
}

// Init configures the global tracer provider and returns a shutdown function
// that flushes pending spans with a bounded timeout. The returned function is
// always non-nil and safe to call; setup failures and disabled telemetry both
// degrade to a no-op shutdown.
func Init(ctx context.Context, cfg *config.GitteConfig, version string) func() {
	r := Resolve(cfg)
	if !r.Enabled {
		return func() {}
	}

	// Only mutate process-wide OTEL state once telemetry is known to be enabled.
	// GITTE_TELEMETRY_DEBUG surfaces export errors to stderr for diagnostics.
	if os.Getenv("GITTE_TELEMETRY_DEBUG") != "" {
		otel.SetErrorHandler(debugErrorHandler{})
	} else {
		otel.SetErrorHandler(noopErrorHandler{})
	}

	var opts []otlptracehttp.Option
	if !r.UseSDKEnv {
		opts = append(opts, otlptracehttp.WithEndpointURL(r.Endpoint))
		if len(r.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(r.Headers))
		}
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		// Never block gitte: disable telemetry on exporter setup failure.
		return func() {}
	}

	username := ""
	if u, uerr := user.Current(); uerr == nil {
		username = u.Username
	}
	hostname, _ := os.Hostname()
	res := resource.NewSchemaless(resourceAttributes(version, username, hostname)...)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	var lp *sdklog.LoggerProvider
	if logsEnabled() {
		var logOpts []otlplog.Option
		if !r.UseSDKEnv {
			logOpts = append(logOpts, otlplog.WithEndpointURL(logsEndpointURL(r.Endpoint)))
			if len(r.Headers) > 0 {
				logOpts = append(logOpts, otlplog.WithHeaders(r.Headers))
			}
		}
		if logExp, lerr := otlplog.New(ctx, logOpts...); lerr == nil {
			lp = sdklog.NewLoggerProvider(
				sdklog.WithResource(res),
				sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
			)
			otellog.SetLoggerProvider(lp)
		}
	}

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), flushTimeout)
		defer cancel()
		_ = tp.Shutdown(shutdownCtx)
		if lp != nil {
			_ = lp.Shutdown(shutdownCtx)
		}
	}
}

// logsEnabled reports whether OTEL logs should be exported (enabled with tracing
// unless GITTE_TELEMETRY_LOGS=off).
func logsEnabled() bool {
	return !strings.EqualFold(os.Getenv("GITTE_TELEMETRY_LOGS"), "off")
}

// logsEndpointURL returns the OTLP/HTTP logs endpoint for a configured base
// endpoint. otlploghttp.WithEndpointURL uses the URL's path verbatim, so a
// path-less endpoint (e.g. "https://apm.example.com") would POST to the server
// root and be rejected. When the endpoint has no path we append the standard
// "/v1/logs" intake path (matching how the traces exporter targets
// "/v1/traces"); an endpoint that already carries a path is left untouched.
func logsEndpointURL(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Path == "" || u.Path == "/" {
		return strings.TrimRight(endpoint, "/") + "/v1/logs"
	}
	return endpoint
}

// Tracer returns gitte's tracer from the global provider (a no-op tracer when
// telemetry is disabled).
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartPhaseSpan starts a span for a gitte run phase (startup/gitops/actions)
// and returns the derived context to thread into that phase's work.
func StartPhaseSpan(ctx context.Context, phase string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, phase)
}

// StartCommandSpan starts the root span for a gitte invocation.
func StartCommandSpan(ctx context.Context, commandPath string, args []string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, commandPath)
	span.SetAttributes(
		attribute.String("gitte.command", commandPath),
		attribute.StringSlice("gitte.args", args),
	)
	return ctx, span
}

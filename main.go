package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	_ "cloud.google.com/go/storage"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

var (
	// The task latency in milliseconds.
	latencyS = stats.Float64("repl/start_time", "start time in seconds", "s")
)

func main() {
	osMethod, _ := tag.NewKey("os")
	driverMethod, _ := tag.NewKey("driver")

	ctx, _ := tag.New(context.Background(), tag.Insert(osMethod, runtime.GOOS), tag.Insert(driverMethod, "docker"))

	// Register the view. It is imperative that this step exists,
	// otherwise recorded metrics will be dropped and never exported.
	v := &view.View{
		Name:        "minikube_performance_trace",
		Measure:     latencyS,
		Description: "Minikube start time",
		Aggregation: view.LastValue(),
	}
	if err := view.Register(v); err != nil {
		log.Fatalf("Failed to register the view: %v", err)
	}

	sd, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID: "priya-wadhwa",
		// MetricPrefix helps uniquely identify your metrics.
		MetricPrefix: "minikube_performance_trace",
		// ReportingInterval sets the frequency of reporting metrics
		// to stackdriver backend.
		ReportingInterval: 1 * time.Minute,
	})
	if err != nil {
		log.Fatalf("Failed to create the Stackdriver exporter: %v", err)
	}
	// It is imperative to invoke flush before your main function exits
	defer sd.Flush()

	// Register it as a trace exporter
	trace.RegisterExporter(sd)

	if err := sd.StartMetricsExporter(); err != nil {
		log.Fatalf("Error starting metric exporter: %v", err)
	}
	defer sd.StopMetricsExporter()

	// Record 2p0 start time values
	for {
		st := minikubeStartTime(ctx)
		fmt.Printf("Latency: %f\n", st)
		stats.Record(ctx, latencyS.M(st))
		time.Sleep(30 * time.Second)
	}
}

func minikubeStartTime(ctx context.Context) float64 {
	minikube := filepath.Join(os.Getenv("HOME"), "minikube/out/minikube")
	profile := "cloud-monitoring"
	defer func() {
		cmd := exec.CommandContext(ctx, minikube, "delete", "-p", profile)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("error deleting: %v", err)
		}
	}()
	log.Print("Running minikube start....")

	t := time.Now()
	cmd := exec.CommandContext(ctx, minikube, "start", "--driver=docker", "-p", profile)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	return time.Since(t).Seconds()
}

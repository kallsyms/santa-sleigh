package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kallsyms/santa-sleigh/internal/config"
	"github.com/kallsyms/santa-sleigh/internal/daemon"
	"github.com/kallsyms/santa-sleigh/internal/logging"
	"github.com/kallsyms/santa-sleigh/internal/uploader"
)

func main() {
	var cfgPath string
	var showVersion bool
	flag.StringVar(&cfgPath, "config", "", "Path to santa-sleigh configuration file")
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(daemon.Version())
		return
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatalf("load config: %v", err)
	}

	logger, cleanup, err := logging.Setup(cfg.Logging)
	if err != nil {
		fatalf("configure logging: %v", err)
	}
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	uploaderClient, err := uploader.NewS3Uploader(ctx, cfg.AWS, cfg.Upload.MaxRetries)
	if err != nil {
		fatalf("initialise uploader: %v", err)
	}

	d := daemon.New(cfg, uploaderClient, logger)
	if err := d.Run(ctx); err != nil {
		logger.Error("daemon exited with error", "error", err)
		os.Exit(1)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

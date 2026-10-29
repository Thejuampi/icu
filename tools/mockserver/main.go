// Command mockserver is a local, in-memory mock of the intervals.icu REST API.
//
// It serves committed, anonymized fixtures from testdata/fixtures, accepts
// writes (PUT/POST/DELETE) in memory without persisting them, and exposes
// /_mock/writes and /_mock/reset so tests and release candidates can verify
// that a command did (or did not) write. Run it with:
//
//	go run ./tools/mockserver -port 8765
//
// The optional -record mode is only for the one-time fixture capture: it
// forwards GET requests to -upstream, anonymizes the responses into the
// fixture directory, and refuses every non-GET request without forwarding it.
//
// The optional -redirect-sink mode (off by default) starts a second loopback
// listener and serves /_redirect/<mode>/..., /_mock/leaks and a synthetic Zepp
// login, so an RC can check that no secret follows a redirect to another host
// (see redirect.go).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	defaultFixtures   = "testdata/fixtures"
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

type options struct {
	host     string
	port     int
	fixtures string
	record   bool
	upstream string
	anonDir  string
	// shiftDays/shiftSalt pin the -anonymize date shift, so a second fixture
	// set (the analysis corpus) can share the first one's offset.
	shiftDays int
	shiftSalt uint
	// redirectSink turns on redirect mode (see redirect.go).
	redirectSink bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{}
	fs := flag.NewFlagSet("icu-mock", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.host, "host", "127.0.0.1", "listen host (keep it on loopback)")
	fs.IntVar(&opts.port, "port", 0, "listen port (0 picks a free port)")
	fs.StringVar(&opts.fixtures, "fixtures", defaultFixtures, "fixture directory")
	fs.BoolVar(&opts.record, "record", false, "fixture capture mode: forward GETs to -upstream and save anonymized responses")
	fs.StringVar(&opts.upstream, "upstream", "", "upstream API for -record (required with -record; no default)")

	fs.BoolVar(&opts.redirectSink, "redirect-sink", false,
		"redirect mode: start a second loopback listener and serve /_redirect/<mode>/..., /_mock/leaks and synthetic Zepp login")
	fs.StringVar(&opts.anonDir, "anonymize", "", "re-anonymize every fixture in this directory and exit")
	fs.IntVar(&opts.shiftDays, "shift-days", 0, "with -anonymize: date shift in days (0 = random whole weeks back)")
	fs.UintVar(&opts.shiftSalt, "shift-salt", 0, "with -anonymize: jitter salt (0 = random)")

	if err := fs.Parse(args); err != nil {
		return opts, fmt.Errorf("parse flags: %w", err)
	}

	if opts.record && opts.upstream == "" {
		return opts, errors.New("-record requires an explicit -upstream URL")
	}

	if opts.port < 0 || opts.port > 65535 {
		return opts, fmt.Errorf("invalid -port %d", opts.port)
	}

	return opts, nil
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "icu-mock: %v\n", err)

		return 2
	}

	if opts.anonDir != "" {
		if anonErr := anonymizeDirWith(opts.anonDir, opts.dateShift()); anonErr != nil {
			fmt.Fprintf(stderr, "icu-mock: %v\n", anonErr)

			return 1
		}

		fmt.Fprintf(stdout, "anonymized %s\n", opts.anonDir)

		return 0
	}

	var handler http.Handler
	if opts.record {
		handler = newRecorder(opts.upstream, opts.fixtures, stderr)
	} else {
		srv, loadErr := newServer(opts.fixtures, stderr)
		if loadErr != nil {
			fmt.Fprintf(stderr, "icu-mock: %v\n", loadErr)

			return 1
		}

		if opts.redirectSink {
			if sinkErr := startRedirectSink(ctx, opts.host, srv, stdout); sinkErr != nil {
				fmt.Fprintf(stderr, "icu-mock: %v\n", sinkErr)

				return 1
			}
		}

		handler = srv
	}

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", net.JoinHostPort(opts.host, strconv.Itoa(opts.port)))
	if err != nil {
		fmt.Fprintf(stderr, "icu-mock: %v\n", err)

		return 1
	}

	return serve(ctx, listener, handler, stdout, stderr)
}

func serve(ctx context.Context, listener net.Listener, handler http.Handler, stdout, stderr io.Writer) int {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: readHeaderTimeout}

	fmt.Fprintf(stdout, "icu-mock listening on http://%s\n", listener.Addr().String())

	errCh := make(chan error, 1)

	go func() { errCh <- server.Serve(listener) }()

	select {
	case err := <-errCh:
		fmt.Fprintf(stderr, "icu-mock: %v\n", err)

		return 1
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "icu-mock: shutdown: %v\n", err)

		return 1
	}

	return 0
}

// dateShift is the -anonymize shift: random unless pinned by flags.
func (opts options) dateShift() dateShift {
	shift := randomDateShift()
	if opts.shiftDays != 0 {
		shift.days = opts.shiftDays
	}

	if opts.shiftSalt != 0 {
		shift.salt = uint32(opts.shiftSalt) //nolint:gosec // salt only needs 32 bits
	}

	return shift
}

// startRedirectSink starts the redirect-mode sink on another loopback port;
// it stops when ctx is done.
func startRedirectSink(ctx context.Context, host string, srv *server, stdout io.Writer) error {
	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return fmt.Errorf("redirect sink: %w", err)
	}

	sinkURL := "http://" + listener.Addr().String()
	srv.sink = newRedirectSink(sinkURL)
	sinkServer := &http.Server{Handler: srv.sink, ReadHeaderTimeout: readHeaderTimeout}

	go func() { _ = sinkServer.Serve(listener) }()
	go func() {
		<-ctx.Done()
		_ = sinkServer.Close()
	}()

	fmt.Fprintf(stdout, "icu-mock redirect sink on %s\n", sinkURL)

	return nil
}

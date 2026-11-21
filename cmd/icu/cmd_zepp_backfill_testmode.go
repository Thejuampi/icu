package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	icu "github.com/Thejuampi/icu"
)

const (
	zeppBackfillRealWriteFlag = "real-write-test"
	zeppBackfillFITFlag       = "fit"
	zeppBackfillTestTimeout   = 5 * time.Minute
)

func zeppBackfillTestCommand() *Command {
	return &Command{
		Usage:       "zepp backfill-test --real-write-test --fit FILE",
		Description: "Temporarily upload and undo one local cycling FIT using a saved Zepp web session.",
		Run:         runZeppBackfillTest,
	}
}

func runZeppBackfillTest(args []string, flags map[string]string, _ *icu.Client, stdout, _ io.Writer) error {
	flag, present := flags[zeppBackfillRealWriteFlag]
	if !present || (flag != "" && flag != "true") {
		return errors.New("writes are disabled; explicitly pass --real-write-test")
	}
	path := flags[zeppBackfillFITFlag]
	if len(args) != 0 || path == "" {
		return errors.New("provide exactly one local FIT file with --fit FILE")
	}

	cfg, err := icu.LoadConfig()
	if err != nil || cfg == nil || cfg.ZeppAppToken == "" || cfg.ZeppLoginToken == "" || cfg.ZeppUserID == "" {
		return errors.New("no usable saved Zepp web-session configuration; stopped without login")
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return errors.New("could not open the FIT file directory")
	}
	defer root.Close()
	name := filepath.Base(path)
	info, err := root.Stat(name)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("FIT must be a readable regular file")
	}
	fit, err := root.Open(name)
	if err != nil {
		return errors.New("could not open the FIT file")
	}
	defer fit.Close()

	client := icu.NewZeppClientFromAuth(&icu.ZeppAuthResult{
		LoginToken:  cfg.ZeppLoginToken,
		AppToken:    cfg.ZeppAppToken,
		UserID:      cfg.ZeppUserID,
		CountryCode: cfg.ZeppCountryCode,
	})
	ctx, cancel := context.WithTimeout(context.Background(), zeppBackfillTestTimeout)
	defer cancel()

	trackID, err := client.TestRideBackfill(ctx, fit)
	if err != nil {
		if trackID != 0 {
			return fmt.Errorf("zepp canary trackid %d: %w", trackID, err)
		}
		return err
	}

	return writeJSON(stdout, map[string]any{"trackid": trackID, "undo_verified": true})
}

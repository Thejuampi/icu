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
	zeppBackfillPreviewFlag   = "preview"
	zeppBackfillFITFlag       = "fit"
	zeppBackfillTestTimeout   = 5 * time.Minute
)

func zeppBackfillTestCommand() *Command {
	return &Command{
		Usage:       "zepp backfill-test (--preview | --real-write-test) --fit FILE",
		Description: "Inspect one cycling FIT offline, or explicitly test and undo an upload with an existing Zepp web session.",
		Run:         runZeppBackfillTest,
	}
}

func runZeppBackfillTest(args []string, flags map[string]string, _ *icu.Client, stdout, _ io.Writer) error {
	flag, realPresent := flags[zeppBackfillRealWriteFlag]
	preview, previewPresent := flags[zeppBackfillPreviewFlag]
	if previewPresent && realPresent {
		return errors.New("--preview and --real-write-test are mutually exclusive")
	}
	if previewPresent {
		if preview != "" && preview != "true" {
			return errors.New("--preview must be enabled without a value or with true")
		}
	} else if !realPresent || (flag != "" && flag != "true") {
		return errors.New("writes are disabled; pass --preview or explicitly pass --real-write-test")
	}
	path := flags[zeppBackfillFITFlag]
	if len(args) != 0 || path == "" {
		return errors.New("provide exactly one local FIT file with --fit FILE")
	}

	if previewPresent {
		return runZeppBackfillPreview(path, stdout)
	}

	cfg, err := icu.LoadConfig()
	if err != nil || cfg == nil || cfg.ZeppAppToken == "" || cfg.ZeppLoginToken == "" || cfg.ZeppUserID == "" {
		return errors.New("no usable saved Zepp web-session configuration; stopped without login")
	}

	fit, err := openZeppBackfillFIT(path)
	if err != nil {
		return err
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

func runZeppBackfillPreview(path string, stdout io.Writer) error {
	fit, err := openZeppBackfillFIT(path)
	if err != nil {
		return err
	}
	defer fit.Close()

	preview, err := icu.PreviewZeppRideFIT(fit)
	if err != nil {
		return err
	}

	return writeJSON(stdout, preview)
}

func openZeppBackfillFIT(path string) (*os.File, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, errors.New("could not open the FIT file directory")
	}
	defer root.Close()

	name := filepath.Base(path)
	info, err := root.Stat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("FIT must be a readable regular file")
	}
	fit, err := root.Open(name)
	if err != nil {
		return nil, errors.New("could not open the FIT file")
	}

	return fit, nil
}

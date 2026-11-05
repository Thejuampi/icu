package main

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	icu "github.com/Thejuampi/icu"
)

// activityIntervalsCommand reads GET /activity/{id}?intervals=true, the
// endpoint that returns icu_intervals and icu_groups, and prints them as a
// table (default) or JSON (--json).
func activityIntervalsCommand() *Command {
	return &Command{
		Name:  "",
		Usage: "activity <id> intervals [--json]",
		Description: "Get detected intervals (icu_intervals) and interval groups (icu_groups) " +
			"from GET /activity/{id}?intervals=true. Prints a table; --json prints the intervals object.",
		Schema: &CommandSchema{
			Flags: []CommandFlag{
				{Name: "json", ValueName: "BOOL", Description: "Print JSON instead of a table.", Kind: commandFlagBoolean},
			},
		},
		Run: func(args []string, flags map[string]string, client *icu.Client, stdout, _ io.Writer) error {
			if len(args) == 0 {
				return errMissing("activity id")
			}
			var dto icu.IntervalsDTO
			if err := client.Get("activity", []string{args[0]}, map[string]string{"intervals": "true"}, &dto); err != nil {
				return wrapCommandError(err)
			}
			if dto.ID == "" {
				dto.ID = args[0]
			}
			if dto.Intervals == nil {
				dto.Intervals = []icu.Interval{}
			}
			if dto.Groups == nil {
				dto.Groups = []icu.IntervalGroup{}
			}
			if BoolFlag(flags, "json") {
				return writeJSON(stdout, dto)
			}

			return printActivityIntervals(stdout, &dto)
		},
	}
}

func printActivityIntervals(stdout io.Writer, dto *icu.IntervalsDTO) error {
	fmt.Fprintf(stdout, "activity %s: %d interval(s), %d group(s)\n", dto.ID, len(dto.Intervals), len(dto.Groups))
	if len(dto.Intervals) == 0 {
		fmt.Fprintln(stdout, "no intervals")

		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tTYPE\tLABEL\tSTART\tDURATION\tAVG W\tNP\tMAX W\tAVG HR\tMAX HR\tDECOUPLING\tZONE\tLOAD\tGROUP")
	for i := range dto.Intervals {
		iv := &dto.Intervals[i]
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", i+1, dashIfEmpty(iv.Type), dashIfEmpty(iv.Label),
			clock(iv.StartTime), clock(iv.MovingTime), intCell(iv.AvgPower), intCell(iv.WeightedPower), intCell(iv.MaxPower),
			intCell(iv.AvgHR), intCell(iv.MaxHR), pctCell(iv.Decoupling), intCell(iv.Zone), floatCell(iv.TrainingLoad), dashIfEmpty(iv.GroupID))
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("print intervals: %w", err)
	}
	for i := range dto.Groups {
		g := &dto.Groups[i]
		fmt.Fprintf(stdout, "group %s: %d x, avg %s W, avg HR %s, duration %s\n", dashIfEmpty(g.ID), g.Count,
			intCell(g.AvgPower), intCell(g.AvgHR), clock(g.MovingTime))
	}

	return nil
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}

	return s
}

func intCell(v int) string {
	if v == 0 {
		return "-"
	}

	return strconv.Itoa(v)
}

func floatCell(v float64) string {
	if v == 0 {
		return "-"
	}

	return fmt.Sprintf("%.1f", v)
}

func pctCell(v float64) string {
	if v == 0 {
		return "-"
	}

	return fmt.Sprintf("%.1f%%", v)
}

// clock formats seconds as H:MM:SS or M:SS.
func clock(secs int) string {
	const secsPerMin, minsPerHour = 60, 60
	h, m, s := secs/(secsPerMin*minsPerHour), secs/secsPerMin%minsPerHour, secs%secsPerMin
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}

	return fmt.Sprintf("%d:%02d", m, s)
}

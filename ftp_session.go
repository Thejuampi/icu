package icu

// Session environment values accepted by --session-environment (D-10c).
const (
	SessionEnvironmentIndoor  = "indoor"
	SessionEnvironmentOutdoor = "outdoor"
)

// resolveEventFTP resolves a calendar event's FTPSelection. rebalance and
// plan operate on calendar events, never completed activities, so Entity is
// always the event partition (D-10). indoor is the matched existing event's
// own Indoor flag when one exists, or nil for a not-yet-created session with
// no per-record signal.
func resolveEventFTP(sportType string, indoor *bool, settings *SportSettings, sessionEnvironment string) FTPSelection {
	return ResolveFTP(FTPRequest{
		Type:        sportType,
		Entity:      FTPEntityEvent,
		EventIndoor: indoor,
		Settings:    settings,
		Explicit:    explicitPtrFromSessionEnvironment(sessionEnvironment),
	})
}

// athleteHasFTP reports whether any FTP number is configured at all — a
// distinct question from "which one applies to this session" (D-10). Used by
// envelope metrics that gate on data availability, not on pricing a session.
func athleteHasFTP(settings *SportSettings) bool {
	return settings != nil && (settings.FTP > 0 || settings.IndoorFTP > 0)
}

func explicitPtrFromSessionEnvironment(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

// ftpMixedEnvironmentWeek reports whether the week contains sessions whose
// natural (no-Explicit) resolution disagrees on indoor vs. outdoor — the
// case D-10c names explicitly: "a week containing both VirtualRide and Ride
// events." A single --session-environment scalar cannot describe that week
// without silently overriding whichever side's already-known signal
// contradicts it, so the caller must refuse rather than apply the override.
func ftpMixedEnvironmentWeek(sportTypes []string, indoorFlags []*bool, settings *SportSettings) bool {
	var sawIndoor, sawOutdoor bool

	for index := range sportTypes {
		var indoor *bool
		if index < len(indoorFlags) {
			indoor = indoorFlags[index]
		}

		natural := resolveEventFTP(sportTypes[index], indoor, settings, "")
		if !natural.Applicable {
			continue
		}

		if natural.Indoor != nil && *natural.Indoor {
			sawIndoor = true
		} else {
			sawOutdoor = true
		}
	}

	return sawIndoor && sawOutdoor
}

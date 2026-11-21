package icu

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

const (
	fitUnixEpoch   = 631065600
	maxZeppFITSize = 64 << 20
)

// ZeppRideFIT contains one FIT cycling session, with UTC Unix seconds and SI units.
type ZeppRideFIT struct {
	Start, End               int64
	Timer, Elapsed, Distance float64
	Calories                 *uint16
	AverageSpeed             *float64
	Laps                     int
	Indoor                   bool
	Points                   []ZeppRideFITPoint
}

// ZeppRideFITPoint contains samples present in the original FIT record. Nil
// distinguishes missing readings from valid zeros.
type ZeppRideFITPoint struct {
	At                                             int64
	Latitude, Longitude, Altitude, Speed, Distance *float64
	HeartRate, Cadence, Power                      *uint16
}

type zeppFITField struct{ number, size, base byte }

type zeppFITDef struct {
	order          binary.ByteOrder
	global         uint16
	fields         []zeppFITField
	developerBytes int
}

type zeppFITRecords struct {
	definitions   [16]*zeppFITDef
	lastTimestamp uint32
	hasTimestamp  bool
	ride          ZeppRideFIT
	foundSession  bool
}

// DecodeZeppRideFIT reads one CRC-checked FIT activity with a single cycling
// session. It uses standard FIT profile fields only, including enhanced speed
// and altitude when present. It does not synthesize samples, pauses, or laps.
func DecodeZeppRideFIT(input io.Reader) (ZeppRideFIT, error) {
	raw, err := io.ReadAll(io.LimitReader(input, maxZeppFITSize+1))
	if err != nil {
		return ZeppRideFIT{}, fmt.Errorf("read FIT: %w", err)
	}
	if len(raw) > maxZeppFITSize {
		return ZeppRideFIT{}, errors.New("FIT exceeds 64 MiB")
	}
	if len(raw) < 14 || (raw[0] != 12 && raw[0] < 14) || !bytes.Equal(raw[8:12], []byte(".FIT")) {
		return ZeppRideFIT{}, errors.New("invalid FIT header")
	}
	h := int(raw[0])
	if uint64(h)+uint64(binary.LittleEndian.Uint32(raw[4:8]))+2 != uint64(len(raw)) {
		return ZeppRideFIT{}, errors.New("FIT data length mismatch (chained files unsupported)")
	}
	if h >= 14 && binary.LittleEndian.Uint16(raw[h-2:h]) != 0 && zeppFITCRC(raw[:h]) != 0 {
		return ZeppRideFIT{}, errors.New("invalid FIT header CRC")
	}
	if zeppFITCRC(raw) != 0 {
		return ZeppRideFIT{}, errors.New("invalid FIT file CRC")
	}
	state := zeppFITRecords{}
	if err = state.decode(raw[h : len(raw)-2]); err != nil {
		return ZeppRideFIT{}, err
	}
	if !state.foundSession {
		return ZeppRideFIT{}, errors.New("FIT lacks a cycling session")
	}
	if err = validateZeppRideFIT(state.ride); err != nil {
		return ZeppRideFIT{}, err
	}
	return state.ride, nil
}

func zeppFITCRC(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xa001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func readZeppFITDef(data []byte, developer bool) (*zeppFITDef, int, error) {
	if len(data) < 5 || data[1] > 1 {
		return nil, 0, errors.New("invalid FIT message definition")
	}
	d := &zeppFITDef{order: binary.LittleEndian}
	if data[1] == 1 {
		d.order = binary.BigEndian
	}
	d.global = d.order.Uint16(data[2:4])
	at := 5
	count := int(data[4])
	if len(data) < at+3*count {
		return nil, 0, io.ErrUnexpectedEOF
	}
	for range count {
		n, size, base := data[at], data[at+1], data[at+2]
		if size == 0 {
			return nil, 0, errors.New("zero-length FIT field")
		}
		d.fields = append(d.fields, zeppFITField{n, size, base})
		at += 3
	}
	if developer {
		if len(data) == at {
			return nil, 0, io.ErrUnexpectedEOF
		}
		count = int(data[at])
		at++
		if len(data) < at+3*count {
			return nil, 0, io.ErrUnexpectedEOF
		}
		for range count {
			d.developerBytes += int(data[at+1])
			at += 3
		}
	}
	return d, at, nil
}

func (s *zeppFITRecords) decode(data []byte) error {
	for len(data) > 0 {
		header := data[0]
		data = data[1:]
		compressed := header&0x80 != 0
		local := header & 15
		if compressed {
			local = (header >> 5) & 3
		} else if header&0x40 != 0 {
			d, n, err := readZeppFITDef(data, header&0x20 != 0)
			if err != nil {
				return fmt.Errorf("FIT local definition %d: %w", local, err)
			}
			s.definitions[local] = d
			data = data[n:]
			continue
		}
		d := s.definitions[local]
		if d == nil {
			return fmt.Errorf("undefined FIT local message %d", local)
		}
		n, err := s.readRecord(d, data, compressed, header&31)
		if err != nil {
			return fmt.Errorf("FIT message %d: %w", d.global, err)
		}
		data = data[n:]
	}
	return nil
}

func (s *zeppFITRecords) readRecord(d *zeppFITDef, data []byte, compressed bool, offset byte) (int, error) {
	var (
		present [256]bool
		value   [256]int64
	)
	if compressed {
		if !s.hasTimestamp || s.lastTimestamp >= math.MaxUint32-31 {
			return 0, errors.New("invalid compressed FIT timestamp")
		}
		t := s.lastTimestamp&^31 | uint32(offset)
		if t < s.lastTimestamp {
			t += 32
		}
		value[253], present[253] = int64(t), true
	}
	at := 0
	for i, f := range d.fields {
		if compressed && f.number == 253 {
			if i != 0 || f.size != 4 || f.base != 0x86 {
				return 0, errors.New("invalid compressed timestamp field definition")
			}
			continue
		}
		next := at + int(f.size)
		if next > len(data) {
			return 0, io.ErrUnexpectedEOF
		}
		if f.number == 253 || d.global == 18 || d.global == 20 {
			value[f.number], present[f.number] = zeppFITNumeric(d.order, f, data[at:next])
		}
		at = next
	}
	at += d.developerBytes
	if at > len(data) {
		return 0, io.ErrUnexpectedEOF
	}
	if present[253] {
		if value[253] < 0 || value[253] >= math.MaxUint32 {
			return 0, errors.New("invalid FIT timestamp value")
		}
		s.lastTimestamp = uint32(value[253])
		s.hasTimestamp = true
	}
	switch d.global {
	case 18:
		return at, s.addSession(value, present)
	case 19:
		s.ride.Laps++
		return at, nil
	case 20:
		return at, s.addPoint(value, present)
	default:
		return at, nil
	}
}

func zeppFITNumeric(order binary.ByteOrder, f zeppFITField, data []byte) (int64, bool) {
	switch {
	case len(data) == 1 && (f.base == 0 || f.base == 2):
		return int64(data[0]), data[0] != 0xff
	case len(data) == 2 && f.base == 0x84:
		i := order.Uint16(data)
		return int64(i), i != math.MaxUint16
	case len(data) == 4 && f.base == 0x86:
		i := order.Uint32(data)
		return int64(i), i != math.MaxUint32
	case len(data) == 4 && f.base == 0x85:
		i := int32(order.Uint32(data))
		return int64(i), i != math.MaxInt32
	default:
		return 0, false
	}
}

func (s *zeppFITRecords) addSession(v [256]int64, p [256]bool) error {
	if s.foundSession {
		return errors.New("multiple FIT sessions unsupported")
	}
	if !p[5] || v[5] != 2 {
		return errors.New("FIT session is not cycling")
	}
	if !p[2] || !p[253] || (!p[7] && !p[8]) || !p[9] {
		return errors.New("FIT session lacks start, end, duration or distance")
	}
	s.foundSession = true
	s.ride.Start = v[2] + fitUnixEpoch
	s.ride.End = v[253] + fitUnixEpoch
	s.ride.Indoor = p[6] && v[6] == 6
	s.ride.Distance = float64(v[9]) / 100
	s.ride.AverageSpeed = zeppFITScaled(v, p, 124, 14, 1000, 0)
	if p[11] {
		k := uint16(v[11])
		s.ride.Calories = &k
	}
	if p[7] {
		s.ride.Elapsed = float64(v[7]) / 1000
	}
	if p[8] {
		s.ride.Timer = float64(v[8]) / 1000
	} else {
		s.ride.Timer = s.ride.Elapsed
	}
	return nil
}

func (s *zeppFITRecords) addPoint(v [256]int64, p [256]bool) error {
	if !p[253] {
		return errors.New("FIT sample has no timestamp")
	}
	pt := ZeppRideFITPoint{At: v[253] + fitUnixEpoch}
	if p[0] {
		latitude := math.Ldexp(float64(v[0])*180, -31)
		pt.Latitude = &latitude
	}
	if p[1] {
		longitude := math.Ldexp(float64(v[1])*180, -31)
		pt.Longitude = &longitude
	}
	pt.Altitude = zeppFITScaled(v, p, 78, 2, 5, -500)
	pt.Speed = zeppFITScaled(v, p, 73, 6, 1000, 0)
	pt.Distance = zeppFITScaled(v, p, 5, 5, 100, 0)
	if p[3] {
		hr := uint16(v[3])
		pt.HeartRate = &hr
	}
	if p[4] {
		cadence := uint16(v[4])
		pt.Cadence = &cadence
	}
	if p[7] {
		power := uint16(v[7])
		pt.Power = &power
	}
	s.ride.Points = append(s.ride.Points, pt)
	return nil
}

func zeppFITScaled(v [256]int64, p [256]bool, preferred, legacy int, divisor, offset float64) *float64 {
	i := preferred
	if !p[i] {
		i = legacy
	}
	if !p[i] {
		return nil
	}
	value := float64(v[i])/divisor + offset
	return &value
}

func validateZeppRideFIT(r ZeppRideFIT) error {
	if r.Start <= 0 || r.End <= r.Start || r.End > fitUnixEpoch+math.MaxUint32 ||
		!zeppFITInRange(r.Distance, 0, math.MaxUint32) || !zeppFITInRange(r.Timer, 0, float64(r.End-r.Start)+1) ||
		!zeppFITInRange(r.Elapsed, 0, float64(r.End-r.Start)+1) {
		return errors.New("invalid cycling session bounds or summary")
	}
	if r.Elapsed > 0 && r.Timer > r.Elapsed+1 {
		return errors.New("FIT timer exceeds elapsed time")
	}
	if len(r.Points) == 0 {
		return errors.New("FIT session contains no samples")
	}
	last := r.Start - 1
	for _, p := range r.Points {
		if p.At < r.Start || p.At > r.End || p.At <= last {
			return errors.New("FIT samples are outside the session or not strictly ordered")
		}
		last = p.At
		if p.Altitude != nil && !zeppFITInRange(*p.Altitude, -500, 1e6) ||
			p.Speed != nil && !zeppFITInRange(*p.Speed, 0, 1e6) ||
			p.Distance != nil && !zeppFITInRange(*p.Distance, 0, math.MaxUint32) ||
			p.Latitude != nil && !zeppFITInRange(*p.Latitude, -90, 90) ||
			p.Longitude != nil && !zeppFITInRange(*p.Longitude, -180, 180) ||
			p.HeartRate != nil && *p.HeartRate >= 255 || p.Cadence != nil && *p.Cadence >= 255 {
			return errors.New("FIT sample has an invalid sensor value")
		}
	}
	return nil
}

func zeppFITInRange(n, minValue, maxValue float64) bool { return n >= minValue && n <= maxValue }

// EncodeZeppRideFIT is the pure wire encoder for the observed Zepp cycling
// upload format. Unverified pause and lap fields are deliberately absent.
func EncodeZeppRideFIT(r ZeppRideFIT) (summary, detail map[string]string, err error) {
	if err = validateZeppRideFIT(r); err != nil {
		return nil, nil, err
	}
	typ := "9"
	if r.Indoor {
		typ = "10"
	}
	summary = map[string]string{
		"trackid": strconv.FormatInt(r.Start, 10), "end_time": strconv.FormatInt(r.End, 10),
		"dis": strconv.FormatInt(int64(math.Round(r.Distance)), 10), "run_time": strconv.FormatInt(int64(math.Round(r.Timer)), 10),
		"avg_pace": "0", "type": typ, "sport_mode": "0", "source": "run.fit.huami.com",
		"version": "16", "devicesource": "-1", "bind_device": "",
	}
	// Distance divided by recorded timer duration is the Garmin running/cycling
	// session-average speed definition when the explicit summary speed is absent.
	if r.AverageSpeed != nil {
		if !zeppFITInRange(*r.AverageSpeed, 0, 1e6) {
			return nil, nil, errors.New("invalid FIT average speed")
		}
		if *r.AverageSpeed > 0 {
			summary["avg_pace"] = strconv.FormatFloat(1 / *r.AverageSpeed, 'f', -1, 64)
		}
	} else if r.Timer > 0 && r.Distance > 0 {
		summary["avg_pace"] = strconv.FormatFloat(r.Timer/r.Distance, 'f', -1, 64)
	}
	if r.Calories != nil {
		summary["calorie"] = strconv.FormatUint(uint64(*r.Calories), 10)
	}
	detail, err = encodeZeppFITPoints(r)
	if err != nil {
		return nil, nil, err
	}
	for _, k := range []string{"trackid", "source", "version"} {
		detail[k] = summary[k]
	}
	return summary, detail, nil
}

type zeppFITSeries struct {
	text      strings.Builder
	lastAt    int64
	lastValue int64
	hasValue  bool
}

func (s *zeppFITSeries) add(at, value int64, deltaValue bool) {
	delta := at - s.lastAt
	s.lastAt = at
	n := value
	first := !s.hasValue
	if deltaValue && !first {
		n = value - s.lastValue
	}
	s.hasValue = true
	s.lastValue = value
	if !deltaValue || delta != 1 {
		s.text.WriteString(strconv.FormatInt(delta, 10))
	}
	s.text.WriteByte(',')
	s.text.WriteString(strconv.FormatInt(n, 10))
	s.text.WriteByte(';')
}

func encodeZeppFITPoints(r ZeppRideFIT) (map[string]string, error) {
	var times, gps, altitude, pace strings.Builder
	speed := zeppFITSeries{lastAt: r.Start}
	hr := zeppFITSeries{lastAt: r.Start}
	cad := zeppFITSeries{lastAt: r.Start}
	power := zeppFITSeries{lastAt: r.Start}
	lastTime := r.Start
	var (
		lastLat, lastLon               float64
		gpsCount, altCount, speedCount int
	)
	for i, p := range r.Points {
		writeZeppFITNumber(&times, p.At-lastTime)
		lastTime = p.At
		if (p.Latitude == nil) != (p.Longitude == nil) {
			return nil, errors.New("FIT sample has only one GPS coordinate")
		}
		if p.Latitude != nil {
			if gpsCount == 0 && i != 0 {
				return nil, errors.New("unaligned FIT GPS series")
			}
			gps.WriteString(strconv.FormatInt(int64((*p.Latitude-lastLat)*1e8), 10))
			gps.WriteByte(',')
			writeZeppFITNumber(&gps, int64((*p.Longitude-lastLon)*1e8))
			lastLat, lastLon = *p.Latitude, *p.Longitude
			gpsCount++
		}
		if p.Altitude != nil {
			writeZeppFITNumber(&altitude, int64(*p.Altitude*100))
			altCount++
		}
		if p.Speed != nil {
			pace.WriteString(strconv.FormatFloat(*p.Speed, 'f', 2, 64))
			pace.WriteByte(';')
			speedCount++
			inverse := "0.00"
			if *p.Speed > 0 {
				inverse = strconv.FormatFloat(1 / *p.Speed, 'f', 2, 64)
			}
			speed.text.WriteString(strconv.FormatInt(p.At-speed.lastAt, 10) + "," + inverse + ";")
			speed.lastAt = p.At
		}
		if p.HeartRate != nil {
			hr.add(p.At, int64(*p.HeartRate), true)
		}
		if p.Cadence != nil {
			cad.add(p.At, int64(*p.Cadence), false)
		}
		if p.Power != nil {
			power.add(p.At, int64(*p.Power), false)
		}
	}
	for _, n := range []int{gpsCount, altCount, speedCount} {
		if n != 0 && n != len(r.Points) {
			return nil, errors.New("FIT has gaps in aligned GPS, altitude or speed data; refusing to drop or invent samples")
		}
	}
	detail := map[string]string{"time": strings.TrimSuffix(times.String(), ";")}
	for k, v := range map[string]string{
		"longitude_latitude": strings.TrimSuffix(gps.String(), ";"), "altitude": strings.TrimSuffix(altitude.String(), ";"),
		"pace": strings.TrimSuffix(pace.String(), ";"), "speed": strings.TrimSuffix(speed.text.String(), ";"),
		"heart_rate": strings.TrimSuffix(hr.text.String(), ";"), "cadence": cad.text.String(), "power_meter": power.text.String(),
	} {
		if v != "" {
			detail[k] = v
		}
	}
	return detail, nil
}

func writeZeppFITNumber(b *strings.Builder, n int64) {
	b.WriteString(strconv.FormatInt(n, 10))
	b.WriteByte(';')
}

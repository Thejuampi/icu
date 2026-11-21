package icu

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func zeppFITFixture(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	// record: timestamp, semicircle lat/lon, altitude, speed, HR, cadence, power.
	data.Write([]byte{0x40, 0, 0, 20, 0, 8, 253, 4, 0x86, 0, 4, 0x85, 1, 4, 0x85, 2, 2, 0x84, 6, 2, 0x84, 3, 1, 2, 4, 1, 2, 7, 2, 0x84})
	for i, offset := range []uint32{2, 3, 6} {
		data.WriteByte(0)
		data.Write(binary.LittleEndian.AppendUint32(nil, 1000+offset))
		data.Write(binary.LittleEndian.AppendUint32(nil, uint32(1<<29)+uint32(i)*(1<<20)))
		data.Write(binary.LittleEndian.AppendUint32(nil, uint32(1<<31)+uint32(1<<29)))
		data.Write(binary.LittleEndian.AppendUint16(nil, 2500+uint16(i)*5))
		data.Write(binary.LittleEndian.AppendUint16(nil, []uint16{4000, 0, 5000}[i]))
		data.Write([]byte{[]byte{140, 145, 143}[i], []byte{90, 0, 84}[i]})
		data.Write(binary.LittleEndian.AppendUint16(nil, []uint16{230, 0, 250}[i]))
	}
	// Big-endian session plus a developer field.
	data.Write([]byte{0x61, 0, 1, 0, 18, 8, 253, 4, 0x86, 2, 4, 0x86, 5, 1, 0, 6, 1, 0, 7, 4, 0x86, 8, 4, 0x86, 9, 4, 0x86, 11, 2, 0x84, 1, 0, 3, 0})
	data.WriteByte(1)
	for _, x := range []uint32{1008, 1000} {
		data.Write(binary.BigEndian.AppendUint32(nil, x))
	}
	data.Write([]byte{2, 6})
	for _, x := range []uint32{8000, 6000, 2500} {
		data.Write(binary.BigEndian.AppendUint32(nil, x))
	}
	data.Write(binary.BigEndian.AppendUint16(nil, 12))
	data.Write([]byte{1, 2, 3})
	return zeppFITFile(data.Bytes())
}

func zeppFITFile(data []byte) []byte {
	dataLen := len(data)
	if dataLen < 0 || dataLen > maxZeppFITSize {
		panic("oversized synthetic FIT fixture")
	}
	raw := make([]byte, 0, 14+dataLen+2)
	raw = append(raw, 14, 0x20, 0, 0, 0, 0, 0, 0, '.', 'F', 'I', 'T', 0, 0)
	binary.LittleEndian.PutUint32(raw[4:8], uint32(dataLen))
	binary.LittleEndian.PutUint16(raw[12:14], zeppFITCRC(raw[:12]))
	raw = append(raw, data...)
	return binary.LittleEndian.AppendUint16(raw, zeppFITCRC(raw))
}

func TestZeppRideFITDecodeEncode(t *testing.T) {
	ride, err := DecodeZeppRideFIT(bytes.NewReader(zeppFITFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	summary, detail, err := EncodeZeppRideFIT(ride)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"trackid": "631066600", "end_time": "631066608", "run_time": "6", "dis": "25", "avg_pace": "0.24", "calorie": "12", "type": "10", "sport_mode": "0", "devicesource": "-1", "bind_device": "", "source": "run.fit.huami.com", "version": "16"} {
		if summary[k] != want {
			t.Errorf("summary %s = %q, want %q", k, summary[k], want)
		}
	}
	for k, want := range map[string]string{
		"time": "2;1;3", "trackid": "631066600", "source": "run.fit.huami.com", "version": "16",
		"longitude_latitude": "4500000000,-13500000000;8789062,0;8789062,0",
		"altitude":           "0;100;200",
		"pace":               "4.00;0.00;5.00",
		"speed":              "2,0.25;1,0.00;3,0.20",
		"heart_rate":         "2,140;,5;3,-2",
		"cadence":            "2,90;1,0;3,84;",
		"power_meter":        "2,230;1,0;3,250;",
	} {
		if detail[k] != want {
			t.Errorf("detail %s = %q, want %q", k, detail[k], want)
		}
	}
	for _, key := range []string{"pause", "lap", "bulkdistance", "currentDistance"} {
		if _, ok := detail[key]; ok {
			t.Errorf("unexpected %s", key)
		}
	}
}

func TestZeppRideFITRejectInvalid(t *testing.T) {
	raw := zeppFITFixture(t)
	corrupt := append([]byte(nil), raw...)
	corrupt[len(corrupt)-3] ^= 1
	for _, tt := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"header", []byte("not fit"), "header"},
		{"corrupt", corrupt, "CRC"},
		{"truncated", raw[:len(raw)-1], "length"},
		{"undefined", zeppFITFile([]byte{0}), "undefined"},
		{"compressed without previous", zeppFITFile([]byte{0x40, 0, 0, 20, 0, 0, 0x81}), "timestamp"},
		{"missing session", zeppFITFile(nil), "cycling session"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeZeppRideFIT(bytes.NewReader(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name   string
		change func(*ZeppRideFIT)
		want   string
	}{
		{"invalid timer", func(r *ZeppRideFIT) { r.Timer = math.NaN() }, "summary"},
		{"duplicate time", func(r *ZeppRideFIT) { r.Points[1].At = r.Points[0].At }, "ordered"},
		{"incomplete coordinate", func(r *ZeppRideFIT) { r.Points[0].Latitude = nil }, "one GPS"},
		{"GPS gap", func(r *ZeppRideFIT) { r.Points[1].Latitude = nil; r.Points[1].Longitude = nil }, "gaps"},
		{"speed gap", func(r *ZeppRideFIT) { r.Points[1].Speed = nil }, "gaps"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := DecodeZeppRideFIT(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			tt.change(&r)
			if _, _, err = EncodeZeppRideFIT(r); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestZeppFITCompressedRecordRollover(t *testing.T) {
	s := zeppFITRecords{}
	raw := []byte{0x40, 0, 0, 20, 0, 2, 253, 4, 0x86, 3, 1, 2, 0, 0x3d, 0, 0, 0, 140, 0x82, 145}
	if err := s.decode(raw); err != nil {
		t.Fatal(err)
	}
	if len(s.ride.Points) != 2 || s.ride.Points[1].At != fitUnixEpoch+66 || *s.ride.Points[1].HeartRate != 145 {
		t.Fatalf("points=%+v", s.ride.Points)
	}
}

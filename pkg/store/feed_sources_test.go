package store

import (
	"errors"
	"math"
	"testing"
)

func ptrInt(v int) *int       { return &v }
func ptrStr(v string) *string { return &v }
func bbox(minLat, minLon, maxLat, maxLon float64) *BBox {
	return &BBox{MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}
}

func TestSourceConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     SourceConfig
		wantIdx int // expected InvalidSourceError.Index when wantErr
		wantErr bool
	}{
		{name: "empty is valid", cfg: SourceConfig{}},
		{name: "nil is valid", cfg: nil},
		{
			name: "valid adsb with bbox",
			cfg:  SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9)}},
		},
		{
			name: "valid adsb with cred_ref",
			cfg:  SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9), CredRef: ptrStr("secret/speyer")}},
		},
		{
			name: "valid radar with sac/sic, no bbox",
			cfg:  SourceConfig{{Type: SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)}},
		},
		{
			name: "valid mixed config",
			cfg: SourceConfig{
				{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9)},
				{Type: SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)},
			},
		},
		{
			name:    "unknown type",
			cfg:     SourceConfig{{Type: "satellite_quantum"}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "adsb without bbox",
			cfg:     SourceConfig{{Type: SourceADSBOpenSky}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "flarm without bbox",
			cfg:     SourceConfig{{Type: SourceFLARMAPRS}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "area source with sensor identity rejected",
			cfg:     SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9), SAC: ptrInt(1)}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "radar without sac/sic",
			cfg:     SourceConfig{{Type: SourceRadarASTERIX}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "radar sac out of range",
			cfg:     SourceConfig{{Type: SourceRadarASTERIX, SAC: ptrInt(256), SIC: ptrInt(4)}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "bbox latitude out of range",
			cfg:     SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(-91, 7, 50, 9)}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "bbox min exceeds max",
			cfg:     SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(50, 7, 48, 9)}},
			wantErr: true, wantIdx: 0,
		},
		{
			name:    "blank cred_ref rejected",
			cfg:     SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9), CredRef: ptrStr("   ")}},
			wantErr: true, wantIdx: 0,
		},
		{
			name: "error reports offending index",
			cfg: SourceConfig{
				{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9)},
				{Type: SourceRadarASTERIX}, // invalid: no sac/sic
			},
			wantErr: true, wantIdx: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				var ise *InvalidSourceError
				if !errors.As(err, &ise) {
					t.Fatalf("Validate() = %v, want *InvalidSourceError", err)
				}
				if ise.Index != tt.wantIdx {
					t.Fatalf("error index = %d, want %d (%v)", ise.Index, tt.wantIdx, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestCoverageBBoxUnion(t *testing.T) {
	cfg := SourceConfig{
		{Type: SourceADSBOpenSky, BBox: bbox(48, 7, 50, 9)},
		{Type: SourceFLARMAPRS, BBox: bbox(47, 8, 49, 11)},
		{Type: SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)}, // no bbox: ignored
	}
	got := cfg.CoverageBBox(0)
	if got == nil {
		t.Fatal("CoverageBBox = nil, want union")
	}
	want := BBox{MinLat: 47, MinLon: 7, MaxLat: 50, MaxLon: 11}
	if *got != want {
		t.Fatalf("union = %+v, want %+v", *got, want)
	}
}

func TestCoverageBBoxNoBBoxSources(t *testing.T) {
	cfg := SourceConfig{{Type: SourceRadarASTERIX, SAC: ptrInt(1), SIC: ptrInt(4)}}
	if got := cfg.CoverageBBox(50); got != nil {
		t.Fatalf("CoverageBBox = %+v, want nil (no bbox sources)", got)
	}
}

func TestCoverageBBoxMarginExpands(t *testing.T) {
	cfg := SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(49, 8, 50, 9)}}
	got := cfg.CoverageBBox(111) // ~1° latitude
	if got == nil {
		t.Fatal("CoverageBBox = nil")
	}
	// Latitude margin ≈ 1°, so the box grows by roughly a degree on each side.
	if math.Abs(got.MinLat-48) > 0.02 {
		t.Errorf("MinLat = %f, want ≈48", got.MinLat)
	}
	if math.Abs(got.MaxLat-51) > 0.02 {
		t.Errorf("MaxLat = %f, want ≈51", got.MaxLat)
	}
	// Longitude margin is larger than the latitude margin at ~50°N (a degree of
	// longitude is shorter), so the box must widen by more than 1° in lon.
	if got.MinLon >= 7 || got.MaxLon <= 10 {
		t.Errorf("lon span = [%f,%f], want wider than [7,10]", got.MinLon, got.MaxLon)
	}
}

func TestCoverageBBoxMarginClampsToValidRange(t *testing.T) {
	// A box near the pole with a huge margin must clamp, never overflow.
	cfg := SourceConfig{{Type: SourceADSBOpenSky, BBox: bbox(89, 179, 89.5, 179.5)}}
	got := cfg.CoverageBBox(500)
	if got == nil {
		t.Fatal("CoverageBBox = nil")
	}
	if got.MinLat < -90 || got.MaxLat > 90 || got.MinLon < -180 || got.MaxLon > 180 {
		t.Fatalf("coverage not clamped: %+v", *got)
	}
}

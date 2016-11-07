package gosolar

import "testing"

func TestPSH(t *testing.T) {
	loc := Location{
		Lat:   47.1718,
		Lon:   122.5185,
		City:  "Lakewood",
		State: "WA",
	}

	r := PeakSolarHours(0, loc)
	if r > 24 {
		t.Error("Cannot have more than 24 PSH in a day")
	}

	if r < 0 {
		t.Error("Must have more than 0 PSH in a day")
	}
}

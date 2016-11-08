package gosolar

import "testing"

var loc Location = Location{
	Lat:   47.1718,
	Lon:   -122.5185,
	Alt:   0.079,
	City:  "Lakewood",
	State: "WA",
}

func TestPSH(t *testing.T) {

	r := PeakSolarHours(0, loc)
	if r > 24 {
		t.Error("Cannot have more than 24 PSH in a day")
	}

	if r < 0 {
		t.Error("Must have more than 0 PSH in a day")
	}
}

func TestTimezone(t *testing.T) {
	tz := TimezoneFor(loc)
	// Should roughly be -8 (PST)
	if int(tz) != -8 {
		t.Errorf("Expecting timezone -8, got %f", tz)
	}
}

func TestLSTM(t *testing.T) {
	l := LSTM(TimezoneFor(loc))
	if RadToDeg*l != loc.Lon {
		t.Errorf("Expecting %f == %f", RadToDeg*l, loc.Lon)
	}
}

func TestEoT(t *testing.T) {
	e := EoT(0)
	// Should be somewhere around -3.(1-2) for Jan. 1 (day 0)
	if int(e) != -3 {
		t.Errorf("Expecing ~ -3, got %f", e)
	}
}

func TestTCF(t *testing.T) {
	// 172nd day of year is jun 21; the solstice.
	tcf := TCF(172, loc)
	if int(tcf) != -1 {
		t.Errorf("Expecting to be off by ~ 1 minutes on solstice in WA, got %f (%d)", tcf, int(tcf))
	}
}

func TestLST(t *testing.T) {
	// The local solar time should be the local time in minutes after midnight,
	// plus the time-correction-factor (also in minutes)
	// FIXME: I'm not positive that my understanding of this is correct. I will
	// continue on with some other equivelences first and come back to this.
	/*
		lst := LST(720, 0, loc)
		if lst != TCF(720, loc) {
			t.Errorf("At noon, the local solar time should == the time correction factor. Got: %f", lst)
		}
	*/
}

func TestHRA(t *testing.T) {
	// The HRA at noon is, by definition, 0. We round because 720 minutes past
	// midnight is not quite noon in LST, but should be very close.
	h := RadToDeg * HRA(720, 0, loc)
	if int(h) > 1 {
		t.Errorf("Expecting 0 degrees at solar noon, got %f", h)
	}

	// Before noon it will be negative, and after noon will be positive.
	h = RadToDeg * HRA(0, 0, loc)
	if h > 0 {
		t.Errorf("Expecting before noon to be negative, got %f", h)
	}

	h = RadToDeg * HRA(800, 0, loc)
	if h < 0 {
		t.Errorf("Expecting afternoon to be positive, got %f", h)
	}
}

func TestDeclination(t *testing.T) {
	// On the Equinoxes it should be 0. On the solstices it should be +/- ~23
	// degrees. We round here because the actual solstices and equinoxes happen
	// only for an instant on their given days, so for a whole day we will be off
	// by a small amount.
	d := RadToDeg * Declination(79) // March 20; spring equinox
	if int(d) != 0 {
		t.Errorf("%f", d)
	}

	d = RadToDeg * Declination(171) // June 20; Summer solstice
	if int(d) != 23 {
		t.Errorf("Expecting to be close to 23, got %f (%d)", RadToDeg*d, int(d))
	}
}

func TestElevation(t *testing.T) {
	// On Jan. 1, at 7:45 in the morning we should be close to 1 degree
	// elevation.
	e := Elevation(465, 0, loc)
	if int(e*RadToDeg) != 0 {
		t.Errorf("Expecting to be close to 0.8, got %f", e*RadToDeg)
	}

	// On the same day at 12:00 noon, we should be close to 19-20 degrees.
	e = Elevation(720, 0, loc)
	if int(e*RadToDeg) != 19 {
		t.Errorf("Expecting to be close to 19-20, got %f", e*RadToDeg)
	}
}

func TestZenith(t *testing.T) {
	// The zenith angle is, by definition, 90 degrees minus the elevation angle.
	e := Elevation(465, 0, loc)
	z := Zenith(465, 0, loc)
	if z != (DegToRad*90.0)-e {
		t.Errorf("Expecting zenith to be Elevation relative to the vertical, got %f", z)
	}
}

func TestAzimuth(t *testing.T) {
	// The azimuth for noon on Jan 1 should be ~ 180 degrees (someplace between
	// 176 and 179 depending)
	a := Azimuth(720, 0, loc)
	if int(RadToDeg*a) != 179 {
		t.Errorf("Expecting ~ 180, got %f", RadToDeg*a)
	}
}

func TestAM(t *testing.T) {
	// Noon on an equinox should be as close as directly overhead as possible,
	// meaning as close to 1 as possible.
	am := AM(720, 79, loc)
	if int(am) != 1 {
		t.Errorf("Expecting to be close to 1, got %f", am)
	}

	// And sun-up on a solstice should be the longest possible.
	am = AM(465, 179, loc)
	if int(am+0.5) != 2 {
		t.Errorf("Expecting close to 2, got %f", am)
	}
}

func TestModulePower(t *testing.T) {
	// For noon at the hight of summer, we should expect close to 1 kw/m^2
	p := ModulePower(720, 171, loc)
	if int(p) != 1 {
		t.Errorf("Expected close to 1, got %f", p)
	}
}

func TestSunrise(t *testing.T) {
	// Sunrise on Jan. 1 should be close to 7:37 am (457 minutes).
	// It actually changes depending on year, but we are taking some broad
	// sweeps, here...
	s := Sunrise(1, loc)
	if int(s) != 457 {
		t.Errorf("Expecting close to 457, got %f", s)
	}
}

func TestSunset(t *testing.T) {
	// 4:22 pm = 982 minutes
	s := Sunset(1, loc)
	if int(s) != 982 {
		t.Errorf("Expecing close to 982, got %f", s)
	}
}

func TestSuntime(t *testing.T) {
	// Should get about 8.5 hours
	s := SunTime(1, loc)
	if int(s) != 525 {
		t.Errorf("Expecting close to 525, got %f", s)
	}
}

func TestPeakSolarHours(t *testing.T) {
	// For the hight of summer, we should get somewhere around 10 PSH's (not
	// including weather conditions)
	p := PeakSolarHours(171, loc)
	if int(p) != 10 {
		t.Errorf("Expected close to 10, got %f", p)
	}
}

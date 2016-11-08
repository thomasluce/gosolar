package gosolar

import (
	"math"

	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
)

const DegToRad = math.Pi / 180.0
const RadToDeg = 180.0 / math.Pi

const Circle = DegToRad * 360.0
const DaysPerYear = 365.0

// Location holds information about where in the world we are. City and State
// are US specific.
type Location struct {
	Lat, Lon, Alt float64
	City          string
	State         string
}

// LSTM returns the Local Standard Time Meridian based on your difference in time
// from GMT in hours. It returns in radians
func LSTM(timezone float64) float64 {
	return DegToRad * 15.0 * timezone
}

// EoT returns the Equation of Time for the given day of the year. This is the
// number of minutes off, for a given day of the year, solar time is from
// clock-time based on orbital eccentricity and axial tilt. It is an imperical
// equation based on observation and fitting to that. Given that, the magic
// co-efficients are just that: magic and unit-less. It is also regardless of
// location; we factor that in elsewhere.
func EoT(day int) float64 {
	b := (Circle / DaysPerYear) * (float64(day) - 81.0)
	return (9.87 * math.Sin(2*b)) - (7.53 * math.Cos(b)) - (1.5 * math.Sin(b))
}

// TimezoneFor returns the timezone offset from GMT based on the longitude.
// For our purposes, we want something relatively accurate (as apposed to
// literal timezone, which is largely politically motivated), so we base it on
// the idea that the earth rotates 15 degrees per hour. Even though it will be
// off by ~5 seconds of angle, we'll just assume Greenwhich is at 0 degrees
// longitude.
func TimezoneFor(loc Location) float64 {
	lon := loc.Lon
	if lon > 180.0 {
		lon = -(lon - 180.0)
	}

	return lon / 15.0
}

// TCF returns the Time Correction Factor. The Time Correction Factor is the
// number of minutes off from solar time, for a given day of the year, the
// clock will be based on longitude and other factors. Put another way, 12 noon
// solar time is when the sun is at its zenith regardless of what the
// wall-clock says. This function returns the number of minutes different
// between zenith-time and 12:00 noon.
func TCF(day int, loc Location) float64 {
	return 4*(DegToRad*loc.Lon-LSTM(TimezoneFor(loc))) + EoT(day)
}

// LST returns the local solar time. localTime is in minutes from midnight.
func LST(localTime int, day int, loc Location) float64 {
	return float64(localTime) + TCF(day, loc)/60.0
}

// HRA returns the Hour Angle. The Hour Angle is the angle that the sun moves across
// the sky on a given day of the year. By definition it is 0 degrees at noon,
// negative in the morning, and positive in the afternoon. This returns in radians
func HRA(localTime int, day int, loc Location) float64 {
	lst := LST(localTime, day, loc)
	return DegToRad * 0.25 * (lst - 720)
}

// Declination returns the declanation angle of the sun on a given day of the
// year. The declanation angle is the angle of tilt of the Earth's axis
// relative to its orbital plane.
func Declination(day int) float64 {
	return DegToRad * 23.45 * math.Sin((Circle/DaysPerYear)*(float64(day)-81.0))
}

// Elevation returns the elevation angle of the sun given a location, time of day, and day of
// year. The angle is measured relative to the horizontal, and is defined as 0
// at sunrise and 90 degrees when directly overhead (at the equator on an
// equinox). This function returns in radians.
func Elevation(localTime int, day int, loc Location) float64 {
	sinDsinLat := math.Sin(Declination(day)) * math.Sin(DegToRad*loc.Lat)
	cosDcosLat := math.Cos(Declination(day)) * math.Cos(DegToRad*loc.Lat)
	cosH := math.Cos(HRA(localTime, day, loc))

	s := math.Asin(sinDsinLat + (cosDcosLat * cosH))
	return s
}

// Zenith returns the zenith angle, which is the same as elevation, but
// measured from the vertical instead of from the horizontal (as with
// elevation)
func Zenith(localTime int, day int, loc Location) float64 {
	return DegToRad*90.0 - Elevation(localTime, day, loc)
}

// Azimuth returns the azimuth of the sun in the sky given a particular
// location and time of day and year. This is the compass reading of the sun
// projected onto a plane from above. 0 degrees is N, and 180 degrees is S.
// This is shifted somewhat for the solar afternoon. Returned in Radians.
func Azimuth(localTime int, day int, loc Location) float64 {
	dec := Declination(day)
	lat := loc.Lat * DegToRad
	hourAngle := HRA(localTime, day, loc)
	zenith := Zenith(localTime, day, loc)

	cosTheta := math.Sin(dec) * math.Cos(lat)
	cosTheta -= math.Cos(hourAngle) * math.Cos(dec) * math.Sin(lat)
	cosTheta /= math.Sin(zenith)
	theta := math.Acos(cosTheta)

	if LST(localTime, day, loc) < 720 {
		return theta
	}
	return DegToRad*360 - theta

	return theta
}

// AM returns the Air Mass, which is the amount of air that a beam of light
// from the sun has to pass through to get to the ground at a time of day and
// year. It is in standard units defined as the distance from the top of the
// atmosphere to sea-level at noon being equal to 1.
func AM(localTime int, day int, loc Location) float64 {
	theta := Zenith(localTime, day, loc)
	x := DegToRad*96.07995 - theta
	if x < 0.0 {
		return 0
	}

	d := math.Cos(theta) + (DegToRad * 0.50572 * math.Pow(x, DegToRad*-1.6364))
	return 1.0 / d
}

// ID returns the Direct Intensity of the light; the strength of the light at a
// given place, time, and altitude. It is measured in kW/m^2 and factors in the
// atmospheric and solar elevation/angle. A complete explanation of all the
// constants used in this and related functions can be found here:
// http://www.pveducation.org/pvcdrom/properties-of-sunlight/air-mass
// This version assumes that the light from the sun is striking a surface
// exactly perpendicular to the light.
func ID(localTime int, day int, loc Location) float64 {
	am := AM(localTime, day, loc)
	if am <= 0.0 {
		return 0.0
	}

	p := math.Pow(am, 0.678)
	p = math.Pow(0.7, p)
	ah := 0.14 * loc.Alt

	return 1.353 * ((1.0-ah)*p + ah)
}

// IG returns the Global Intensity; 1.1 * the direct inensity, as we get about
// a 10% boost from scattering.
func IG(localTime int, day int, loc Location) float64 {
	return 1.1 * ID(localTime, day, loc)
}

// ModulePower returns the amount of sunlight, in Kw/m^2, that lands on a panel
// that is tilted to the same angle as the latitude of the location (which is
// optimal.)
func ModulePower(localTime int, day int, loc Location) float64 {
	e := Elevation(localTime, day, loc)
	id := IG(localTime, day, loc)
	sHoriz := id * math.Sin(e+DegToRad*loc.Lat)
	return sHoriz / math.Sin(e)
}

// SunTime returns the amount of time that the sun is shining during the course
// of a given day, in minutes.
func SunTime(day int, loc Location) float64 {
	return Sunset(day, loc) - Sunrise(day, loc)
}

// Sunrise returns the time of the sunrise in local-solar-time (not corrected) in
// minutes past midnight for a given day.
func Sunrise(day int, loc Location) float64 {
	a := 1.0 / 0.25 * DegToRad
	dec := Declination(day)
	lat := loc.Lat * DegToRad
	b := (-math.Sin(lat) * math.Sin(dec)) / (math.Cos(lat) * math.Cos(dec))
	return (12 - (a * math.Acos(b) * RadToDeg)) * 60
}

// Sunset returns the time of the sunset in local-solar-time (not corrected) in
// minutes past midnight for a given day.
func Sunset(day int, loc Location) float64 {
	a := 1.0 / 0.25 * DegToRad
	dec := Declination(day)
	lat := loc.Lat * DegToRad
	b := (-math.Sin(lat) * math.Sin(dec)) / (math.Cos(lat) * math.Cos(dec))
	return (12 + (a * math.Acos(b) * RadToDeg)) * 60
}

// PeakSolarHours returns the cumulative number of hours in a day, for a given
// location, where 1kW of useful solar radiation reaches 1 m^2 of ground. So,
// if for 12 hours only 0.5 kW reaches the ground, then there are 6 peak solar
// hours.
func PeakSolarHours(day int, loc Location) (sum float64) {
	// We step forward one hour at a time through the day, and add up the total
	// amount of energy in kW/m^2
	sr := int(Sunrise(day, loc))
	for i := sr; i < int(Sunset(day, loc)); i++ {
		sum += IG(sr+i, day, loc)
	}
	return sum / 60
}

func stringInSlice(a string, list []string) bool {
	for _, s := range list {
		if a == s {
			return true
		}
	}
	return false
}

func cityStateFromGeocodingResult(resp []maps.GeocodingResult) (string, string) {
	var city, state string
	for _, component := range resp[0].AddressComponents {
		if stringInSlice("locality", component.Types) && stringInSlice("political", component.Types) {
			city = component.ShortName
		}

		if stringInSlice("political", component.Types) && stringInSlice("administrative_area_level_1", component.Types) {
			state = component.LongName
		}
	}
	return city, state
}

// FindLocation finds a given location using googl'e geocoding api's and
// returns a Location struct or an error.
func FindLocation(apiKey string, location string) (Location, error) {
	l := Location{}

	c, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return l, err
	}
	req := &maps.GeocodingRequest{
		Address: location,
	}

	resp, err := c.Geocode(context.Background(), req)
	if err != nil {
		return l, err
	}

	l.Lat = resp[0].Geometry.Location.Lat
	l.Lon = resp[0].Geometry.Location.Lng
	l.City, l.State = cityStateFromGeocodingResult(resp)

	ereq := &maps.ElevationRequest{
		Locations: []maps.LatLng{
			maps.LatLng{
				Lat: l.Lat,
				Lng: l.Lon,
			},
		},
	}

	elevations, err := c.Elevation(context.Background(), ereq)
	if err != nil {
		// TODO: maybe partial returns are okay...?
		return l, err
	}

	// We get things in meters, so convert to KM
	l.Alt = elevations[0].Elevation / 1000.0

	return l, nil
}

// LatLonAltForLocation returns the latitude, longitude, and altitude for a
// location specified in plain text. This uses the google maps api.
func LatLonAltForLocation(apiKey string, location string) (float64, float64, float64) {
	c, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		panic(err.Error())
	}
	req := &maps.GeocodingRequest{
		Address: location,
	}

	resp, err := c.Geocode(context.Background(), req)
	if err != nil {
		panic(err.Error())
	}

	lat := resp[0].Geometry.Location.Lat
	lon := resp[0].Geometry.Location.Lng

	ereq := &maps.ElevationRequest{
		Locations: []maps.LatLng{
			maps.LatLng{
				Lat: lat,
				Lng: lon,
			},
		},
	}

	elevations, err := c.Elevation(context.Background(), ereq)
	if err != nil {
		panic(err.Error())
	}

	// We get things in meters, so convert to KM
	elevation := elevations[0].Elevation / 1000.0

	return lat, lon, elevation
}

package gosolar

import (
	"math"

	"golang.org/x/net/context"
	"googlemaps.github.io/maps"
)

const DegToRad = math.Pi / 180.0

// Location holds information about where in the world we are. City and State
// are US specific.
type Location struct {
	Lat, Lon, Alt float64
	City          string
	State         string
}

// LSTM returns the Local Standard Time Meridian based on your difference in time
// from GMT in hours.
func LSTM(timezone float64) float64 {
	return 15 * timezone
}

// EoT returns the Equation of Time for the given day of the year. This is the
// number of minutes off, for a given day of the year, solar time is from
// clock-time based on orbital eccentricity and axial tilt.
func EoT(day int) float64 {
	b := (360.0 / 365.0) * (float64(day) - 81.0)
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
// clock will be based on longitude and other factors.
func TCF(day int, loc Location) float64 {
	return 4*(loc.Lon-LSTM(TimezoneFor(loc))) + EoT(day)
}

// LST returns the local solar time. localTime is in minutes from midnight.
func LST(localTime int, day int, loc Location) float64 {
	return float64(localTime) + (TCF(day, loc) / 60.0)
}

// HRA returns the Hour Angle. The Hour Angle is the angle that the sun moves across
// the sky on a given day of the year. By definition it is 0 degrees at noon,
// negative in the morning, and positive in the afternoon.
func HRA(localTime int, day int, loc Location) float64 {
	return 15 * (LST(localTime, day, loc) - 12.0)
}

// Declanation returns the declanation angle of the sun on a given day of the year.
func Declanation(day int) float64 {
	return 23.45 * math.Sin((360.0/365.0)*(float64(day)-81.0))
}

// Elevation returns the elevation angle of the sun given a location, time of day, and day of
// year.
func Elevation(localTime int, day int, loc Location) float64 {
	sinDsinLat := math.Sin(Declanation(day)) * math.Sin(loc.Lat)
	cosDcosLat := math.Cos(Declanation(day)) * math.Cos(loc.Lat)
	cosH := math.Cos(HRA(localTime, day, loc))

	s := math.Sin(sinDsinLat + (cosDcosLat * cosH))
	return 1.0 / s
}

// Zenith returns the zenith angle, which is the same as elevation, but
// measured from the vertical instead of from the horizontal (as with
// elevation)
func Zenith(localTime int, day int, loc Location) float64 {
	return 90.0 - Elevation(localTime, day, loc)
}

// Azimuth returns the azimuth of the sun in the sky given a particular
// location and time of day and year. This is the compass reading of the sun
// projected onto a plane from above. 0 degrees is N, and 180 degrees is S.
// This is shifted somewhat for the solar afternoon.
func Azimuth(localTime int, day int, loc Location) float64 {
	theta := Zenith(localTime, day, loc)
	sinDec := math.Sin(Declanation(day))
	cosTheta := math.Cos(theta)
	cosDec := math.Cos(Declanation(day))
	sinTheta := math.Sin(theta)
	cosH := math.Cos(HRA(localTime, day, loc))
	a := Elevation(localTime, day, loc)

	azPrime := math.Cos(((sinDec * cosTheta) - (cosDec * sinTheta * cosH)) / a)
	az := 1.0 / azPrime

	if LST(localTime, day, loc) < 12 || HRA(localTime, day, loc) < 0 {
		return az
	}

	return 360.0 - az
}

// AM returns the Air Mass, which is the amount of air that a beam of light
// from the sun has to pass through to get to the ground at a time of day and
// year. It is in standard units defined as the distance from the top of the
// atmosphere to sea-level at noon being equal to 1.
func AM(localTime int, day int, loc Location) float64 {
	theta := Zenith(localTime, day, loc)
	x := 96.07995 - theta
	if x < 0.0 {
		return 0
	}

	d := math.Cos(theta) + (0.50572 * math.Pow(x, -1.6364))
	return 1.0 / d
}

// ID returns the Direct Intensity of the light; the strength of the light at a
// given place, time, and altitude. It is measured in kW/m^2 and factors in the
// atmospheric and solar elevation/angle. A complete explanation of all the
// constants used in this and related functions can be found here:
// http://www.pveducation.org/pvcdrom/properties-of-sunlight/air-mass
func ID(localTime int, day int, loc Location) float64 {
	am := AM(localTime, day, loc)
	if am <= 0.0 {
		return 0.0
	}

	p := math.Pow(am, 0.678)
	return 1.353 * ((1.0-(0.14*loc.Alt))*math.Pow(0.7, p) + (0.14 * loc.Alt))
}

// IG returns the Global Intensity; 1.1 * the direct inensity, as we get about
// a 10% boost from scattering.
func IG(localTime int, day int, loc Location) float64 {
	return 1.1 * ID(localTime, day, loc)
}

// PeakSolarHours returns the cumulative number of hours in a day, for a given
// location, where 1kW of useful solar radiation reaches 1 m^2 of ground. So,
// if for 12 hours only 0.5 kW reaches the ground, then there are 6 peak solar
// hours.
func PeakSolarHours(day int, loc Location) float64 {
	// We step forward one hour at a time through the day, and add up the total
	// amount of energy in kW/m^2
	sum := 0.0
	// 60*24 == number of minutes in a day
	for i := 0; i < 60*24; i++ {
		sum += IG(i, day, loc)
	}

	return sum / 60.0 / 24.0
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

	l.Alt = elevations[0].Elevation

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

	elevation := elevations[0].Elevation

	return lat, lon, elevation
}

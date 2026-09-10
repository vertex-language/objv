package sysroot

import (
	"strconv"
	"strings"
)

// A Version is an Apple OS or SDK version: three components, of which the
// third is usually zero and usually omitted.
//
// The zero Version means "none stated", which is why IsZero exists and why
// nothing here treats 0.0.0 as a real version — every macOS there has ever
// been is at least 10.0.
type Version struct {
	Major, Minor, Patch int
}

func (v Version) IsZero() bool { return v == Version{} }

// String is the spelling a command line uses: "13.2", or "13.2.1" when the
// patch is meaningful.
func (v Version) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor)
	if v.Patch != 0 {
		s += "." + strconv.Itoa(v.Patch)
	}
	return s
}

// Triple is the three-component spelling ld64's -platform_version wants,
// which states the patch whether or not it is zero.
func (v Version) Triple() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

// Less reports whether v precedes w.
func (v Version) Less(w Version) bool {
	if v.Major != w.Major {
		return v.Major < w.Major
	}
	if v.Minor != w.Minor {
		return v.Minor < w.Minor
	}
	return v.Patch < w.Patch
}

// ParseVersion reads "13", "13.2" or "13.2.1". A component that is not a
// number ends the parse, so "26.4.99" reads and "10.15-beta" reads as 10.15.
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, false
	}
	parts := strings.SplitN(s, ".", 4)
	var out [3]int
	n := 0
	for i := 0; i < len(parts) && i < 3; i++ {
		v, err := strconv.Atoi(parts[i])
		if err != nil {
			break
		}
		out[i] = v
		n++
	}
	if n == 0 {
		return Version{}, false
	}
	return Version{out[0], out[1], out[2]}, true
}

// MacroValue is the encoding Apple's Availability.h compares against:
// major*10000 + minor*100 + patch.
//
//	10.13    →  101300
//	10.15.4  →  101504
//	26.4     →  260400
//
// It is the value of __ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__, which is
// what AvailabilityMacros.h and the __OSX_AVAILABLE family read, and it is
// therefore what decides whether a Cocoa header declares a method at all.
// A minor or patch above 99 cannot be encoded and does not occur; nothing
// here clamps, because a version that needed clamping would be a version
// Apple's own headers could not describe either.
func (v Version) MacroValue() int {
	return v.Major*10000 + v.Minor*100 + v.Patch
}

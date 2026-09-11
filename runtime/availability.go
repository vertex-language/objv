package runtime

import "strconv"

// @available, and what it becomes.
//
// §6.10's `@available(macOS 12.0, *)` asks a question about the machine the
// program is running on, not the one it was built on: an image with a
// deployment target of macOS 11 may be launched on 12, and the check is how
// it finds out. The trailing `*` stands for every platform not named and
// makes the check succeed there, which is what lets one source file carry
// checks for platforms it is not being built for.
//
// It is a compile-time constant whenever the deployment target already
// answers it. A program built for macOS 12 that asks whether it is on macOS
// 12 is asking about something the linker recorded, and clang folds it; so
// does lower.
//
// The runtime call is _availability_version_check, which is libSystem's.
// clang emits __isPlatformVersionAtLeast instead, which is compiler-rt's
// wrapper around this same function with a fallback for systems too old to
// have it — macOS before 10.15. objv links libSystem and not compiler-rt,
// and its Darwin deployment floor is above that fallback's range, so it
// calls the one underneath.

// Platform is the platform field of Mach-O's LC_BUILD_VERSION, which is also
// what _availability_version_check takes. A simulator and a device are
// distinct platforms rather than a flag on one.
type Platform uint32

const (
	PlatformUnknown           Platform = 0
	PlatformMacOS             Platform = 1
	PlatformIOS               Platform = 2
	PlatformTVOS              Platform = 3
	PlatformWatchOS           Platform = 4
	PlatformBridgeOS          Platform = 5
	PlatformMacCatalyst       Platform = 6
	PlatformIOSSimulator      Platform = 7
	PlatformTVOSSimulator     Platform = 8
	PlatformWatchOSSimulator  Platform = 9
	PlatformDriverKit         Platform = 10
	PlatformVisionOS          Platform = 11
	PlatformVisionOSSimulator Platform = 12
)

// platformNames are the spellings §6.10 admits, lowercased.
//
// Both of everything: `macOS` is the name Apple's headers use and `macos`
// the one the clang driver does, `OSX` and `macosx` are the older ones, and
// a program that says any of them means the same platform.
var platformNames = map[string]Platform{
	"macos":             PlatformMacOS,
	"macosx":            PlatformMacOS,
	"osx":               PlatformMacOS,
	"ios":               PlatformIOS,
	"iphoneos":          PlatformIOS,
	"tvos":              PlatformTVOS,
	"watchos":           PlatformWatchOS,
	"bridgeos":          PlatformBridgeOS,
	"maccatalyst":       PlatformMacCatalyst,
	"uikitformac":       PlatformMacCatalyst,
	"iossimulator":      PlatformIOSSimulator,
	"tvossimulator":     PlatformTVOSSimulator,
	"watchossimulator":  PlatformWatchOSSimulator,
	"driverkit":         PlatformDriverKit,
	"visionos":          PlatformVisionOS,
	"xros":              PlatformVisionOS,
	"visionossimulator": PlatformVisionOSSimulator,
	"xrossimulator":     PlatformVisionOSSimulator,
}

// PlatformNamed resolves an availability clause's platform name. The
// comparison ignores case, because the names in circulation differ only in
// it and a program that writes macOS means what one that writes macos does.
func PlatformNamed(name string) (Platform, bool) {
	p, ok := platformNames[lower(name)]
	return p, ok
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// OSVersion is a platform version: three optional numbers, of which only the
// first is required.
type OSVersion struct{ Major, Minor, Patch int }

// ParseOSVersion reads `12`, `12.1` or `12.1.3`. A component that is not a
// number, an empty one, or a fourth is a version nobody wrote.
func ParseOSVersion(s string) (OSVersion, bool) {
	var v OSVersion
	out := []*int{&v.Major, &v.Minor, &v.Patch}
	n, start := 0, 0
	for i := 0; i <= len(s); i++ {
		if i < len(s) && s[i] != '.' {
			continue
		}
		if n == len(out) || i == start {
			return OSVersion{}, false
		}
		num, err := strconv.Atoi(s[start:i])
		if err != nil || num < 0 {
			return OSVersion{}, false
		}
		*out[n] = num
		n++
		start = i + 1
	}
	return v, n > 0
}

// AtLeast reports whether v is w or newer, which is the comparison
// @available makes.
func (v OSVersion) AtLeast(w OSVersion) bool {
	if v.Major != w.Major {
		return v.Major > w.Major
	}
	if v.Minor != w.Minor {
		return v.Minor > w.Minor
	}
	return v.Patch >= w.Patch
}

func (v OSVersion) IsZero() bool { return v == OSVersion{} }

// Encoded is the packed form _availability_version_check reads: the three
// components in one word, a byte each below the major's sixteen bits.
func (v OSVersion) Encoded() uint32 {
	return uint32(v.Major)<<16 | uint32(v.Minor&0xff)<<8 | uint32(v.Patch&0xff)
}

// AvailabilityCheck is libSystem's entry point:
//
//	bool _availability_version_check(uint32_t count, dyld_build_version_t versions[]);
//
// where a dyld_build_version_t is a platform and an encoded version.
const AvailabilityCheck = "_availability_version_check"

// BuildVersion is dyld_build_version_t, which the check takes an array of.
var BuildVersion = []Field{
	{U32, "platform"},
	{U32, "version"},
}

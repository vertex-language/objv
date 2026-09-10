package sysroot

import (
	"encoding/json"
	"strings"
)

// The macOS SDK.
//
// /usr/include does not exist on a modern macOS. Headers live only inside an
// SDK, and so do the stub libraries a Mach-O link reads — the real dylibs
// were replaced by the shared cache and are not files any more. One SDK
// therefore answers both halves, which is why the path is found once and
// carried in Result rather than looked up again at link time.

// SDK is the macOS SDK this build compiles and links against.
type SDK struct {
	// Path is the .sdk directory. It is what -syslibroot is given.
	Path string

	// Version is the SDK's own version, from SDKSettings.json. It is the
	// second version in ld64's -platform_version, and it is what a
	// program's LC_BUILD_VERSION records as the SDK it was built with.
	Version Version

	// MinimumDeployment and DefaultDeployment are what the SDK says it
	// supports: the oldest OS it will still declare for, and the one its
	// own tools default to. Both come from SupportedTargets.macosx.
	MinimumDeployment Version
	DefaultDeployment Version

	// Archs are the architectures the SDK carries, spelled Apple's way
	// ("arm64", "x86_64"). A target the SDK does not list still resolves;
	// the note says what happened.
	Archs []string
}

func (s SDK) Found() bool { return s.Path != "" }

// darwinResolve fills in a Result for a macOS target.
func darwinResolve(h Host, opt Options, r *Result) {
	sdk, ok := darwinSDK(h, opt.SDKPath)
	if !ok {
		r.Notes = append(r.Notes,
			"no macOS SDK found: set SDKROOT, pass -isysroot, or install the command line tools (xcode-select --install)")
		return
	}
	r.SDK = sdk

	// The order clang uses with a sysroot in force. /usr/local/include is
	// deliberately absent: clang drops it once -isysroot is set, and a
	// package manager's prefix is the user's to name with -I.
	r.Include = append(r.Include, dirEntries(h, []string{sdk.Path + "/usr/include"})...)

	// SubFrameworks is not an exotic case. AppKit's own private pieces
	// live there, and a header inside a framework reaches its siblings
	// through it.
	r.Frameworks = append(r.Frameworks, dirEntries(h, []string{
		sdk.Path + "/System/Library/Frameworks",
		sdk.Path + "/System/Library/SubFrameworks",
	})...)

	r.LibraryDirs = existing(h, []string{
		"/usr/local/lib",
		sdk.Path + "/usr/lib",
	})

	// libSystem re-exports libobjc on Darwin, which is why clang links an
	// Objective-C program with -lSystem and nothing else — checked by
	// reading what `clang -v` hands ld for a file with a class in it.
	r.Libraries = []string{"System"}

	r.Deployment = darwinDeployment(h, opt, sdk, r)

	if arch := appleArch(archOf(opt.Target)); arch != "" && len(sdk.Archs) > 0 {
		if !contains(sdk.Archs, arch) {
			r.Notes = append(r.Notes,
				"the SDK at "+sdk.Path+" does not list "+arch+" among its architectures")
		}
	}
}

// darwinSDK is the four-step lookup, in the order the platform's own tools
// use it:
//
//  1. -isysroot, which is the caller saying so outright;
//  2. $SDKROOT — set by xcrun and by Xcode-driven builds, so honouring it
//     means objv composes under both;
//  3. `xcrun --show-sdk-path` — the authoritative answer. Apple owns the
//     developer-directory walk behind it (DEVELOPER_DIR, the xcode-select
//     symlink, the app-path fallbacks) and changes it between releases, so
//     objv asks rather than reimplements;
//  4. the Command Line Tools SDK at its fixed path, for a machine with the
//     tools installed but xcrun not answering.
func darwinSDK(h Host, override string) (SDK, bool) {
	path := override
	if path == "" {
		path = h.Getenv("SDKROOT")
	}
	if path == "" {
		if out, err := h.Run("xcrun", "--show-sdk-path"); err == nil {
			path = strings.TrimSpace(out)
		}
	}
	if path == "" {
		const clt = "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk"
		if h.IsDir(clt) {
			path = clt
		}
	}
	if path == "" || !h.IsDir(path) {
		return SDK{}, false
	}
	sdk := SDK{Path: path}
	readSDKSettings(h, &sdk)
	return sdk, true
}

// sdkSettings is the part of SDKSettings.json this package reads. The file
// carries a great deal more — code-signing defaults, sanitizer flags — and
// none of it is a compiler's business.
type sdkSettings struct {
	Version          string `json:"Version"`
	SupportedTargets map[string]struct {
		MinimumDeploymentTarget string   `json:"MinimumDeploymentTarget"`
		DefaultDeploymentTarget string   `json:"DefaultDeploymentTarget"`
		Archs                   []string `json:"Archs"`
	} `json:"SupportedTargets"`
}

// readSDKSettings fills in what the SDK says about itself.
//
// An SDK with no readable settings is still an SDK: the headers are there and
// the versions fall back to the caller's or the host's. Refusing one over a
// missing JSON file would turn a cosmetic gap into "no SDK found".
func readSDKSettings(h Host, sdk *SDK) {
	body, err := h.ReadFile(sdk.Path + "/SDKSettings.json")
	if err != nil {
		return
	}
	var s sdkSettings
	if json.Unmarshal([]byte(body), &s) != nil {
		return
	}
	sdk.Version, _ = ParseVersion(s.Version)
	t := s.SupportedTargets["macosx"]
	sdk.MinimumDeployment, _ = ParseVersion(t.MinimumDeploymentTarget)
	sdk.DefaultDeployment, _ = ParseVersion(t.DefaultDeploymentTarget)
	sdk.Archs = t.Archs
}

// darwinDeployment settles the oldest OS this build will run on.
//
// The order is clang's: -mmacosx-version-min, then
// $MACOSX_DEPLOYMENT_TARGET, then the version of the machine doing the
// building. The last one surprises people who expect the SDK's default, and
// it is what clang does — a build with no opinion targets the machine it is
// running on, not the newest OS the SDK knows about.
//
// Whatever comes out is then raised to the floor the architecture imposes.
// Apple Silicon did not exist before macOS 11, so an arm64 build asking for
// 10.13 is asking for something that cannot be: clang silently raises it, and
// so does this, with a note.
func darwinDeployment(h Host, opt Options, sdk SDK, r *Result) Version {
	v := opt.Deployment
	if v.IsZero() {
		v, _ = ParseVersion(h.Getenv("MACOSX_DEPLOYMENT_TARGET"))
	}
	if v.IsZero() {
		v = hostMacOS(h)
	}
	if v.IsZero() {
		v = sdk.DefaultDeployment
	}
	if floor := archFloor(archOf(opt.Target)); v.Less(floor) {
		if !v.IsZero() && !opt.Deployment.IsZero() {
			r.Notes = append(r.Notes, "macOS "+v.String()+" predates "+
				archOf(opt.Target)+"; the deployment target is "+floor.String())
		}
		v = floor
	}
	return v
}

// archFloor is the oldest macOS an architecture ever ran.
func archFloor(arch string) Version {
	if arch == "aarch64" || arch == "arm64" {
		return Version{Major: 11}
	}
	return Version{Major: 10, Minor: 13}
}

// hostMacOS is the version of the machine doing the building, truncated the
// way clang truncates it: a build with no stated target aims at the major
// release, not at the point update it happens to be running.
func hostMacOS(h Host) Version {
	out, err := h.Run("sw_vers", "-productVersion")
	if err != nil {
		return Version{}
	}
	v, ok := ParseVersion(out)
	if !ok {
		return Version{}
	}
	if v.Major >= 11 {
		return Version{Major: v.Major}
	}
	return Version{Major: v.Major, Minor: v.Minor}
}

// appleArch translates objv's architecture name to Apple's. The two agree
// about x86_64 and disagree about the other one: objv says aarch64, which is
// the architecture's name, and Apple says arm64, which is the name in every
// SDK manifest and every -arch flag.
func appleArch(arch string) string {
	switch arch {
	case "aarch64":
		return "arm64"
	case "x86_64":
		return "x86_64"
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

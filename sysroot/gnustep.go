package sysroot

import "strings"

// The GNUstep path: Objective-C on a platform Apple never shipped one for.
//
// Everything Darwin gets from one SDK is three separate things here. The
// runtime is libobjc2, installed like any other library; Foundation is
// GNUstep Base, installed like any other library; and there are no
// frameworks at all — GNUstep's headers are ordinary directories, so
//
//	#import <Foundation/Foundation.h>
//
// resolves as a plain include of Foundation/Foundation.h rather than as a
// framework lookup. That is not a special case in the preprocessor: a
// framework include is one whose first component names a framework, and where
// no framework directory holds a Foundation.framework the ordinary search
// finds the ordinary directory. It works because the spelling is the same.
//
// GNUSTEP_MAKEFILES and the gnustep-config tool are how a GNUstep
// installation says where it put itself, and both are honoured for the same
// reason $SDKROOT is on Darwin: the platform has an answer and inventing a
// second one would disagree with it.

// gnustepResolve fills in a Result for a target that uses libobjc2.
func gnustepResolve(h Host, opt Options, r *Result) {
	root := gnustepRoot(h)
	if root != "" {
		r.Include = append(r.Include, dirEntries(h, []string{root + "/include"})...)
	}

	// The well-known directories, probed rather than assumed. An absent
	// one is an absent entry and no distro table decides anything.
	dirs := []string{"/usr/local/include", "/usr/include/GNUstep", "/usr/include"}
	if tuple := multiarch[archOf(opt.Target)]; tuple != "" && osOf(opt.Target) == "linux" {
		dirs = append(dirs, "/usr/include/"+tuple)
	}
	r.Include = append(r.Include, dirEntries(h, dirs)...)

	libs := []string{"/usr/local/lib"}
	if root != "" {
		libs = append(libs, root+"/lib")
	}
	if tuple := multiarch[archOf(opt.Target)]; tuple != "" {
		libs = append(libs, "/usr/lib/"+tuple)
	}
	switch archOf(opt.Target) {
	case "x86_64", "aarch64":
		libs = append(libs, "/usr/lib64")
	}
	libs = append(libs, "/usr/lib")
	r.LibraryDirs = existing(h, libs)

	// Three names where Darwin needs one, because nothing here re-exports
	// anything: objc is libobjc2, gnustep-base is Foundation, and the
	// platform's own C runtime is what both stand on. A program that wants
	// none of them says --freestanding.
	r.Libraries = []string{"objc", "gnustep-base"}

	if !hasAny(h, []string{"/usr/include/objc/objc.h", "/usr/local/include/objc/objc.h"}) &&
		root == "" {
		r.Notes = append(r.Notes,
			"no GNUstep installation found: install libobjc2 and gnustep-base, or set GNUSTEP_SYSTEM_ROOT")
	}
}

// gnustepRoot is where a GNUstep installation says it put itself.
func gnustepRoot(h Host) string {
	if root := h.Getenv("GNUSTEP_SYSTEM_ROOT"); root != "" && h.IsDir(root) {
		return root
	}
	if out, err := h.Run("gnustep-config", "--variable=GNUSTEP_SYSTEM_ROOT"); err == nil {
		if root := strings.TrimSpace(out); root != "" && h.IsDir(root) {
			return root
		}
	}
	return ""
}

// hasAny reports whether any of the paths' parent directories exist. It is
// asked about a header rather than a directory because the question is
// whether the *runtime* is installed, and /usr/include always exists.
func hasAny(h Host, paths []string) bool {
	for _, p := range paths {
		if i := strings.LastIndexByte(p, '/'); i > 0 {
			if h.IsDir(p[:i]) {
				return true
			}
		}
	}
	return false
}

// multiarch maps an architecture to Debian's multiarch tuple: the directory
// under /usr/include and /usr/lib where Debian-family distros keep the
// architecture-specific files.
//
// Fedora, Arch and Alpine do not use multiarch. No table entry or distro
// check handles that: the directory does not exist there, and an absent
// directory is an absent entry.
var multiarch = map[string]string{
	"x86_64":  "x86_64-linux-gnu",
	"i386":    "i386-linux-gnu",
	"aarch64": "aarch64-linux-gnu",
	"arm":     "arm-linux-gnueabihf",
	"riscv64": "riscv64-linux-gnu",
}

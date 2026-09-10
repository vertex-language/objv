package preprocessor

import "testing"

// §6.4.8's pp-number runs through identifier characters, and Apple's
// availability macros depend on it: CF_AVAILABLE(10_0, 2_0) pastes its
// argument onto __MAC_ and then onto __AVAILABILITY_INTERNAL, and a `10_0`
// scanned as two tokens produces __AVAILABILITY_INTERNAL__MAC_10 followed by
// a stray _0 — which is not the macro name it meant, so nothing expands and
// the attribute reaches the parser as an unknown type.
func TestPasteOntoPPNumber(t *testing.T) {
	wantOut(t, Config{}, `
#define TARGET10_0 hit
#define INNER(x) TARGET##x
INNER(10_0)`, "hit")

	// The chain CoreFoundation actually walks: the pasted name is itself a
	// macro, and the result is pasted again one level up.
	wantOut(t, Config{}, `
#define __MAC_10_0 1000
#define __AVAILABILITY_INTERNAL__MAC_10_0 ATTR
#define __OSX_AVAILABLE_STARTING(_osx, _ios) __AVAILABILITY_INTERNAL##_osx
#define CF_AVAILABLE(_mac, _ios) __OSX_AVAILABLE_STARTING(__MAC_##_mac, __IPHONE_##_ios)
CF_AVAILABLE(10_0, 2_0)`, "ATTR")
}

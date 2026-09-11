// §6.10's @available.
//
// The question is about the machine the program is *running* on: an image
// with a deployment target of macOS 11 may be launched on 12, and this is
// how it finds out. Two of the three answers are constants.

// A clause naming this platform that the deployment target already satisfies
// is true, and nothing is emitted: the comparison was settled when the
// deployment target was chosen. The corpus builds for macOS 12.
int old(void) {
    if (@available(macOS 10.9, *)) return 1;
    return 0;
}
// vir-not: availability_version_check

// A list naming no platform this image is for is true as well. That is what
// the trailing `*` means, and it is what lets one source file carry checks
// for platforms it is not being built for.
int elsewhere(void) {
    if (@available(iOS 13.0, watchOS 6.0, *)) return 1;
    return 0;
}
// vir-not: availability_version_check

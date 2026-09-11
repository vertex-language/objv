// §6.10's @available, when it is not a constant.
//
// The companion to 14-availability.m, which has the two cases that fold. A
// marker is a fact about the whole module, so the call and the absence of it
// cannot be asserted in one file.

// Only a clause about this platform asking for something newer than the
// deployment target becomes a call. 999 will do for a while.
int future(void) {
    if (__builtin_available(macOS 999.0, *)) return 1;
    return 0;
}
// The platform is macOS, which LC_BUILD_VERSION numbers 1, and the version
// is packed with the major above the minor's byte: 999 << 16.
// vir: i32.const 1
// vir: i32.const 65470464
// vir: call @__availability_version_check
// The check returns a C bool, whose upper bits are not promised.
// vir: i32.and

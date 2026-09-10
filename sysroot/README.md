# sysroot

`package sysroot` answers the question phase 4 cannot: where does this host
keep the target's headers, frameworks and libraries?

```
import "github.com/vertex-language/objv/sysroot"
```

```go
r := sysroot.Resolve(sysroot.Options{Target: "aarch64-macos", Hosted: true})
// r.Include, r.Frameworks, r.LibraryDirs, r.Libraries,
// r.SDK, r.Deployment, r.Notes
```

It is **data, not policy**. The `objv` package turns the result into
`preprocessor.Config` and linker arguments, and `objv env` prints it before
the build runs. This package imports the standard library only; the
preprocessor never learns what an SDK is, and this never learns what a token
is.

## Why an Objective-C compiler needs more of this than a C compiler

A C program can be compiled against no system headers at all. An Objective-C
one effectively cannot: the language's own literals are sends to Foundation
classes, and the first line of almost every file is

```objc
#import <Foundation/Foundation.h>
```

which is not a directory lookup but a **framework** lookup —
`Foundation.framework/Headers/Foundation.h` — and which reaches some nine
hundred headers whose every declaration is gated on a deployment target this
package has to determine. That is why `Result` carries a `Deployment` and an
SDK version where a C compiler's would carry only paths.

## The SDK is one thing, found once

```
-isysroot  →  $SDKROOT  →  xcrun --show-sdk-path  →  the Command Line Tools SDK
```

`/usr/include` does not exist on a modern macOS. Headers live only inside an
SDK, and so do the stub libraries a Mach-O link reads — the real dylibs were
replaced by the shared cache and are not files any more. One SDK answers both
halves, which is why the path is found once and carried in `Result` rather
than looked up again at link time: resolving it twice by two routes is how a
machine ends up compiling against one SDK and failing to link against
another.

`xcrun` is asked rather than reimplemented. Apple owns the developer-directory
walk behind it — `DEVELOPER_DIR`, the `xcode-select` symlink, the app-path
fallbacks — and changes it between releases.

`SDKSettings.json` supplies the rest: the SDK's version (the second version in
ld64's `-platform_version`), the oldest deployment target it supports, and the
architectures it carries. An SDK with no readable settings is still an SDK —
refusing one over a missing JSON file would turn a cosmetic gap into "no SDK
found".

## The deployment target

```
-mmacosx-version-min  →  $MACOSX_DEPLOYMENT_TARGET  →  the building machine
```

The last step surprises people who expect the SDK's default, and it is what
clang does: a build with no opinion targets the machine it is running on, not
the newest OS the SDK knows about. Whatever comes out is then raised to the
floor the architecture imposes — Apple Silicon did not exist before macOS 11,
so an arm64 build asking for 10.13 is asking for something that cannot be.

It matters more than a note in an Info.plist. `Version.MacroValue` is the
encoding Apple's availability headers compare against:

| written | macro value |
| --- | --- |
| `10.13` | `101300` |
| `10.15.4` | `101504` |
| `26.4` | `260400` |

That number is `__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__`, and a Cocoa
header whose method is newer than it **does not declare the method**.

## The predefines

`Predefines` returns the macros that are facts about *this platform, this SDK
and this deployment target* — not about the type model, which is
`types.Model`'s to state, and not about the language, which is the front end's.

Every one of them is load-bearing, and each was found by preprocessing
`<Foundation/Foundation.h>` and reading what broke without it:

| | |
| --- | --- |
| `__LITTLE_ENDIAN__` | `CFBase.h` and `NSByteOrder.h` `#error` without it. It is an architecture fact written the way Darwin writes it, and nowhere else writes it that way at all |
| `__APPLE_CC__` | `TargetConditionals.h` tests it beside `__GNUC__`, and says `#error unknown compiler` otherwise |
| `__arm64__` | CarbonCore's `fp.h` `#error`s on a CPU it does not recognize, and recognizes this spelling rather than `__aarch64__` |
| `__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__` | above |

## What the caller still has to answer

Two things this package deliberately does not decide, because they are facts
about the compiler and not about the host. Both were found the same way, and
both are worth writing down because getting them wrong fails *silently* —
`TargetConditionals.h` falls through to `#define TARGET_OS_MAC 0` rather than
complaining:

- **`__has_builtin(__is_target_arch)`** and its three neighbours must answer
  **true**. `TargetConditionals.h` uses them to decide `TARGET_OS_OSX`, and
  everything Foundation does with `#if TARGET_OS_OSX` follows.
- **`__has_extension(define_target_os_macros)`** must answer **false**. It
  means "the compiler predefines the `TARGET_OS_*` macros itself", which clang
  does and objv does not; answering true skips the block that would have
  defined them.

Beyond those, the feature table is large and it matters: `__has_feature(objc_fixed_enum)`
decides whether `NS_ENUM` produces a fixed underlying type or a bare typedef,
`__has_feature(objc_property_explicit_atomic)` decides whether
`NS_NONATOMIC_IOSONLY` expands to `atomic` or to nothing (and a property
attribute list with a hole in it is a syntax error), and
`__has_feature(objc_bridge_id)` decides whether CoreFoundation forward-declares
half of Foundation.

## Targets

| target | headers | runtime |
| --- | --- | --- |
| `aarch64-macos`, `x86_64-macos` | the macOS SDK | objc4, through `-lSystem` |
| `aarch64-linux`, `x86_64-linux` | GNUstep, probed | `-lobjc -lgnustep-base` |
| `x86_64-windows` | GNUstep, probed | as above |

On Darwin the link needs **one** library, because libSystem re-exports
libobjc — which is what `clang -v` hands `ld` for a file with a class in it.
On GNUstep it needs three, because nothing there re-exports anything.

There are no frameworks off Darwin. GNUstep's headers are ordinary
directories, so `#import <Foundation/Foundation.h>` resolves as a plain
include — which works without a special case, because a framework include is
one whose first component names a framework, and where no framework directory
holds a `Foundation.framework` the ordinary search finds the ordinary
directory.

## The builtin headers

`<stdarg.h>`, `<stddef.h>`, `<stdint.h>` and kin are compiler property, not
libc property. Apple's SDK ships none of them — clang's own
`usr/lib/clang/<v>/include` is the first directory on every Darwin include
list, and `builtin/` is objv's answer to it. They are mounted as `<builtin>`,
never an absolute path, so `__FILE__` and diagnostics read
`<builtin>/stdarg.h` on every machine identically.

They are written once, against target-parameterized predefined macros, so one
text serves every target; `builtin.go` lists the macros the `objv` package
must supply from `types.Model`.

Nothing Objective-C is among them. `id`, `Class`, `SEL`, `BOOL`, `nil` and
`YES` come from `<objc/objc.h>`, which belongs to the runtime and is shipped
by the SDK — clang carries no Objective-C header either, and a compiler that
shipped its own copy would be declaring the runtime's types on the runtime's
behalf.

## Notes, never errors

A host with no SDK still resolves — to the builtins — and says what is
missing. The failure that matters is the `#import` that does not find its
file, reported there, with this list to point at.

## Tests

Every test drives a fake `Host`, so the answers are the same on a Mac with
three Xcodes, a Linux box with no GNUstep, and CI. Probing is impure by
nature; the ordering logic is not, and injecting the host is what separates
them. The one thing a real machine is asked is whether the embedded headers
are there, which is a fact about the binary rather than about the host.

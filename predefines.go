package objv

import (
	"fmt"
	"strings"

	"github.com/vertex-language/objv/preprocessor"
	"github.com/vertex-language/objv/sysroot"
	"github.com/vertex-language/objv/types"
)

// The predefined macros, in three parts that belong in three places.
//
//	preprocessor  the language's: __OBJC__, __OBJC2__, __STDC__, __GNUC__
//	sysroot       the platform's: __APPLE__, __LITTLE_ENDIAN__, and the
//	              deployment target every availability macro reads
//	here          the type model's: how wide a long is, what float.h says
//
// The split follows what each part knows. Phase 4 does not import types and
// must not learn what an SDK is; sysroot probes a machine and must not learn
// what a Model is; this package has both and composes them.
//
// Without the model half the system headers do not compile: Darwin's
// <stdio.h> needs __SIZE_TYPE__ and one of these wrong is a struct of the
// wrong shape rather than an error message.

// Predefines computes the target-dependent macros the builtin headers are
// written against. The names are gcc's spellings, on purpose — Apple's
// headers already test them, and inventing a second vocabulary for the same
// facts would buy nothing.
func (t Target) Predefines() []preprocessor.Predefine {
	m := t.model
	var ds []preprocessor.Predefine
	def := func(name, value string) {
		ds = append(ds, preprocessor.Predefine{Text: name + "=" + value})
	}
	smax := func(bytes int64) uint64 { return 1<<(bytes*8-1) - 1 }
	umax := func(bytes int64) uint64 {
		if bytes >= 8 {
			return ^uint64(0)
		}
		return 1<<(bytes*8) - 1
	}

	// limits.h's inputs.
	def("__CHAR_BIT__", "8")
	if !m.CharSigned {
		def("__CHAR_UNSIGNED__", "1")
	}
	def("__SCHAR_MAX__", "127")
	def("__SHRT_MAX__", fmt.Sprint(smax(m.SizeShort)))
	def("__INT_MAX__", fmt.Sprint(smax(m.SizeInt)))
	def("__LONG_MAX__", fmt.Sprintf("%dL", smax(m.SizeLong)))
	def("__LONG_LONG_MAX__", fmt.Sprintf("%dLL", smax(m.SizeLongLong)))

	def("__SIZEOF_SHORT__", fmt.Sprint(m.SizeShort))
	def("__SIZEOF_INT__", fmt.Sprint(m.SizeInt))
	def("__SIZEOF_LONG__", fmt.Sprint(m.SizeLong))
	def("__SIZEOF_LONG_LONG__", fmt.Sprint(m.SizeLongLong))
	def("__SIZEOF_POINTER__", fmt.Sprint(m.SizePtr))
	def("__SIZEOF_FLOAT__", fmt.Sprint(m.SizeFloat))
	def("__SIZEOF_DOUBLE__", fmt.Sprint(m.SizeDouble))
	def("__SIZEOF_LONG_DOUBLE__", fmt.Sprint(m.SizeLongDouble))

	// stddef.h and stdint.h's types. size_t is the unsigned integer type of
	// pointer width: unsigned long where long is that wide (LP64), unsigned
	// long long otherwise (LLP64).
	sizeType, ptrdiffType := "unsigned long", "long"
	if m.SizeLong != m.SizePtr {
		sizeType, ptrdiffType = "unsigned long long", "long long"
	}
	def("__SIZE_TYPE__", sizeType)
	def("__SIZE_MAX__", fmt.Sprintf("%dULL", umax(m.SizePtr)))
	def("__PTRDIFF_TYPE__", ptrdiffType)
	def("__PTRDIFF_MAX__", fmt.Sprintf("%dLL", smax(m.SizePtr)))
	def("__INTPTR_TYPE__", ptrdiffType)
	def("__UINTPTR_TYPE__", sizeType)

	wtype, wmax, wmin := kindC(m.WCharKind, m)
	def("__WCHAR_TYPE__", wtype)
	def("__WCHAR_MAX__", wmax)
	def("__WCHAR_MIN__", wmin)
	def("__WINT_TYPE__", t.wint)
	if strings.HasPrefix(t.wint, "unsigned") {
		def("__WINT_MAX__", fmt.Sprintf("%dU", umax(m.SizeInt)))
		def("__WINT_MIN__", "0U")
	} else {
		def("__WINT_MAX__", fmt.Sprint(smax(m.SizeInt)))
		def("__WINT_MIN__", fmt.Sprintf("(-%d - 1)", smax(m.SizeInt)))
	}

	// intmax_t is long long on every target objv models.
	def("__INTMAX_TYPE__", "long long")
	def("__UINTMAX_TYPE__", "unsigned long long")
	def("__INTMAX_MAX__", fmt.Sprintf("%dLL", smax(8)))

	// float.h's inputs.
	def("__FLT_EVAL_METHOD__", "0")
	def("__FLT_RADIX__", "2")
	for _, d := range fltDefs {
		def(d[0], d[1])
	}
	for _, d := range dblDefs {
		def(d[0], d[1])
	}
	for _, d := range ldblDefs[t.ldbl] {
		def(d[0], d[1])
	}

	// What BOOL is, which <objc/objc.h> reads in preference to guessing
	// from TARGET_OS_*.
	//
	// It is a fact about the *architecture*. Apple's 64-bit ARM ABI made
	// BOOL a one-byte `bool`; the x86_64 Mac kept the `signed char` it
	// shipped with, and so does every other target. clang says the same
	// thing with useSignedCharForObjCBool, and getting it wrong is not a
	// warning: @encode(BOOL) becomes "c" where the runtime and every other
	// object file say "B", which is the type encoding a method list
	// publishes and an NSInvocation reads back.
	boolIsBool := "0"
	if m.ObjCBoolIsBool {
		boolIsBool = "1"
	}
	def("__OBJC_BOOL_IS_BOOL", boolIsBool)
	return ds
}

// kindC spells a basic kind as C and gives its range, for wchar_t.
func kindC(k types.Kind, m types.Model) (typ, max, min string) {
	switch k {
	case types.Int:
		n := uint64(1)<<(m.SizeInt*8-1) - 1
		return "int", fmt.Sprint(n), fmt.Sprintf("(-%d - 1)", n)
	case types.UInt:
		return "unsigned int", fmt.Sprintf("%dU", uint64(1)<<(m.SizeInt*8)-1), "0U"
	case types.UShort:
		return "unsigned short", fmt.Sprint(uint64(1)<<(m.SizeShort*8) - 1), "0"
	case types.Long:
		n := uint64(1)<<(m.SizeLong*8-1) - 1
		return "long", fmt.Sprintf("%dL", n), fmt.Sprintf("(-%dL - 1L)", n)
	}
	return "int", "2147483647", "(-2147483647 - 1)"
}

// IEEE single and double are the same on every target objv models; only long
// double varies, by ldblKind.
var fltDefs = [][2]string{
	{"__FLT_MANT_DIG__", "24"}, {"__FLT_DIG__", "6"},
	{"__FLT_MIN_EXP__", "(-125)"}, {"__FLT_MIN_10_EXP__", "(-37)"},
	{"__FLT_MAX_EXP__", "128"}, {"__FLT_MAX_10_EXP__", "38"},
	{"__FLT_MAX__", "3.40282347e+38F"},
	{"__FLT_EPSILON__", "1.19209290e-7F"},
	{"__FLT_MIN__", "1.17549435e-38F"},
	{"__FLT_DENORM_MIN__", "1.40129846e-45F"},
}

var dblDefs = [][2]string{
	{"__DBL_MANT_DIG__", "53"}, {"__DBL_DIG__", "15"},
	{"__DBL_MIN_EXP__", "(-1021)"}, {"__DBL_MIN_10_EXP__", "(-307)"},
	{"__DBL_MAX_EXP__", "1024"}, {"__DBL_MAX_10_EXP__", "308"},
	{"__DBL_MAX__", "1.7976931348623157e+308"},
	{"__DBL_EPSILON__", "2.2204460492503131e-16"},
	{"__DBL_MIN__", "2.2250738585072014e-308"},
	{"__DBL_DENORM_MIN__", "4.9406564584124654e-324"},
}

var ldblDefs = map[ldblKind][][2]string{
	ldblDouble: {
		{"__LDBL_MANT_DIG__", "53"}, {"__LDBL_DIG__", "15"},
		{"__LDBL_MIN_EXP__", "(-1021)"}, {"__LDBL_MIN_10_EXP__", "(-307)"},
		{"__LDBL_MAX_EXP__", "1024"}, {"__LDBL_MAX_10_EXP__", "308"},
		{"__LDBL_MAX__", "1.7976931348623157e+308L"},
		{"__LDBL_EPSILON__", "2.2204460492503131e-16L"},
		{"__LDBL_MIN__", "2.2250738585072014e-308L"},
		{"__LDBL_DENORM_MIN__", "4.9406564584124654e-324L"},
		{"__DECIMAL_DIG__", "17"},
	},
	ldblX87: {
		{"__LDBL_MANT_DIG__", "64"}, {"__LDBL_DIG__", "18"},
		{"__LDBL_MIN_EXP__", "(-16381)"}, {"__LDBL_MIN_10_EXP__", "(-4931)"},
		{"__LDBL_MAX_EXP__", "16384"}, {"__LDBL_MAX_10_EXP__", "4932"},
		{"__LDBL_MAX__", "1.18973149535723176502e+4932L"},
		{"__LDBL_EPSILON__", "1.08420217248550443401e-19L"},
		{"__LDBL_MIN__", "3.36210314311209350626e-4932L"},
		{"__LDBL_DENORM_MIN__", "3.64519953188247460253e-4951L"},
		{"__DECIMAL_DIG__", "21"},
	},
	ldblQuad: {
		{"__LDBL_MANT_DIG__", "113"}, {"__LDBL_DIG__", "33"},
		{"__LDBL_MIN_EXP__", "(-16381)"}, {"__LDBL_MIN_10_EXP__", "(-4931)"},
		{"__LDBL_MAX_EXP__", "16384"}, {"__LDBL_MAX_10_EXP__", "4932"},
		{"__LDBL_MAX__", "1.189731495357231765085759326628007e+4932L"},
		{"__LDBL_EPSILON__", "1.925929944387235853055977942584927e-34L"},
		{"__LDBL_MIN__", "3.362103143112093506262677817321753e-4932L"},
		{"__LDBL_DENORM_MIN__", "6.475175119438025110924438958227646e-4966L"},
		{"__DECIMAL_DIG__", "36"},
	},
}

// ---- the four interrogation operators ----
//
// __has_feature and its three neighbours are not phase 4's to answer: what
// this compiler implements is decided where it is implemented, and a copy of
// that list inside the preprocessor would be a second list that could
// disagree with the first.
//
// The answers matter more than they look. A Cocoa header asks these before
// it decides what to declare, and a wrong answer is not a diagnostic — it is
// a header taking a fallback path, silently, and producing a program that
// means something else. Two of them decide whether Foundation compiles at
// all; see the note above hasExtension.

// HasFeature, HasExtension, HasBuiltin and HasAttribute are objv's answers to
// clang's four interrogation operators, exported because they are part of
// what the compiler says about itself: `objv env` prints them, and a tool
// driving the front end has to hand the same answers to phase 4 that objv
// would.
// HasFeature answers __has_feature. arc says whether this compilation has
// automatic reference counting on, which three of the answers depend on: ARC
// is a mode and not a capability, and <objc/objc.h> reads it to decide
// whether -retain and -release are declared unavailable.
func HasFeature(name string, arc bool) bool   { return hasFeature(arc)(name) }
func HasExtension(name string, arc bool) bool { return extensionOn(name, arc) }
func HasBuiltin(name string) bool             { return hasBuiltin(name) }
func HasAttribute(name string) bool           { return hasAttribute(name) }

// hasFeature answers __has_feature.
//
// Every entry is a language feature objv implements, and the list is short
// on purpose: a feature claimed and not implemented is worse than one denied,
// because a header is entitled to be told no and take its fallback.
// HasFeatureFunc is HasFeature bound to one compilation's ARC mode, which is
// the shape preprocessor.Config wants.
func HasFeatureFunc(arc bool) func(string) bool { return hasFeature(arc) }

func hasFeature(arc bool) func(string) bool {
	return func(name string) bool { return featureOn(name, arc) }
}

func featureOn(name string, arc bool) bool {
	switch name {
	// ARC is a mode, not a capability. A header is entitled to ask whether
	// it is on and to declare different things either way: <objc/objc.h>
	// makes -retain and -release unavailable under it, and answering yes
	// when it is off takes away the two methods manual reference counting
	// is written in.
	case "objc_arc", "objc_arc_weak", "objc_arc_fields":
		return arc

	// The Objective-C surface, §4 through §7.
	case "objc_instancetype", "objc_generics", "objc_generics_variance",
		"objc_kindof", "objc_class_property", "objc_subscripting",
		"objc_array_literals", "objc_dictionary_literals",
		"objc_boxed_expressions", "objc_boxed_nsvalue_expressions",
		"objc_bool", "objc_fixed_enum", "objc_default_synthesize_properties",
		"objc_property_explicit_atomic", "objc_protocol_qualifier_mangling",
		"objc_modules", "nullability", "nullability_on_arrays",
		"objc_bridge_id", "objc_bridge_id_on_typedefs",
		"blocks", "enumerator_attributes",
		"attribute_availability", "attribute_availability_with_message",
		// The version-underscore spelling — availability(macosx,introduced=10_2)
		// — is the branch Apple's headers maintain, and denying it sends
		// CoreFoundation down a chain of three token pastes into
		// AvailabilityInternalLegacy.h that does not define every name it
		// can produce. objv ignores availability attributes, so accepting
		// the spelling costs nothing and takes the path that works.
		"attribute_availability_with_version_underscores",
		"attribute_availability_app_extension",
		"attribute_availability_swift",
		"attribute_deprecated_with_message",
		"attribute_unavailable_with_message",
		"attribute_overloadable", "attribute_ns_returns_retained",
		"attribute_cf_returns_retained":
		return true

	// The C11 surface the substrate implements.
	case "c_static_assert", "c_generic_selections", "c_alignas", "c_alignof",
		"c_atomic", "c_thread_local":
		return true
	}
	return false
}

// hasExtension answers __has_extension, which is true wherever __has_feature
// is and true besides where a feature is available as an extension.
//
// define_target_os_macros is the one that must answer *no*, and it is the
// one worth reading twice. It means "the compiler predefines the TARGET_OS_*
// macros itself", which clang does and objv does not. Answering yes makes
// TargetConditionals.h skip the block that would have defined them, and the
// file then falls through to `#define TARGET_OS_MAC 0` — no error, no
// warning, and every `#if TARGET_OS_OSX` in Foundation takes the wrong
// branch. It is on this list explicitly so that nobody adds it.
func hasExtension(name string) bool { return extensionOn(name, false) }

// HasExtensionFunc is hasExtension bound to one compilation's ARC mode.
// §6.10's __has_extension answers yes to everything __has_feature does, so
// it carries the mode for the same three names.
func HasExtensionFunc(arc bool) func(string) bool {
	return func(name string) bool { return extensionOn(name, arc) }
}

func extensionOn(name string, arc bool) bool {
	switch name {
	case "define_target_os_macros":
		return false
	}
	return featureOn(name, arc)
}

// hasBuiltin answers __has_builtin.
//
// The four target predicates must answer yes, and are the other half of the
// TargetConditionals.h question: with define_target_os_macros denied, the
// header decides TARGET_OS_OSX by asking __is_target_os(macos) — but only
// after asking whether the four operators exist at all. Deny them and the
// same silent fallthrough happens.
func hasBuiltin(name string) bool {
	switch name {
	case "__is_target_arch", "__is_target_vendor",
		"__is_target_os", "__is_target_environment":
		return true
	case "__builtin_va_start", "__builtin_va_arg", "__builtin_va_end",
		"__builtin_va_copy", "__builtin_offsetof", "__builtin_expect",
		"__builtin_unreachable", "__builtin_trap":
		return true
	}
	return false
}

// hasAttribute answers __has_attribute.
//
// Yes to everything, which is not laziness. An attribute objv does not
// implement is one it ignores, and the alternative a header takes when told
// no is usually a *different declaration* — a macro expanding to nothing
// rather than to an attribute is fine, but one expanding to a second
// spelling of the same API is not. Saying yes and ignoring what arrives is
// what clang does for the attributes it does not know, with a warning it can
// afford and objv cannot yet.
func hasAttribute(name string) bool { return name != "" }

// config assembles the preprocessor configuration for a resolved sysroot.
func (t Target) ppConfig(r sysroot.Result) preprocessor.Config {
	cfg := preprocessor.Config{
		Triple:    t.Triple(),
		Feature:   hasFeature(false),
		Extension: hasExtension,
		Builtin:   hasBuiltin,
		Attribute: hasAttribute,
	}
	for _, e := range r.Include {
		cfg.Search = append(cfg.Search, preprocessor.Mount{
			Name: e.Name, FS: e.FS, System: e.System})
	}
	for _, e := range r.Frameworks {
		cfg.Frameworks = append(cfg.Frameworks, preprocessor.Mount{
			Name: e.Name, FS: e.FS, System: e.System})
	}
	return cfg
}

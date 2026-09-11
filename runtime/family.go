package runtime

// What a selector's *name* promises about the object it returns.
//
// ARC has no annotations on the framework methods it calls. What it has is a
// naming convention, and the convention is normative: a method whose selector
// begins with alloc, copy, init, mutableCopy or new returns an object the
// caller owns, and every other method returns one it does not. That is why
// `[[NSString alloc] init]` needs no release in the caller's source and
// `[NSString stringWithFormat:…]` does — and it is why a method called
// `newlineCharacterSet` is not in the family, because the word is "newline"
// and not "new".
//
// The rule is exact, from clang's ARC specification: skip leading
// underscores, take the longest of the five names that the selector starts
// with, and it is a family only if what follows is not a lowercase letter.

// Family is the ownership convention a selector's name implies.
type Family uint8

const (
	FamilyNone Family = iota
	FamilyAlloc
	FamilyCopy
	FamilyInit
	FamilyMutableCopy
	FamilyNew
)

// familyNames is checked longest-first, because "mutableCopy" begins with no
// other name but "new" and "newX" differ only in what follows.
var familyNames = []struct {
	name string
	fam  Family
}{
	{"mutableCopy", FamilyMutableCopy},
	{"alloc", FamilyAlloc},
	{"copy", FamilyCopy},
	{"init", FamilyInit},
	{"new", FamilyNew},
}

// FamilyOf is the family a selector belongs to.
func FamilyOf(sel string) Family {
	i := 0
	for i < len(sel) && sel[i] == '_' {
		i++
	}
	rest := sel[i:]
	for _, f := range familyNames {
		if len(rest) < len(f.name) || rest[:len(f.name)] != f.name {
			continue
		}
		// "newline" is not in the new family: the convention reads words,
		// and a lowercase letter continues the one before it.
		if len(rest) > len(f.name) && rest[len(f.name)] >= 'a' && rest[len(f.name)] <= 'z' {
			continue
		}
		return f.fam
	}
	return FamilyNone
}

// ReturnsRetained reports whether a method of this family hands the caller an
// object it owns. Four of the five do; init is the fifth and also does — it
// consumes the receiver and returns the same +1 it was given.
func (f Family) ReturnsRetained() bool { return f != FamilyNone }

// ConsumesSelf reports whether the method takes ownership of its receiver,
// which only init does: `[[X alloc] init]` hands the +1 from alloc to init,
// and what comes back is that same one.
func (f Family) ConsumesSelf() bool { return f == FamilyInit }

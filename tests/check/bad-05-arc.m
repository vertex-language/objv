// What ARC refuses. This file is checked with ARC on.

@interface Managed : NSObject
@property (nonatomic, assign) NSObject *assigned;   // expect: does not keep it alive
@end

@implementation Managed
- (void)mistakes {
    [self retain];                  // expect: ARC forbids sending 'retain'
    [self release];                 // expect: ARC forbids sending 'release'
    [self autorelease];             // expect: ARC forbids sending 'autorelease'
    [self dealloc];                 // expect: ARC forbids sending 'dealloc'

    void *raw = (void *)self;       // expect: requires a bridge cast under ARC
    NSObject *back = (NSObject *)raw;  // expect: requires a bridge cast under ARC

    NSObject *fine = (__bridge NSObject *)raw;
    void *alsoFine = (__bridge void *)self;
    (void)fine; (void)alsoFine;
}
@end

// An object pointer in a struct is allowed: early ARC forbade it, and clang
// has admitted it since objc_arc_fields, which objv claims. What the field
// costs is a struct whose copies and destruction the compiler has to manage,
// which lower does not do yet and which lower/README.md says.
struct Holder {
    NSObject *object;
};

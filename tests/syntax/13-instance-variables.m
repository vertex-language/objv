// §4.5 Instance Variables

@protocol NSObject;

@interface Ivars
{
    // Before any VisibilitySpecification: @protected in a ClassInterface
    int _defaultVisibility;

@private
    int _private;
    id _privateObject;

@protected
    int _protected;

@public
    int _public;
    struct { int x, y; } _nested;

@package
    int _package;

@private
    // A visibility section may hold anything a StructDeclaration may
    unsigned _bitfield : 3;
    int _a, *_b, _c[4];
}
- (void)method;
@end

@implementation Ivars
{
    // In a ClassImplementation the default is @private
    int _implementationIvar;
@public
    int _publicImplementationIvar;
}
- (void)method { }
@end

@interface Empty { } @end

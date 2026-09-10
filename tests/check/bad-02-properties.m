// Properties, their attributes, and dot syntax.

@interface Props : NSObject
@property (nonatomic, copy, retain) NSString *twoOwners;   // expect: mutually exclusive
@property (readonly, readwrite) int bothWays;              // expect: readonly and readwrite
@property (atomic, nonatomic) int bothAtomic;              // expect: atomic and nonatomic
@property (nonatomic, copy) NSString *name;
@property (nonatomic, copy) NSString *name;                // expect: duplicate property 'name'
@end

@implementation Props
@synthesize missing;                                       // expect: no property named 'missing'

- (void)use {
    self.nope = 1;                                         // expect: no property or getter named 'nope'
    int n = self.name;                                     // expect: initializing
    (void)n;
}
@end

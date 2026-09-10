// What an implementation promised and did not provide.

@protocol Required
- (void)mustImplement;
@optional
- (void)mayImplement;
@end

@interface Incomplete : NSObject <Required>
- (void)declaredButMissing;
- (void)provided;
@end

@implementation Incomplete         // expect: '-mustImplement' required by protocol // expect: '-declaredButMissing' declared by 'Incomplete' is not implemented
- (void)provided { }
- (void)provided { }               // expect: duplicate definition of method
@end

@implementation NoInterface        // expect: cannot find interface declaration
- (void)whatever { }
@end

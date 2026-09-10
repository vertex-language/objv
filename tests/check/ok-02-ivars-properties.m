// Instance variables, their visibility, and properties.

@interface Holder : NSObject {
@protected
    int _protectedCount;
@private
    NSString *_secret;
@public
    int open;
}
@property (nonatomic, copy) NSString *name;
@property (nonatomic, readonly) int count;
@property (nonatomic, weak) id delegate;
@property (nonatomic, getter=isReady) int ready;
@end

@implementation Holder
@synthesize name = _name;
@dynamic delegate;

- (void)use {
    _protectedCount = 1;
    _secret = @"hidden";        // private, but this is the class
    _name = @"n";               // the synthesized backing variable
    self.name = @"other";
    NSString *s = self.name;
    int r = self.isReady;       // the getter the attribute named
    (void)s; (void)r;
}
@end

@interface Sub : Holder
@end

@implementation Sub
- (void)use {
    _protectedCount = 2;        // @protected reaches a subclass
    self.name = @"x";
    open = 1;
}
@end

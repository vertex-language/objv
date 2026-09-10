// §4.2 Categories and Class Extensions

@interface Base
@end

@protocol Drawable;

// CategoryInterface
@interface Base (Persistence)
- (void)save;
@end

@interface Base (Drawing) <Drawable>
- (void)draw;
@end

// ClassExtension: a category with no name
@interface Base ()
- (void)privateHelper;
@end

// An extension may declare instance variables, which a category may not
@interface Base () {
    int _extensionIvar;
    id _delegate;
}
@property (nonatomic) int hidden;
@end

// CategoryImplementation
@implementation Base (Persistence)
- (void)save { }
@end

@implementation Base (Drawing)
- (void)draw { }
@end

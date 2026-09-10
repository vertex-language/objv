// §5.5's lightweight generics and §4.3's protocols.

@protocol Feeder <NSObject>
@required
- (NSString *)nameForIndex:(int)i;
@optional
- (void)reset;
@end

@interface Feed : NSObject <Feeder>
@end

@implementation Feed
- (NSString *)nameForIndex:(int)i { (void)i; return @"x"; }
@end

@interface Box<__covariant T : NSObject *> : NSObject
- (T)value;
- (void)setValue:(T)value;
@end

@implementation Box
- (id)value { return (id)0; }
- (void)setValue:(id)value { (void)value; }
@end

@interface Use : NSObject
@end

@implementation Use
- (void)generics {
    NSArray<NSString *> *strings;
    NSArray<NSString *> *same = strings;
    NSArray *erased = strings;          // unspecialized: the runtime erases
    NSArray<id<NSCopying>> *nested;

    Box<NSMutableString *> *mutable_;
    Box<NSString *> *covariant = mutable_;   // __covariant admits it

    id<Feeder> feeder = [[Feed alloc] init];
    NSString *n = [feeder nameForIndex:0];

    id<NSCopying> copying = @"a";
    (void)same; (void)erased; (void)nested; (void)covariant; (void)n; (void)copying;
}
@end

// The combinations, as the Cocoa headers and ordinary code write them.
// Nothing new here: every construct appears in an earlier file. What is new
// is that they meet.

typedef long NSInteger;
typedef unsigned long NSUInteger;
typedef signed char BOOL;

@class NSString, NSArray, NSError, NSCoder;
@protocol NSCopying, NSCoding;

@interface NSObject <NSObject>
+ (instancetype)alloc;
- (instancetype)init;
@end

// NS_ENUM and NS_OPTIONS, expanded
enum NSComparisonResult : NSInteger NSComparisonResult; enum NSComparisonResult : NSInteger {
    NSOrderedAscending = -1L,
    NSOrderedSame,
    NSOrderedDescending,
};

enum NSStringDrawingOptions : NSUInteger NSStringDrawingOptions; enum NSStringDrawingOptions : NSUInteger {
    NSStringDrawingUsesLineFragmentOrigin = 1 << 0,
    NSStringDrawingUsesFontLeading        = 1 << 1,
};

// A class as a framework header declares one: attributes before the
// interface, generics, protocols, designated initializers, nullability
__attribute__((availability(macosx, introduced=10.9)))
@interface Cache<__covariant KeyType : id<NSCopying>, ObjectType> : NSObject <NSCopying, NSCoding>
{
@private
    NSUInteger _count;
    __weak id _delegate;
}

@property (class, nonatomic, readonly) Cache *sharedCache;
@property (nonatomic, readonly) NSUInteger count;
@property (nonatomic, copy, nullable) NSString *name;
@property (nonatomic, copy) NSArray<KeyType> *allKeys;
@property (nonatomic, copy) void (^evictionHandler)(KeyType key, ObjectType object);

- (instancetype)initWithCapacity:(NSUInteger)capacity
    __attribute__((objc_designated_initializer));
- (instancetype)init __attribute__((unavailable("use -initWithCapacity:")));

- (nullable ObjectType)objectForKey:(KeyType)key;
- (nullable ObjectType)objectForKeyedSubscript:(KeyType)key;
- (void)setObject:(ObjectType)object forKeyedSubscript:(KeyType)key;
- (void)setObject:(ObjectType)object forKey:(KeyType)key;
- (void)enumerate:(void (^)(KeyType key, ObjectType object, BOOL *stop))block;
+ (nullable instancetype)cacheWithCapacity:(NSUInteger)capacity
                                     error:(NSError *_Nullable *_Nullable)error;
@end

@interface Cache (Persistence) <NSCoding>
- (BOOL)writeToFile:(NSString *)path error:(NSError **)error;
@end

@interface Cache ()
@property (nonatomic, readwrite) NSUInteger count;
- (void)evictAll;
@end

@implementation Cache {
    NSUInteger _capacity;
}

@synthesize name = _name;
@dynamic sharedCache;

- (instancetype)initWithCapacity:(NSUInteger)capacity {
    self = [super init];
    if (self) {
        _capacity = capacity;
        _count = 0;
    }
    return self;
}

- (nullable id)objectForKey:(id<NSCopying>)key {
    (void)key;
    return nil;
}

- (nullable id)objectForKeyedSubscript:(id<NSCopying>)key {
    return [self objectForKey:key];
}

- (void)setObject:(id)object forKeyedSubscript:(id<NSCopying>)key {
    [self setObject:object forKey:key];
}

- (void)setObject:(id)object forKey:(id<NSCopying>)key {
    (void)object; (void)key;
    self.count += 1;
}

- (void)enumerate:(void (^)(id, id, BOOL *))block {
    BOOL stop = __objc_no;
    for (id key in self.allKeys) {
        block(key, [self objectForKey:key], &stop);
        if (stop) break;
    }
}

- (void)evictAll {
    __weak __typeof__(self) weakSelf = self;
    void (^handler)(void) = ^{
        __strong __typeof__(weakSelf) strongSelf = weakSelf;
        if (!strongSelf) return;
        @synchronized (strongSelf) {
            @try {
                for (id key in strongSelf.allKeys) {
                    (void)key;
                }
            } @catch (id e) {
                (void)e;
            } @finally {
                strongSelf.count = 0;
            }
        }
    };
    handler();
}

+ (nullable instancetype)cacheWithCapacity:(NSUInteger)capacity error:(NSError **)error {
    (void)error;
    return [[self alloc] initWithCapacity:capacity];
}
@end

int main(void) {
    @autoreleasepool {
        Cache<NSString *, NSArray<NSString *> *> *cache =
            [[Cache alloc] initWithCapacity:16];
        cache[@"key"] = @[@"value"];
        NSArray<NSString *> *values = cache[@"key"];
        // The middle operand of ?: is optional -- GCC's extension, which
        // every compiler that builds Cocoa accepts and which Objective-C
        // uses for exactly this: a default where a property is nil, with
        // the property read once.
        id info = @{
            @"name": cache.name ?: @"unnamed",
            @"count": @(cache.count),
            @"selector": @"objectForKey:",
            @"values": values ?: @[],
        };
        if (@available(macOS 10.12, *)) {
            (void)info;
        }
        return (int)cache.count;
    }
}

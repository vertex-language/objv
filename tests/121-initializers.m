// A designated initializer chain, instancetype, and init returning a
// different object.
// mode: both
#import <Foundation/Foundation.h>
#import <objc/runtime.h>
#include <stdio.h>

@interface Person : NSObject
@property (nonatomic) int age;
@property (nonatomic, copy) NSString *name;
- (instancetype)initWithName:(NSString *)name age:(int)age;
- (instancetype)initWithName:(NSString *)name;
+ (instancetype)personNamed:(NSString *)name;
@end

@implementation Person
- (instancetype)initWithName:(NSString *)name age:(int)age {
    if ((self = [super init])) {
        _name = [name copy];
        _age = age;
    }
    return self;
}
- (instancetype)initWithName:(NSString *)name { return [self initWithName:name age:-1]; }
+ (instancetype)personNamed:(NSString *)name { return [[self alloc] initWithName:name]; }
@end

@interface Student : Person
@end
@implementation Student
@end

int main(void) {
    @autoreleasepool {
        Person *a = [[Person alloc] initWithName:@"ada" age:36];
        Student *b = [Student personNamed:@"bo"];
        printf("%s %d / %s %d %s\n", a.name.UTF8String, a.age, b.name.UTF8String, b.age,
               class_getName([b class]));
    }
    return 0;
}

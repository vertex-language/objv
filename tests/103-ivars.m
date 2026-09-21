// Instance variables of several types, read and written by methods.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Account : NSObject {
    int _id;
    double _balance;
    char _kind;
    long long _history[3];
}
- (void)setUp;
- (void)deposit:(double)v;
- (void)print;
@end

@implementation Account
- (void)setUp { _id = 7; _kind = 'S'; _history[2] = 1ll << 40; }
- (void)deposit:(double)v { _balance += v; _history[0]++; }
- (void)print { printf("%d %c %.2f %lld %lld\n", _id, _kind, _balance, _history[0], _history[2]); }
@end

int main(void) {
    Account *a = [Account new];
    [a print];
    [a setUp];
    [a deposit:10.5];
    [a deposit:0.25];
    [a print];
    return 0;
}

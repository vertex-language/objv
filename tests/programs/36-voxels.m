// A first-person voxel world: an @implementation with a C helper in it, and
// a class whose state is arrays rather than objects.
//
// The construct this file was written for is one line of the program:
//
//	@implementation World
//	static int elevationAt(int x, int z) { … }
//	- (void)generate { … elevationAt(wx, wz) … }
//	@end
//
// A C function written between two methods is at file scope like any other
// (§4.4), and keeping it static is how a class keeps a helper to itself.
// The use set collected the conditionally-emitted bodies from the *file's*
// top level only, so a helper inside a class body was in no set anything
// could mark, the definition was dropped as unused, and the method that
// called it reached lowering with the name undeclared.
//
// Around it: a two-dimensional int instance variable and an array of object
// pointers beside it, which is how a grid-shaped program holds a grid; and a
// tick function whose arithmetic is all CGFloat, run enough times that a
// rounding difference would show.
//
// frameworks: Foundation

#import <Foundation/Foundation.h>
#include <math.h>

#define WORLD_HALF 18
#define WORLD_SIZE (WORLD_HALF * 2)

typedef NS_ENUM(NSInteger, BlockType) {
    BlockTypeGrass = 0,
    BlockTypeStone,
    BlockTypeWood,
    BlockTypeGold,
    BlockTypeSnow,
    BlockTypeCount
};

@interface World : NSObject {
    int _heightMap[WORLD_SIZE][WORLD_SIZE];
    NSString *_names[BlockTypeCount];
    CGFloat _worldTime;
}
@property (nonatomic, assign) BlockType selected;
@property (nonatomic, assign) CGFloat playerX, playerY, playerZ;
@property (nonatomic, assign) CGFloat yaw, pitch;
@property (nonatomic, assign) CGFloat yVelocity;
@property (nonatomic, assign) BOOL grounded, jumping, walking;
- (void)generate;
- (void)tick;
- (int)heightAtX:(int)x z:(int)z;
@end

@implementation World

// The helper this file exists for: static, defined between two methods, and
// called from one of them.
static int computeElevation(int x, int z) {
    float h1 = sinf(x * 0.18f) * cosf(z * 0.18f) * 3.5f;
    float h2 = sinf(x * 0.08f + 1.2f) * sinf(z * 0.08f + 0.8f) * 4.0f;
    float raw = 3.0f + h1 + h2;
    if (raw < 0.0f) raw = 0.0f;
    if (raw > 9.0f) raw = 9.0f;
    return (int)roundf(raw);
}

// A second one, reached only from the first, which is what the closure over
// the use set is for.
static BlockType surfaceOf(int h) {
    if (h >= 7) return BlockTypeSnow;
    if (h >= 4) return BlockTypeStone;
    return BlockTypeGrass;
}

// And one nothing calls, which must still not be emitted.
static int unusedHelper(int x) { return x * 999; }

static NSInteger sGenerations = 0;

- (instancetype)init {
    if ((self = [super init])) {
        _names[BlockTypeGrass] = @"Grass";
        _names[BlockTypeStone] = @"Stone";
        _names[BlockTypeWood]  = @"Wood";
        _names[BlockTypeGold]  = @"Gold";
        _names[BlockTypeSnow]  = @"Snow";
        _playerY = 6.0;
    }
    return self;
}

- (void)generate {
    sGenerations++;
    for (int gx = 0; gx < WORLD_SIZE; gx++) {
        for (int gz = 0; gz < WORLD_SIZE; gz++) {
            _heightMap[gx][gz] = computeElevation(gx - WORLD_HALF, gz - WORLD_HALF);
        }
    }
}

- (int)heightAtX:(int)x z:(int)z {
    int gx = x + WORLD_HALF, gz = z + WORLD_HALF;
    if (gx < 0 || gx >= WORLD_SIZE || gz < 0 || gz >= WORLD_SIZE) return 0;
    return _heightMap[gx][gz];
}

- (NSString *)describeSurfaceAtX:(int)x z:(int)z {
    int h = [self heightAtX:x z:z];
    return [NSString stringWithFormat:@"%@@%d", _names[surfaceOf(h)], h];
}

- (void)tick {
    CGFloat dt = 1.0 / 60.0;
    _worldTime += dt * 0.05;

    CGFloat moveSpeed = 7.0 * dt;
    CGFloat fwdX = -sin(self.yaw), fwdZ = -cos(self.yaw);

    if (self.walking) {
        self.playerX += fwdX * moveSpeed;
        self.playerZ += fwdZ * moveSpeed;
    }
    if (self.playerX < -WORLD_HALF + 1) self.playerX = -WORLD_HALF + 1;
    if (self.playerX >  WORLD_HALF - 1) self.playerX =  WORLD_HALF - 1;
    if (self.playerZ < -WORLD_HALF + 1) self.playerZ = -WORLD_HALF + 1;
    if (self.playerZ >  WORLD_HALF - 1) self.playerZ =  WORLD_HALF - 1;

    int gx = (int)roundf(self.playerX), gz = (int)roundf(self.playerZ);
    CGFloat groundY = [self heightAtX:gx z:gz] + 1.65;

    if (self.jumping && self.grounded) {
        self.yVelocity = 6.8;
        self.grounded = NO;
        self.jumping = NO;
    }
    if (!self.grounded) {
        self.yVelocity -= 19.0 * dt;
        self.playerY += self.yVelocity * dt;
        if (self.playerY <= groundY) {
            self.playerY = groundY;
            self.yVelocity = 0.0;
            self.grounded = YES;
        }
    } else {
        self.playerY += (groundY - self.playerY) * 0.35;
    }
}

- (NSString *)timeOfDay {
    CGFloat daylight = sinf(_worldTime);
    CGFloat clamped = (daylight + 0.3) / 1.3;
    if (clamped < 0.0) clamped = 0.0;
    if (clamped > 1.0) clamped = 1.0;
    if (clamped > 0.4) return @"Day";
    if (clamped > 0.15) return @"Sunset";
    return @"Night";
}

+ (NSInteger)generations { return sGenerations; }

@end

int main(void) {
    @autoreleasepool {
        World *w = [[World alloc] init];
        [w generate];

        long sum = 0;
        int mn = 999, mx = -999;
        for (int a = -WORLD_HALF; a < WORLD_HALF; a++) {
            for (int b = -WORLD_HALF; b < WORLD_HALF; b++) {
                int h = [w heightAtX:a z:b];
                sum += h;
                if (h < mn) mn = h;
                if (h > mx) mx = h;
            }
        }
        printf("heights sum %ld min %d max %d gens %ld\n",
               sum, mn, mx, (long)[World generations]);
        printf("corners %d %d %d %d\n",
               [w heightAtX:-WORLD_HALF z:-WORLD_HALF],
               [w heightAtX:-WORLD_HALF z:WORLD_HALF - 1],
               [w heightAtX:WORLD_HALF - 1 z:-WORLD_HALF],
               [w heightAtX:WORLD_HALF - 1 z:WORLD_HALF - 1]);
        for (int i = -4; i <= 4; i += 2) printf("%d ", computeElevation(i, i));
        printf("\n");
        for (int i = -6; i <= 6; i += 3) {
            printf("%s ", [w describeSurfaceAtX:i z:i].UTF8String);
        }
        printf("\n");

        for (int i = 0; i < 120; i++) [w tick];
        printf("settled %.4f grounded %d %s\n", w.playerY, (int)w.grounded,
               [w timeOfDay].UTF8String);

        w.jumping = YES;
        for (int i = 0; i < 20; i++) [w tick];
        printf("jump %.4f vel %.4f grounded %d\n", w.playerY, w.yVelocity, (int)w.grounded);

        w.walking = YES;
        w.yaw = 0.0;
        for (int i = 0; i < 60; i++) [w tick];
        w.walking = NO;
        printf("walked %.4f %.4f %.4f\n", w.playerX, w.playerY, w.playerZ);

        w.yaw = 1.2;
        w.walking = YES;
        for (int i = 0; i < 400; i++) [w tick];
        printf("far %.4f %.4f %.4f %s\n", w.playerX, w.playerY, w.playerZ,
               [w timeOfDay].UTF8String);

        for (BlockType t = BlockTypeGrass; t < BlockTypeCount; t++) {
            w.selected = t;
            printf("%ld ", (long)w.selected);
        }
        printf("\n");
    }
    return 0;
}

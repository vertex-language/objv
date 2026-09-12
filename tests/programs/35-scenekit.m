// SceneKit, which is the first framework whose headers a compiler cannot
// read by knowing Objective-C.
//
// <SceneKit/SceneKitTypes.h> imports Metal and simd. simd is the platform's
// vector library, and it is built out of two clang extensions that are not in
// C: __attribute__((ext_vector_type(N))) and __attribute__((overloadable)).
// <simd/base.h> asks for both by name and defines nothing at all without
// them, and SceneKit's own SIMD Bridge is written outside that guard — so a
// compiler that lacks either cannot read the header, and a program that
// cannot read the header cannot say what a cube looks like.
//
// Getting there also needed: #import meaning "at most once" for a file an
// #include had already read (Metal's MTL4BufferRange.h has no guard of any
// kind and is reached both ways); #pragma mark consumed rather than passed
// through (SceneKit writes one with a typographic apostrophe in it); the
// complex types declarable though not implemented (<simd/math.h> reaches
// <tgmath.h> reaches <complex.h>); the C23 width-named float suffixes
// (0x1.ffcp-1f16); and <arm_neon.h>, which is the compiler's to supply and
// which <simd/logic.h> calls into directly on this architecture.
//
// Nothing here shows a window. The scene graph is built, inspected and
// dropped, which is what the framework is being asked about.
//
// frameworks: SceneKit Cocoa

#import <Cocoa/Cocoa.h>
#import <SceneKit/SceneKit.h>

static SCNNode *makeCube(SCNScene *scene) {
    SCNBox *box = [SCNBox boxWithWidth:2.2 height:2.2 length:2.2 chamferRadius:0.14];

    NSArray<NSColor *> *faces = @[
        [NSColor colorWithSRGBRed:0.95 green:0.26 blue:0.21 alpha:1.0],
        [NSColor colorWithSRGBRed:0.13 green:0.59 blue:0.95 alpha:1.0],
        [NSColor colorWithSRGBRed:0.30 green:0.69 blue:0.31 alpha:1.0],
        [NSColor colorWithSRGBRed:1.00 green:0.76 blue:0.03 alpha:1.0],
        [NSColor colorWithSRGBRed:0.61 green:0.15 blue:0.69 alpha:1.0],
        [NSColor colorWithSRGBRed:0.00 green:0.74 blue:0.83 alpha:1.0],
    ];
    NSMutableArray<SCNMaterial *> *materials = [NSMutableArray arrayWithCapacity:6];
    for (NSColor *c in faces) {
        SCNMaterial *m = [SCNMaterial material];
        m.diffuse.contents = c;
        m.specular.contents = [NSColor colorWithWhite:0.8 alpha:1.0];
        m.roughness.contents = @0.25;
        m.lightingModelName = SCNLightingModelBlinn;
        [materials addObject:m];
    }
    box.materials = materials;

    SCNNode *cube = [SCNNode nodeWithGeometry:box];
    [scene.rootNode addChildNode:cube];
    [cube runAction:[SCNAction repeatActionForever:
                     [SCNAction rotateByX:1.2 y:2.4 z:0.8 duration:6.0]]];
    return cube;
}

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        // SceneKit needs an application to exist; it does not need one to
        // run, and nothing here orders a window on screen.
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyProhibited];
        printf("nsapp %d\n", (int)(NSApp == app));

        SCNScene *scene = [SCNScene scene];

        SCNNode *camera = [SCNNode node];
        camera.camera = [SCNCamera camera];
        camera.position = SCNVector3Make(0, 0, 6);
        [scene.rootNode addChildNode:camera];

        SCNNode *omni = [SCNNode node];
        omni.light = [SCNLight light];
        omni.light.type = SCNLightTypeOmni;
        omni.position = SCNVector3Make(5, 7, 6);
        [scene.rootNode addChildNode:omni];

        SCNNode *ambient = [SCNNode node];
        ambient.light = [SCNLight light];
        ambient.light.type = SCNLightTypeAmbient;
        ambient.light.color = [NSColor colorWithWhite:0.22 alpha:1.0];
        [scene.rootNode addChildNode:ambient];

        SCNNode *cube = makeCube(scene);
        SCNBox *box = (SCNBox *)cube.geometry;

        printf("children %lu\n", (unsigned long)scene.rootNode.childNodes.count);
        printf("camera %g %g %g cam %d\n", camera.position.x, camera.position.y,
               camera.position.z, (int)(camera.camera != nil));
        printf("box %g %g %g chamfer %g materials %lu\n",
               box.width, box.height, box.length, box.chamferRadius,
               (unsigned long)box.materials.count);

        CGFloat r, g, b, a;
        NSInteger i = 0;
        for (SCNMaterial *m in box.materials) {
            NSColor *c = [(NSColor *)m.diffuse.contents
                          colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
            [c getRed:&r green:&g blue:&b alpha:&a];
            printf("mat %ld %.2f %.2f %.2f %s %s\n", (long)i++, r, g, b,
                   m.lightingModelName.UTF8String,
                   [m.roughness.contents description].UTF8String);
        }
        for (SCNNode *n in scene.rootNode.childNodes) {
            if (n.light) {
                printf("light %s at %g %g %g\n", n.light.type.UTF8String,
                       n.position.x, n.position.y, n.position.z);
            }
        }
        printf("actions %lu running %d\n",
               (unsigned long)cube.actionKeys.count, (int)cube.hasActions);

        // The struct geometry, which is plain C and travels by value through
        // every one of these calls.
        SCNMatrix4 m = SCNMatrix4MakeRotation((CGFloat)M_PI_2, 0, 1, 0);
        SCNMatrix4 t = SCNMatrix4MakeTranslation(1, 2, 3);
        SCNMatrix4 both = SCNMatrix4Mult(m, t);
        printf("matrix %.3f %.3f %.3f %.3f\n", both.m41, both.m42, both.m43, both.m44);
        printf("identity %d\n", (int)SCNMatrix4IsIdentity(SCNMatrix4Identity));
        SCNVector3 v = SCNVector3Make(1, 2, 3);
        SCNVector4 v4 = SCNVector4Make(1, 2, 3, 4);
        printf("vec %g %g %g | %g %g %g %g\n", v.x, v.y, v.z, v4.x, v4.y, v4.z, v4.w);
        printf("sizes %zu %zu %zu\n",
               sizeof(SCNVector3), sizeof(SCNVector4), sizeof(SCNMatrix4));

        // And the simd types themselves, at the level a program reads them:
        // the layout the headers are written against.
        printf("simd %zu %zu %zu %zu\n", sizeof(simd_float3), sizeof(simd_float4),
               sizeof(simd_float4x4), sizeof(simd_quatf));
        printf("simd %zu %zu %zu\n", _Alignof(simd_float3), _Alignof(simd_float4x4),
               sizeof(simd_double3));
    }
    return 0;
}

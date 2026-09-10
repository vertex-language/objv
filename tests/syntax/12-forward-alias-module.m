// §4.4 Forward Declarations, Aliases, and Modules

@class Single;
@class First, Second, Third;

// A generic class may state its parameters in a forward declaration
@class Container<T>;
@class Pair<K, V>, Triple<A, B, C>;

@protocol Proto;
@protocol ProtoA, ProtoB;

@interface Original
@end

// CompatibilityAlias
@compatibility_alias Alias Original;

// ModuleImport, at global scope. Each name after the first is a submodule.
@import Foundation;
@import Foundation.NSString;
@import UIKit.UIView.UIViewGeometry;

// The alias names the same class, so it is usable as one
Alias *aliased;
Original *original;

# The Objective-C Grammar

A grammar for the Objective-C programming language, expressed using the notation of the Java Language
Specification (JLS) — a BNF variant chosen for being shorter and easier
to read than other common styles.

This document describes a translation unit as it appears after preprocessing.
Preprocessing directives (`#import`, `#include`, `#define`, `#pragma`, and the
rest) are not part of this grammar. Nor are the `NS_` and `CF_` macros supplied
by the Cocoa headers; each expands to some combination of the productions given
here, chiefly attribute specifiers (§5.9).

Every nonterminal appearing on a right-hand side is defined somewhere in this
document, except for the small number defined by narrative phrase where an
exhaustive listing would be impractical (§8).


## 1. Grammar Notation

### 1.1 JLS Conventions (unmodified)

The definition of a nonterminal is introduced by the name of the nonterminal
being defined, followed by a colon. One or more alternative definitions for the
nonterminal then follow on succeeding lines.

```
IfThenStatement:
  if ( Expression ) Statement
```

  - `{x}` denotes zero or more occurrences of `x`.
  - `[x]` denotes zero or one occurrences of `x`; that is, `x` is optional.
  - A very long right-hand side may be continued on a second line by clearly
    indenting the second line.
  - `(one of)` on the line following the colon signifies that each of the
    symbols on the succeeding line or lines is a separate alternative
    definition.
  - `but not` indicates expansions that are excluded.
  - A few nonterminals are defined by a narrative phrase where it would be
    impractical to list all the alternatives.

### 1.2 Modifications

JLS renders terminals in fixed-width type and nonterminals in italics. A
plain-text grammar has no such distinction available, so:

  1. **Casing.** Nonterminals are `PascalCase`. Every other symbol is a
     terminal, to appear in a program exactly as written.
  2. **Quoted meta-characters.** Objective-C uses `{`, `}`, `[`, and `]` as
     terminals, which collide with the JLS meta-symbols for repetition and
     optionality. A terminal brace or bracket is written in single quotes:
     `'{'`, `'}'`, `'['`, `']'`. Unquoted braces and brackets are always
     meta-syntax. The single quote is not itself a terminal of the language.

### 1.3 Goal Symbol

```
TranslationUnit:
  {ExternalDeclaration}
```

### 1.4 Scope and Dialect

The C substrate described here is C11. Where the language accepts a construct
only by extension, the extension is included if it is in routine use in
Objective-C code or in the Cocoa headers, and is marked as such in the
narrative. The extensions covered are those of GCC and clang: `__attribute__`
(§5.9), `typeof` (§5.3), statement expressions (§6.1), `asm` (§3, §7),
`__auto_type` (§5.3), `__thread` (§5.1), and `__alignof` (§6.5).

Objective-C++ is outside the scope of this document. Two constructs that
originate in C++ are nevertheless listed, because they are reachable from
Objective-C proper: the `::` in a scoped `AttributeName` (§5.9), and `@true`
and `@false` (§6.8), which are noted as ObjC++-only where they appear.


## 2. Lexical Structure

```
Token:
  Keyword
  Identifier
  Constant
  StringLiteral
  Punctuator
  Directive
```

A `Directive` is written as the `@` punctuator immediately followed by one of a
fixed set of identifiers (§2.5). The two are listed as a single token because
the identifier following `@` is drawn from a closed set and is not subject to
ordinary identifier lookup. An implementation will generally tolerate
whitespace or a comment between the `@` and the identifier.

`ContextualKeyword` is not an alternative of `Token`. Each contextual keyword
lexes as an `Identifier` and acquires its meaning only from the syntactic
position in which it appears.

### 2.1 Identifiers

```
Identifier:
  IdentifierNondigit
  Identifier IdentifierNondigit
  Identifier Digit

IdentifierNondigit:
  Nondigit
  UniversalCharacterName

Nondigit:
  (one of)
  _  $
     a b c d e f g h i j k l m n o p q r s t u v w x y z
     A B C D E F G H I J K L M N O P Q R S T U V W X Y Z

Digit:
  (one of)
  0 1 2 3 4 5 6 7 8 9

UniversalCharacterName:
  \u HexQuad
  \U HexQuad HexQuad

HexQuad:
  HexadecimalDigit HexadecimalDigit HexadecimalDigit HexadecimalDigit
```

`$` in an identifier is an extension. It is enabled by default and may be
disabled by a command-line option.

### 2.2 Keywords

```
Keyword:
  (one of)
  auto      break     case      char      const     continue  default   do
  double    else      enum      extern    float     for       goto      if
  inline    int       long      register  restrict  return    short     signed
  sizeof    static    struct    switch    typedef   union     unsigned  void
  volatile  while
  _Alignas  _Alignof  _Atomic   _Bool     _Complex  _Generic  _Imaginary
  _Noreturn _Static_assert       _Thread_local
  ObjectiveCKeyword
  ExtensionKeyword

ObjectiveCKeyword:
  (one of)
  __block  __kindof
  __bridge  __bridge_retained  __bridge_transfer
  __strong  __weak  __unsafe_unretained  __autoreleasing
  __covariant  __contravariant
  _Nonnull  _Nullable  _Null_unspecified
  __nonnull  __nullable  __null_unspecified
  __builtin_available
  __ptrauth
  __attribute__

ExtensionKeyword:
  (one of)
  asm  __asm  __asm__
  typeof  __typeof  __typeof__
  __alignof  __alignof__
  __auto_type  __thread  __extension__
```

```
ContextualKeyword:
  (one of)
  instancetype
  in  out  inout  bycopy  byref  oneway
  nonnull  nullable  null_unspecified  null_resettable
  getter  setter  readonly  readwrite
  assign  retain  copy  strong  weak  unsafe_unretained
  atomic  nonatomic  class  direct
```

`__objc_yes` and `__objc_no` are **not** listed as keywords. They lex as a
`BooleanConstant` (§2.3), which is an alternative of `Constant` and therefore
already an alternative of `Token`; listing them in both places would make
`Token` ambiguous.

`id`, `Class`, `SEL`, `IMP`, and `BOOL` are **not** keywords. They are type
names declared in `<objc/objc.h>`, implicitly available in every Objective-C
translation unit, and they reach the grammar through `TypedefName` (§5.3). `id`
and `Class` are nonetheless given their own alternatives in
`ObjectTypeSpecifier` (§5.4) because each accepts a `ProtocolReferenceList`,
which an arbitrary `TypedefName` does not.

`Protocol` is not a keyword either, and — unlike the names above — is not a
typedef name in Objective-C. `<objc/objc.h>` declares it with `@class Protocol;`
under `__OBJC__`, so it reaches the grammar through `ClassName` (§4.1). Only in
plain C is it a typedef.

`nil`, `Nil`, `YES`, and `NO` are macros, not tokens of the language. `YES` and
`NO` expand to `__objc_yes` and `__objc_no`, which exist so that a boolean
literal can be distinguished from an integer literal when boxed (§6.8).

`instancetype` is a contextual keyword. It is valid only as the return type of a
method declaration or definition, and lexes as an ordinary identifier elsewhere.

### 2.3 Constants

```
Constant:
  IntegerConstant
  FloatingConstant
  EnumerationConstant
  CharacterConstant
  BooleanConstant

EnumerationConstant:
  Identifier

BooleanConstant:
  (one of)
  __objc_yes  __objc_no

IntegerConstant:
  DecimalConstant [IntegerSuffix]
  OctalConstant [IntegerSuffix]
  HexadecimalConstant [IntegerSuffix]
  BinaryConstant [IntegerSuffix]

DecimalConstant:
  NonzeroDigit {Digit}

OctalConstant:
  0 {OctalDigit}

HexadecimalConstant:
  HexadecimalPrefix HexadecimalDigit {HexadecimalDigit}

BinaryConstant:
  BinaryPrefix BinaryDigit {BinaryDigit}

HexadecimalPrefix:
  (one of)
  0x  0X

BinaryPrefix:
  (one of)
  0b  0B

NonzeroDigit:
  (one of)
  1 2 3 4 5 6 7 8 9

OctalDigit:
  (one of)
  0 1 2 3 4 5 6 7

BinaryDigit:
  (one of)
  0 1

HexadecimalDigit:
  (one of)
  0 1 2 3 4 5 6 7 8 9 a b c d e f A B C D E F

IntegerSuffix:
  UnsignedSuffix [LongSuffix]
  UnsignedSuffix LongLongSuffix
  LongSuffix [UnsignedSuffix]
  LongLongSuffix [UnsignedSuffix]

UnsignedSuffix:
  (one of)
  u  U

LongSuffix:
  (one of)
  l  L

LongLongSuffix:
  (one of)
  ll  LL

FloatingConstant:
  DecimalFloatingConstant
  HexadecimalFloatingConstant

DecimalFloatingConstant:
  FractionalConstant [ExponentPart] [FloatingSuffix]
  DigitSequence ExponentPart [FloatingSuffix]

HexadecimalFloatingConstant:
  HexadecimalPrefix HexadecimalFractionalConstant BinaryExponentPart
    [FloatingSuffix]
  HexadecimalPrefix HexadecimalDigitSequence BinaryExponentPart
    [FloatingSuffix]

FractionalConstant:
  [DigitSequence] . DigitSequence
  DigitSequence .

ExponentPart:
  e [Sign] DigitSequence
  E [Sign] DigitSequence

BinaryExponentPart:
  p [Sign] DigitSequence
  P [Sign] DigitSequence

HexadecimalFractionalConstant:
  [HexadecimalDigitSequence] . HexadecimalDigitSequence
  HexadecimalDigitSequence .

Sign:
  (one of)
  +  -

DigitSequence:
  Digit {Digit}

HexadecimalDigitSequence:
  HexadecimalDigit {HexadecimalDigit}

FloatingSuffix:
  (one of)
  f  l  F  L

CharacterConstant:
  [CharacterPrefix] ' CChar {CChar} '

CharacterPrefix:
  (one of)
  L  u  U

CChar:
  any member of the source character set except the single quote, the
    backslash, or a newline
  EscapeSequence

EscapeSequence:
  SimpleEscapeSequence
  OctalEscapeSequence
  HexadecimalEscapeSequence
  UniversalCharacterName

SimpleEscapeSequence:
  (one of)
  \'  \"  \?  \\  \a  \b  \f  \n  \r  \t  \v

OctalEscapeSequence:
  \ OctalDigit [OctalDigit [OctalDigit]]

HexadecimalEscapeSequence:
  \x HexadecimalDigit {HexadecimalDigit}
```

`BinaryConstant` is an extension in C11; it is standard from C23 onward.

### 2.4 String Literals

```
StringLiteral:
  PlainStringLiteral
  ObjectStringLiteral

PlainStringLiteral:
  [EncodingPrefix] " {SChar} "

ObjectStringLiteral:
  @ " {SChar} "

EncodingPrefix:
  (one of)
  u8  u  U  L

SChar:
  any member of the source character set except the double quote, the
    backslash, or a newline
  EscapeSequence
```

A `StringLiteral` is a single token. Concatenation of adjacent literals is not
part of the token and is given instead as `StringLiteralSequence` (§6.1), which
is where a string literal enters the expression grammar.

An `EncodingPrefix` may not be applied to an `ObjectStringLiteral`. A
`PlainStringLiteral` denotes an array of characters; an `ObjectStringLiteral`
denotes a pointer to a string object.

### 2.5 Directives

```
Directive:
  (one of)
  @interface  @implementation  @protocol  @end
  @class  @compatibility_alias  @import
  @property  @synthesize  @dynamic
  @required  @optional
  @private  @protected  @public  @package
  @selector  @encode  @defs  @available
  @try  @catch  @finally  @throw
  @synchronized  @autoreleasepool
```

This list is closed. An `@` followed by any other identifier is ill-formed. The
closure of this list is also why an enumeration constant cannot be boxed by
writing `@` before its name (§6.8).

`@defs` is supported only under the legacy (32-bit) runtime and is rejected by
the modern runtime.

### 2.6 Punctuators

```
Punctuator:
  (one of)
  '['  ']'  (  )  '{'  '}'  .  ->
  ++  --  &  *  +  -  ~  !
  /  %  <<  >>  <  >  <=  >=  ==  !=  ^  |  &&  ||
  ?  :  ::  ;  ...
  =  *=  /=  %=  +=  -=  <<=  >>=  &=  ^=  |=
  ,  #  ##
  <:  :>  <%  %>  %:  %:%:
  @
```

The `^` punctuator serves both as the exclusive-OR operator and as the
introducer of a block pointer type (§5.7) and a block literal (§6.9). The `@`
punctuator introduces a `Directive` (§2.5), an `ObjectStringLiteral` (§2.4), and
the object literal and boxed expression forms (§6.8). The `::` punctuator
appears only in a scoped `AttributeName` (§5.9).


## 3. External Definitions

```
ExternalDeclaration:
  FunctionDefinition
  Declaration
  ClassInterface
  ClassImplementation
  CategoryInterface
  CategoryImplementation
  ClassExtension
  ProtocolDeclaration
  ClassDeclarationList
  ProtocolDeclarationList
  CompatibilityAlias
  ModuleImport
  AsmStatement
  ;

FunctionDefinition:
  DeclarationSpecifiers Declarator [DeclarationList] CompoundStatement

DeclarationList:
  Declaration {Declaration}
```

The `DeclarationList` in a `FunctionDefinition` supports parameter declarations
written after the parameter name list rather than inside it. The form is
obsolescent.

An `AsmStatement` (§7.4) appearing as an `ExternalDeclaration` is a file-scope
assembly definition and may not name operands.


## 4. Classes, Categories, and Protocols

### 4.1 Classes

```
ClassInterface:
  [AttributeSpecifierList] @interface ClassName [TypeParameterList]
    [Superclass] [ProtocolReferenceList] [InstanceVariables]
    [InterfaceDeclarationList] @end

Superclass:
  : SuperclassName [TypeArgumentList]

ClassImplementation:
  [AttributeSpecifierList] @implementation ClassName [: SuperclassName]
    [InstanceVariables] [ImplementationDefinitionList] @end

ClassName:
  Identifier

SuperclassName:
  Identifier
```

An angle-bracket list following `SuperclassName` may be either a
`TypeArgumentList` supplying arguments to a generic superclass or a
`ProtocolReferenceList` naming protocols adopted by the class being declared.
The two are distinguished by resolving each `Identifier` in the list: a protocol
name yields a `ProtocolReferenceList`, anything else a `TypeArgumentList`. Both
may appear, in that order, as in
`@interface Foo : NSArray<NSString *> <NSCopying>`.

A `ClassImplementation` takes neither a `TypeParameterList` nor a
`TypeArgumentList`; lightweight generics are erased and exist only in the
interface. An `AttributeSpecifierList` is accepted before `@implementation` but
has no effect there; attributes belong on the interface. An
`AttributeSpecifierList` may not appear between `@interface` and the class name
(§5.9).

### 4.2 Categories and Class Extensions

```
CategoryInterface:
  [AttributeSpecifierList] @interface ClassName [TypeParameterList]
    ( CategoryName ) [ProtocolReferenceList]
    [InterfaceDeclarationList] @end

CategoryImplementation:
  @implementation ClassName ( CategoryName )
    [ImplementationDefinitionList] @end

ClassExtension:
  [AttributeSpecifierList] @interface ClassName [TypeParameterList] ( )
    [ProtocolReferenceList] [InstanceVariables]
    [InterfaceDeclarationList] @end

CategoryName:
  Identifier
```

A category on a generic class carries a `TypeParameterList`, not a
`TypeArgumentList`: it rebinds the class's type parameters for the duration of
the category rather than instantiating them.

A `ClassExtension` is a `CategoryInterface` with an omitted `CategoryName`. It
is given its own production because, unlike a category, it may declare instance
variables and must appear in the same translation unit as the corresponding
`ClassImplementation`. An implementation parses an `InstanceVariables` block
after a named `CategoryName` as well, in order to diagnose it; the construct is
not well-formed.

### 4.3 Protocols

```
ProtocolDeclaration:
  [AttributeSpecifierList] @protocol ProtocolName [ProtocolReferenceList]
    {ProtocolSection} @end

ProtocolSection:
  @required {InterfaceDeclaration}
  @optional {InterfaceDeclaration}
  InterfaceDeclaration {InterfaceDeclaration}

ProtocolDeclarationList:
  @protocol ProtocolList ;

ProtocolReferenceList:
  < ProtocolList >

ProtocolList:
  ProtocolName {, ProtocolName}

ProtocolName:
  Identifier
```

`@required` and `@optional` are meaningful only within a `ProtocolDeclaration`.
An implementation accepts them anywhere an `InterfaceDeclaration` may appear and
rejects them elsewhere by diagnosis rather than by parse failure.

### 4.4 Forward Declarations, Aliases, and Modules

```
ClassDeclarationList:
  @class ClassList ;

ClassList:
  ClassName [TypeParameterList] {, ClassName [TypeParameterList]}

CompatibilityAlias:
  @compatibility_alias Identifier ClassName ;

ModuleImport:
  @import ModuleName ;

ModuleName:
  Identifier {. Identifier}
```

Each `Identifier` after the first in a `ModuleName` names a submodule of the one
preceding it. A `ModuleImport` must appear at global scope and requires module
support to be enabled.

### 4.5 Instance Variables

```
InstanceVariables:
  '{' {VisibilitySection} '}'

VisibilitySection:
  VisibilitySpecification {StructDeclaration}
  StructDeclaration {StructDeclaration}

VisibilitySpecification:
  (one of)
  @private  @protected  @public  @package
```

Instance variables declared before any `VisibilitySpecification` take a default
visibility that depends on the enclosing construct: `@protected` in a
`ClassInterface`, and `@private` in a `ClassImplementation`, a `ClassExtension`,
or a `CategoryInterface`. Only the interface default is `@protected`; a variable
declared in an implementation or an extension is not visible outside the
translation unit that declares it, and defaults accordingly.

### 4.6 Members

```
InterfaceDeclarationList:
  InterfaceDeclaration {InterfaceDeclaration}

InterfaceDeclaration:
  Declaration
  MethodDeclaration
  PropertyDeclaration
  ;

ImplementationDefinitionList:
  ImplementationDefinition {ImplementationDefinition}

ImplementationDefinition:
  FunctionDefinition
  Declaration
  MethodDefinition
  PropertyImplementation
  ;
```

A `FunctionDefinition` inside a `ClassImplementation` is an ordinary C function.
It is granted access to the class's instance variables and is visible to the
methods of the same `@implementation` regardless of declaration order.

### 4.7 Methods

```
MethodDeclaration:
  MethodKind [MethodType] [AttributeSpecifierList] MethodSelector
    [AttributeSpecifierList] ;

MethodDefinition:
  MethodKind [MethodType] [AttributeSpecifierList] MethodSelector
    [DeclarationList] CompoundStatement

MethodKind:
  (one of)
  +  -

MethodType:
  ( {ProtocolQualifier} [TypeName] )

ProtocolQualifier:
  (one of)
  in  out  inout  bycopy  byref  oneway

MethodSelector:
  Selector
  KeywordDeclarator {KeywordDeclarator} [MethodParameterSuffix]

MethodParameterSuffix:
  , ParameterDeclaration {, ParameterDeclaration} [, ...]
  , ...

KeywordDeclarator:
  [Selector] : [MethodType] [AttributeSpecifierList] Identifier

Selector:
  Identifier
  Keyword  but not __attribute__
```

An `AttributeSpecifierList` may appear in three positions on a method: between
the `MethodType` and the selector, between a keyword's parameter type and the
parameter name, and after the complete `MethodSelector`. The last position is
the one used by `NS_DESIGNATED_INITIALIZER`, `NS_SWIFT_NAME`, and the
deprecation macros. `__attribute__` is excluded from `Selector` so that an
attribute in the first position is not mistaken for a unary selector.

A `MethodType` with the `TypeName` omitted supplies only distributed-object
qualifiers and leaves the type unstated; `- (oneway)shutdown;` is well-formed.
An empty `MethodType`, `- ()foo;`, is derivable but is rejected by diagnosis.

`ProtocolQualifier` is meaningful only inside a `MethodType`, where it describes
distributed-object semantics. It is not a general type qualifier and may not
appear in an ordinary declaration.

Any reserved word may be spelled as a selector piece. Historically the Apple and
GNU grammars admitted only a restricted subset — `enum`, `struct`, `union`,
`if`, `else`, `while`, `do`, `for`, `switch`, `case`, `default`, `break`,
`continue`, `return`, `goto`, `asm`, `sizeof`, `typeof`, `__alignof`,
`unsigned`, `long`, `const`, `short`, `volatile`, `signed`, `restrict`,
`_Complex`, `in`, `out`, `inout`, `bycopy`, `byref`, `oneway`, `int`, `char`,
`float`, `double`, `void`, `_Bool` — but a current implementation accepts any
token that carries an identifier spelling, so `static`, `typedef`, `extern`, and
`auto` are selectors too. Every `ContextualKeyword` (§2.2) already lexes as an
`Identifier` and so is covered by the first alternative of `Selector`.

The `DeclarationList` alternative of `MethodDefinition` supports parameter
declarations written after the selector and is obsolescent.

### 4.8 Properties

```
PropertyDeclaration:
  @property [( [PropertyAttributeList] )] DeclarationSpecifiers
    PropertyDeclaratorList ;

PropertyDeclaratorList:
  Declarator {, Declarator}

PropertyAttributeList:
  PropertyAttribute {, PropertyAttribute}

PropertyAttribute:
  PropertyAttributeName
  getter = Selector
  setter = Selector :

PropertyAttributeName:
  (one of)
  class  direct  atomic  nonatomic  readonly  readwrite
  assign  retain  copy  strong  weak  unsafe_unretained
  nullable  nonnull  null_resettable  null_unspecified

PropertyImplementation:
  @synthesize PropertySynthesizeList ;
  @dynamic PropertySynthesizeList ;

PropertySynthesizeList:
  PropertySynthesizeItem {, PropertySynthesizeItem}

PropertySynthesizeItem:
  Identifier [= Identifier]
```

`PropertyAttributeName` is a closed set. An identifier appearing in a
`PropertyAttributeList` that is not one of the listed names, and is not `getter`
or `setter`, is a syntax error, not a semantic one. The list itself may be
empty: `@property () NSString *name;` is accepted.

Note the trailing colon in `setter = Selector :`, which is part of the setter's
selector.

A `PropertyDeclaration` declares one property per `Declarator`, so
`@property (nonatomic, copy) NSString *first, *last;` declares two. A property
declarator may not carry a bit-field width, and may not be abstract: every
declarator must name an identifier.


## 5. Declarations

```
Declaration:
  DeclarationSpecifiers [InitDeclaratorList] ;
  StaticAssertDeclaration

DeclarationSpecifiers:
  DeclarationSpecifier {DeclarationSpecifier}

DeclarationSpecifier:
  StorageClassSpecifier
  TypeSpecifier
  TypeQualifier
  FunctionSpecifier
  AlignmentSpecifier
  AttributeSpecifier

InitDeclaratorList:
  InitDeclarator {, InitDeclarator}

InitDeclarator:
  Declarator [= Initializer]

StaticAssertDeclaration:
  _Static_assert ( ConstantExpression , StringLiteralSequence ) ;
```

### 5.1 Storage Class Specifiers

```
StorageClassSpecifier:
  (one of)
  typedef  extern  static  _Thread_local  auto  register
  __block  __thread
```

`__block` marks a local variable as shared with, and mutable from, any block
that captures it. It is mutually exclusive with `auto`, `register`, and
`static`.

`__thread` is the extension spelling of `_Thread_local`.

### 5.2 Function and Alignment Specifiers

```
FunctionSpecifier:
  (one of)
  inline  _Noreturn

AlignmentSpecifier:
  _Alignas ( TypeName )
  _Alignas ( ConstantExpression )
```

### 5.3 Type Specifiers

```
TypeSpecifier:
  void
  char
  short
  int
  long
  float
  double
  signed
  unsigned
  _Bool
  _Complex
  __auto_type
  AtomicTypeSpecifier
  TypeofSpecifier
  StructOrUnionSpecifier
  EnumSpecifier
  TypedefName
  ObjectTypeSpecifier

AtomicTypeSpecifier:
  _Atomic ( TypeName )

TypeofSpecifier:
  TypeofKeyword ( Expression )
  TypeofKeyword ( TypeName )

TypeofKeyword:
  (one of)
  typeof  __typeof  __typeof__

TypedefName:
  Identifier
```

`TypeofSpecifier` and `__auto_type` are extensions. `__typeof__` is pervasive in
Objective-C code, chiefly in the strong/weak dance around a captured `self`:
`__strong __typeof__(weakSelf) strongSelf = weakSelf;`.

The two alternatives of `TypeofSpecifier` are distinguished by resolving the
contents of the parentheses; a `TypeName` is preferred where both parse.

### 5.4 Object Type Specifiers

```
ObjectTypeSpecifier:
  id [ProtocolReferenceList]
  Class [ProtocolReferenceList]
  instancetype
  ClassName [TypeArgumentList] [ProtocolReferenceList]
  TypeParameterName
```

`ClassName`, `TypeParameterName`, and `TypedefName` (§5.3) are all a bare
`Identifier`; an implementation resolves the ambiguity by lookup.

`instancetype` is valid only as the return type of a method.

### 5.5 Lightweight Generics

```
TypeParameterList:
  < TypeParameter {, TypeParameter} >

TypeParameter:
  [Variance] Identifier [: TypeName]

TypeParameterName:
  Identifier

Variance:
  (one of)
  __covariant  __contravariant

TypeArgumentList:
  < TypeName {, TypeName} >
```

The bound in a `TypeParameter` is a full `TypeName`, pointer included:
`@interface Container<T : NSView *>`.

`TypeArgumentList` and `ProtocolReferenceList` are distinguished only
semantically; both are angle-bracket lists, and an implementation resolves each
`Identifier` within them by lookup. A list containing a `Variance` or a bound is
unambiguously a `TypeParameterList`.

### 5.6 Type Qualifiers

```
TypeQualifier:
  const
  restrict
  volatile
  _Atomic
  OwnershipQualifier
  NullabilityQualifier
  PtrauthQualifier
  __kindof

TypeQualifierList:
  TypeQualifier {TypeQualifier}

OwnershipQualifier:
  (one of)
  __strong  __weak  __unsafe_unretained  __autoreleasing

NullabilityQualifier:
  (one of)
  _Nonnull  _Nullable  _Null_unspecified
  __nonnull  __nullable  __null_unspecified
  nonnull  nullable  null_unspecified

PtrauthQualifier:
  __ptrauth ( BalancedTokenSequence )
```

An `OwnershipQualifier` is meaningful only under automatic reference counting.

The underscore-free spellings of `NullabilityQualifier` are accepted only inside
a `MethodType` (§4.7); elsewhere they lex as ordinary identifiers. The same
spellings appear in a `PropertyAttributeList` (§4.8), but they arrive there as
`PropertyAttributeName`, not through this production.

`__kindof` qualifies an object pointer type to admit any subclass of the named
class while preserving the class's interface for message sends.

`PtrauthQualifier` is a recent extension supporting pointer authentication on
targets that provide it. Its argument list is given as a
`BalancedTokenSequence` here rather than enumerated, because the set of
accepted arguments is defined by the implementation rather than by the language.

### 5.7 Declarators

```
Declarator:
  [Pointer] DirectDeclarator [AttributeSpecifierList]

DirectDeclarator:
  Identifier
  ( Declarator )
  DirectDeclarator '[' [TypeQualifierList] [AssignmentExpression] ']'
  DirectDeclarator '[' static [TypeQualifierList] AssignmentExpression ']'
  DirectDeclarator '[' TypeQualifierList static AssignmentExpression ']'
  DirectDeclarator '[' [TypeQualifierList] * ']'
  DirectDeclarator ( ParameterTypeList )
  DirectDeclarator ( [IdentifierList] )

Pointer:
  * [TypeQualifierList] [Pointer]
  ^ [TypeQualifierList] [Pointer]

ParameterTypeList:
  ParameterList [, ...]

ParameterList:
  ParameterDeclaration {, ParameterDeclaration}

ParameterDeclaration:
  DeclarationSpecifiers Declarator
  DeclarationSpecifiers [AbstractDeclarator]

IdentifierList:
  Identifier {, Identifier}

TypeName:
  SpecifierQualifierList [AbstractDeclarator]

SpecifierQualifierList:
  SpecifierQualifier {SpecifierQualifier}

SpecifierQualifier:
  TypeSpecifier
  TypeQualifier
  AttributeSpecifier

AbstractDeclarator:
  Pointer
  [Pointer] DirectAbstractDeclarator

DirectAbstractDeclarator:
  ( AbstractDeclarator )
  [DirectAbstractDeclarator] '[' [TypeQualifierList] [AssignmentExpression] ']'
  [DirectAbstractDeclarator] '[' * ']'
  [DirectAbstractDeclarator] ( [ParameterTypeList] )
```

The `^` alternative of `Pointer` introduces a block pointer type. It occupies
the position a `*` would occupy: `int (^handler)(NSString *)` declares a
variable holding a block that takes a string and returns an integer. A block
pointer may not be dereferenced.

### 5.8 Structures, Unions, and Enumerations

```
StructOrUnionSpecifier:
  StructOrUnion [Identifier] '{' StructDeclarationList '}'
  StructOrUnion Identifier
  StructOrUnion [Identifier] '{' @defs ( ClassName ) '}'

StructOrUnion:
  (one of)
  struct  union

StructDeclarationList:
  StructDeclaration {StructDeclaration}

StructDeclaration:
  SpecifierQualifierList [StructDeclaratorList] ;
  StaticAssertDeclaration

StructDeclaratorList:
  StructDeclarator {, StructDeclarator}

StructDeclarator:
  Declarator
  [Declarator] : ConstantExpression

EnumSpecifier:
  enum [Identifier] [: TypeName] '{' EnumeratorList [,] '}'
  enum Identifier [: TypeName]

EnumeratorList:
  Enumerator {, Enumerator}

Enumerator:
  EnumerationConstant [AttributeSpecifierList] [= ConstantExpression]
```

The `: TypeName` in an `EnumSpecifier` fixes the underlying type of the
enumeration. It is the form produced by the `NS_ENUM` and `NS_OPTIONS` macros
and is pervasive throughout the Cocoa headers. A fixed underlying type makes the
enumeration type complete at the point of the specifier and determines how a
value of the type is boxed (§6.8).

The `: TypeName` is optional on the bodyless alternative as well, which is what
makes the `NS_ENUM` expansion derivable. `NS_ENUM(NSInteger, Foo)` expands to

```
enum Foo : NSInteger Foo; enum Foo : NSInteger
```

whose first half is a bodyless `EnumSpecifier` carrying a fixed underlying type,
used as the `DeclarationSpecifiers` of a `Declaration` whose sole `Declarator`
is `Foo`.

`@defs` yields the instance variable layout of a class as a sequence of
structure members. It is supported only under the legacy runtime.

### 5.9 Attributes

```
AttributeSpecifierList:
  AttributeSpecifier {AttributeSpecifier}

AttributeSpecifier:
  __attribute__ ( ( [AttributeList] ) )
  '[' '[' AttributeList ']' ']'

AttributeList:
  Attribute {, Attribute}

Attribute:
  AttributeName [( [BalancedTokenSequence] )]

AttributeName:
  Identifier
  Identifier :: Identifier

BalancedTokenSequence:
  any token sequence in which brackets, parentheses, and braces are
  balanced
```

The brackets of the second `AttributeSpecifier` alternative are terminals and so
are written in single quotes per §1.2. An empty `__attribute__(())` is accepted.

An `AttributeSpecifierList` may appear wherever a `DeclarationSpecifier` or
`SpecifierQualifier` may appear, after a `Declarator`, on an `Enumerator`, and
additionally before `@interface` (§4.1), before `@protocol` (§4.3), and in the
three method positions of §4.7. It may not appear between `@interface` and the
class name.

The scoped form of `AttributeName` originates in C++ and is reached from
Objective-C only through the bracketed `AttributeSpecifier`.

### 5.10 Initializers

```
Initializer:
  AssignmentExpression
  '{' InitializerList [,] '}'

InitializerList:
  [Designation] Initializer {, [Designation] Initializer}

Designation:
  DesignatorList =

DesignatorList:
  Designator {Designator}

Designator:
  '[' ConstantExpression ']'
  . Identifier
```


## 6. Expressions

### 6.1 Primary Expressions

```
PrimaryExpression:
  Identifier
  Constant
  StringLiteralSequence
  ( Expression )
  StatementExpression
  GenericSelection
  MessageExpression
  SelectorExpression
  ProtocolExpression
  EncodeExpression
  BoxedExpression
  ArrayLiteral
  DictionaryLiteral
  BlockLiteral
  AvailabilityCheck

StringLiteralSequence:
  PlainStringLiteral {PlainStringLiteral}
  ObjectStringLiteral {StringLiteral}

StatementExpression:
  ( CompoundStatement )

GenericSelection:
  _Generic ( AssignmentExpression , GenericAssociationList )

GenericAssociationList:
  GenericAssociation {, GenericAssociation}

GenericAssociation:
  TypeName : AssignmentExpression
  default : AssignmentExpression
```

Adjacent string literals are concatenated. The leading `@` may be repeated or
omitted on continuation pieces of an `ObjectStringLiteral`, so `@"a" @"b"` and
`@"a" "b"` are the same object; but a sequence beginning with a
`PlainStringLiteral` may not later acquire an `@`, which is why the first
alternative admits only plain pieces.

`StatementExpression` is an extension. Its value is that of the last
`ExpressionStatement` in the compound statement.

`self` and `_cmd` are ordinary identifiers naming the implicit parameters of a
method body. They are not keywords.

### 6.2 Postfix Expressions

```
PostfixExpression:
  PrimaryExpression
  PostfixExpression '[' Expression ']'
  PostfixExpression ( [ArgumentExpressionList] )
  PostfixExpression . Identifier
  PostfixExpression -> Identifier
  PostfixExpression ++
  PostfixExpression --
  ( TypeName ) '{' InitializerList [,] '}'
  ClassName . Identifier

ArgumentExpressionList:
  AssignmentExpression {, AssignmentExpression}
```

The `.` operator is overloaded. Applied to a structure or union it selects a
member; applied to an object pointer it is *property dot syntax*, rewritten to a
message send of the property's getter, or of its setter when the expression is
the left operand of a simple assignment. The `ClassName . Identifier`
alternative extends dot syntax to class properties, as in `NSObject.class`. A
class name lexes as an `Identifier` and so is already derivable from
`PrimaryExpression`; the alternative is listed separately to record that the
receiver is resolved as a class rather than as an ordinary variable, which
changes the rewriting.

The subscript operator is likewise overloaded. Applied to an object pointer with
an integral subscript it becomes a send of `objectAtIndexedSubscript:` or
`setObject:atIndexedSubscript:`; with an object pointer subscript it becomes a
send of `objectForKeyedSubscript:` or `setObject:forKeyedSubscript:`. Object
subscripting requires the modern runtime, which forbids arithmetic on object
pointers and so leaves the syntax unambiguous.

### 6.3 Message Expressions

```
MessageExpression:
  '[' Receiver MessageSelector ']'

Receiver:
  Expression
  super
  ClassName [TypeArgumentList]

MessageSelector:
  Selector
  KeywordArgument {KeywordArgument}

KeywordArgument:
  [Selector] : AssignmentExpression {, AssignmentExpression}
```

The `ClassName` alternative of `Receiver` is subsumed by `Expression` except
when a `TypeArgumentList` follows; it is listed to make the generic case
explicit. `super` is a receiver only, never an expression in its own right. The
repetition in `KeywordArgument` supplies the trailing arguments of a variadic
method and is permitted only on the final keyword.

### 6.4 Reflection Expressions

```
SelectorExpression:
  @selector ( SelectorName )

SelectorName:
  Selector
  KeywordName {KeywordName}

KeywordName:
  [Selector] :

ProtocolExpression:
  @protocol ( ProtocolName )

EncodeExpression:
  @encode ( TypeName )
```

### 6.5 Unary and Cast Expressions

```
UnaryExpression:
  PostfixExpression
  ++ UnaryExpression
  -- UnaryExpression
  UnaryOperator CastExpression
  sizeof UnaryExpression
  sizeof ( TypeName )
  _Alignof ( TypeName )
  AlignofKeyword UnaryExpression
  AlignofKeyword ( TypeName )

AlignofKeyword:
  (one of)
  __alignof  __alignof__

UnaryOperator:
  (one of)
  &  *  +  -  ~  !

CastExpression:
  UnaryExpression
  ( TypeName ) CastExpression
  ( BridgeKeyword TypeName ) CastExpression

BridgeKeyword:
  (one of)
  __bridge  __bridge_retained  __bridge_transfer
```

`AlignofKeyword` is the extension spelling and, unlike `_Alignof`, applies to an
expression as well as to a type name.

A `BridgeKeyword` casts between an object pointer and a non-object pointer type,
stating what happens to ownership of the value. It is required only under
automatic reference counting.

### 6.6 Binary Expressions

```
MultiplicativeExpression:
  CastExpression
  MultiplicativeExpression * CastExpression
  MultiplicativeExpression / CastExpression
  MultiplicativeExpression % CastExpression

AdditiveExpression:
  MultiplicativeExpression
  AdditiveExpression + MultiplicativeExpression
  AdditiveExpression - MultiplicativeExpression

ShiftExpression:
  AdditiveExpression
  ShiftExpression << AdditiveExpression
  ShiftExpression >> AdditiveExpression

RelationalExpression:
  ShiftExpression
  RelationalExpression < ShiftExpression
  RelationalExpression > ShiftExpression
  RelationalExpression <= ShiftExpression
  RelationalExpression >= ShiftExpression

EqualityExpression:
  RelationalExpression
  EqualityExpression == RelationalExpression
  EqualityExpression != RelationalExpression

AndExpression:
  EqualityExpression
  AndExpression & EqualityExpression

ExclusiveOrExpression:
  AndExpression
  ExclusiveOrExpression ^ AndExpression

InclusiveOrExpression:
  ExclusiveOrExpression
  InclusiveOrExpression | ExclusiveOrExpression

LogicalAndExpression:
  InclusiveOrExpression
  LogicalAndExpression && InclusiveOrExpression

LogicalOrExpression:
  LogicalAndExpression
  LogicalOrExpression || LogicalAndExpression
```

Within a `TypeParameterList`, `TypeArgumentList`, or `ProtocolReferenceList`,
`<` and `>` are delimiters rather than relational operators.

### 6.7 Conditional and Assignment Expressions

```
ConditionalExpression:
  LogicalOrExpression
  LogicalOrExpression ? Expression : ConditionalExpression

AssignmentExpression:
  ConditionalExpression
  UnaryExpression AssignmentOperator AssignmentExpression

AssignmentOperator:
  (one of)
  =  *=  /=  %=  +=  -=  <<=  >>=  &=  ^=  |=

Expression:
  AssignmentExpression {, AssignmentExpression}

ConstantExpression:
  ConditionalExpression
```

An object subscript expression is an lvalue and may appear as the left operand
of a simple assignment, but not of a compound assignment.

### 6.8 Object Literals

```
BoxedExpression:
  @ ( Expression )
  @ [Sign] NumericConstant
  @ CharacterConstant
  @ BooleanConstant
  @ true
  @ false

NumericConstant:
  IntegerConstant
  FloatingConstant

ArrayLiteral:
  @ '[' [AssignmentExpressionList] ']'

AssignmentExpressionList:
  AssignmentExpression {, AssignmentExpression} [,]

DictionaryLiteral:
  @ '{' [KeyValueList] '}'

KeyValueList:
  KeyValuePair {, KeyValuePair} [,]

KeyValuePair:
  AssignmentExpression : AssignmentExpression
```

`@true` and `@false` are accepted only when Objective-C is layered over C++. In
Objective-C proper, `@YES` and `@NO` are the spellings, and expand to
`@__objc_yes` and `@__objc_no`.

An enumeration constant may not be boxed directly. `@` followed by an identifier
introduces a `Directive`, and that list is closed (§2.5), so `@SomeConstant` is
ill-formed whatever `SomeConstant` names. The constant must be written inside a
`@ ( Expression )`.

### 6.9 Blocks

```
BlockLiteral:
  ^ [TypeName] [BlockParameters] CompoundStatement

BlockParameters:
  ( [ParameterTypeList] )
```

The optional `TypeName` gives an explicit return type; when omitted the return
type is inferred from the block's `return` statements, and is `void` if there
are none. `( void )` and an empty parameter list are both derivable from
`( [ParameterTypeList] )` and are equivalent in a block literal, unlike in a
function declarator, where an empty list leaves the parameters unspecified.

A return type written as a `TypeName` may itself contain the parameter list, as
in `^ int ((*)(float x))(char) { ... }`; the `[TypeName] [BlockParameters]`
factoring above covers the common spelling rather than every equivalent one.

### 6.10 Availability Checks

```
AvailabilityCheck:
  @available ( AvailabilitySpecList , * )
  __builtin_available ( AvailabilitySpecList , * )

AvailabilitySpecList:
  AvailabilitySpec {, AvailabilitySpec}

AvailabilitySpec:
  PlatformName VersionTuple

PlatformName:
  Identifier

VersionTuple:
  DigitSequence [. DigitSequence [. DigitSequence]]
```

The trailing `, *` is mandatory. It denotes every platform not named in the
`AvailabilitySpecList` and makes the check succeed on those platforms.

An `AvailabilityCheck` is an ordinary primary expression and may appear wherever
one may. It guards the availability of a declaration, however, only when it is
the entire controlling expression of an `if` statement or the operand of a `!`
applied to such an expression. An implementation will parse other uses and
diagnose them as non-guarding.

`__builtin_available` is a spelling of the same construct usable where the `@`
form is not.


## 7. Statements

```
Statement:
  LabeledStatement
  CompoundStatement
  ExpressionStatement
  SelectionStatement
  IterationStatement
  JumpStatement
  TryStatement
  ThrowStatement
  SynchronizedStatement
  AutoreleasePoolStatement
  AsmStatement

LabeledStatement:
  Identifier : Statement
  case ConstantExpression : Statement
  default : Statement

CompoundStatement:
  '{' {BlockItem} '}'

BlockItem:
  Declaration
  Statement

ExpressionStatement:
  [Expression] ;

SelectionStatement:
  if ( Expression ) Statement
  if ( Expression ) Statement else Statement
  switch ( Expression ) Statement

JumpStatement:
  goto Identifier ;
  continue ;
  break ;
  return [Expression] ;
```

`goto`, `break`, and `continue` do not transfer control out of a `BlockLiteral`.
An exception thrown inside a block propagates normally.

### 7.1 Iteration

```
IterationStatement:
  while ( Expression ) Statement
  do Statement while ( Expression ) ;
  for ( [Expression] ; [Expression] ; [Expression] ) Statement
  for ( Declaration [Expression] ; [Expression] ) Statement
  for ( DeclarationSpecifiers Declarator in Expression ) Statement
  for ( Expression in Expression ) Statement
```

The last two alternatives are the fast-enumeration statement. The second
`Expression` must evaluate to an object conforming to `NSFastEnumeration`.

In the fourth alternative — the C99 form, with a declaration in the
initialisation clause — no semicolon is written after the `Declaration`, because
a `Declaration` already ends in one. The `[Expression] ;` that follows it is the
controlling expression and its terminator, not the initialiser's.

### 7.2 Exception Handling

```
TryStatement:
  @try CompoundStatement CatchClause {CatchClause} [FinallyClause]
  @try CompoundStatement FinallyClause

CatchClause:
  @catch ( ParameterDeclaration ) CompoundStatement
  @catch ( ... ) CompoundStatement

FinallyClause:
  @finally CompoundStatement

ThrowStatement:
  @throw [Expression] ;
```

A `ThrowStatement` with the `Expression` omitted rethrows the exception
currently being handled and is valid only within a `CatchClause`. A
`TryStatement` requires at least one `CatchClause` or a `FinallyClause`.

### 7.3 Synchronization and Autorelease Pools

```
SynchronizedStatement:
  @synchronized ( Expression ) CompoundStatement

AutoreleasePoolStatement:
  @autoreleasepool CompoundStatement
```

The `Expression` in a `SynchronizedStatement` must be an object pointer; the
object serves as the lock for the duration of the compound statement.

### 7.4 Assembly

```
AsmStatement:
  AsmKeyword [TypeQualifier] ( BalancedTokenSequence ) ;

AsmKeyword:
  (one of)
  asm  __asm  __asm__
```

`AsmStatement` is an extension. Its interior — the template string, the output
and input operand lists, the clobber list, and any goto labels — is given as a
`BalancedTokenSequence` rather than enumerated, because its form varies by
implementation and by target.


## 8. Symbols Defined by Narrative

The following nonterminals are defined by narrative phrase rather than by
enumeration, because listing every alternative would be impractical:

`CChar`, `SChar`, `BalancedTokenSequence`.

All other nonterminals used in this document are defined within it.
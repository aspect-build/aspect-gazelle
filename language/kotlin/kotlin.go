package gazelle

import (
	"path"
	"strings"

	"github.com/emirpasic/gods/v2/sets/treeset"

	jvm_java "github.com/bazel-contrib/rules_jvm/java/gazelle/private/java"

	jvm_types "github.com/bazel-contrib/rules_jvm/java/gazelle/private/types"
)

// IsNativeImport returns true if the import path refers to a native Kotlin
// or Java library (e.g. packages starting with "kotlin.", "kotlinx.", or
// standard JDK packages).
func IsNativeImport(impt string) bool {
	if strings.HasPrefix(impt, "kotlin.") || strings.HasPrefix(impt, "kotlinx.") {
		return true
	}

	jvm_import := jvm_types.NewPackageName(impt)

	// Java native/standard libraries
	if jvm_java.IsStdlib(jvm_import) {
		return true
	}

	return false
}

type KotlinTarget struct {
	Imports *treeset.Set[ImportStatement]
}

/**
 * Information for kotlin library target including:
 * - kotlin files
 * - kotlin import statements from all files
 * - kotlin packages implemented
 */
type KotlinLibTarget struct {
	KotlinTarget

	Packages *treeset.Set[string]
	Files    *treeset.Set[string]
}

func NewKotlinLibTarget() *KotlinLibTarget {
	return &KotlinLibTarget{
		KotlinTarget: KotlinTarget{
			Imports: treeset.NewWith(importStatementComparator),
		},
		Packages: treeset.NewWith(strings.Compare),
		Files:    treeset.NewWith(strings.Compare),
	}
}

/**
 * Information for kotlin binary (main() method) including:
 * - kotlin import statements from all files
 * - the package
 * - the file
 */
type KotlinBinTarget struct {
	KotlinTarget

	File    string
	Package string
}

func NewKotlinBinTarget(file, pkg string) *KotlinBinTarget {
	return &KotlinBinTarget{
		KotlinTarget: KotlinTarget{
			Imports: treeset.NewWith(importStatementComparator),
		},
		File:    file,
		Package: pkg,
	}
}

// packagesKey is the name of a private attribute set on generated kt_library
// rules. This attribute contains the KotlinTarget for the target.
const packagesKey = "_kotlin_package"

func toBinaryTargetName(mainFile string) string {
	base := strings.ToLower(strings.TrimSuffix(path.Base(mainFile), path.Ext(mainFile)))

	// TODO: move target name template to directive
	return base + "_bin"
}

// KotlinTestTarget represents target information for a Kotlin test target,
// containing the list of test source files, the package name, and the
// fully-qualified test class name.
type KotlinTestTarget struct {
	KotlinTarget

	Files     []string
	Package   string
	TestClass string
}

// NewKotlinTestTarget creates a new KotlinTestTarget with initialized import treeset.
func NewKotlinTestTarget(files []string, pkg string, testClass string) *KotlinTestTarget {
	return &KotlinTestTarget{
		KotlinTarget: KotlinTarget{
			Imports: treeset.NewWith(importStatementComparator),
		},
		Files:     files,
		Package:   pkg,
		TestClass: testClass,
	}
}

// toTestTargetName returns the auto-generated target name for a Kotlin test rule
// based on the test source file name.
func toTestTargetName(mainFile string) string {
	base := strings.ToLower(strings.TrimSuffix(path.Base(mainFile), path.Ext(mainFile)))

	// TODO: move target name template to directive
	return base + "_test"
}

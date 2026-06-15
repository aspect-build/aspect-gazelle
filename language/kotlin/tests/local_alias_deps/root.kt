package test.root

// Aliased import of a top-level class from a local target.
import test.shapes.Rectangle as Rect

// Star import from a local target.
import test.colors.*

// Plain import from the same local target as the star import (dedups to one dep).
import test.colors.Blue

fun use_imports() {
    Rect(1.0, 2.0)
    Blue()
    Red()
}

package test.consumer

// Both //earth and //moon declare `package test.shared`, so resolving this
// import by package namespace finds two providers. With the current
// package-level resolution this is an unresolvable conflict; precise
// top-level-symbol resolution (which would pick //earth) is added in a
// follow-up PR.
import test.shared.Earth

fun use() {
    Earth()
}

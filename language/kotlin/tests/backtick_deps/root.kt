package test.root

// The provider declares `package test.widgets` plainly, but here the middle
// segment is backtick-escaped. Resolution only succeeds because the parser
// normalizes the backtick-quoted identifier to `widgets`.
import test.`widgets`.Thing

fun use() {
    Thing()
}

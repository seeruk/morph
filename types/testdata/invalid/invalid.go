//nolint:all
package invalid

func Broken() {
	var value DoesNotExist
	_ = value
}

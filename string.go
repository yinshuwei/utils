package utils

// Cat Join elements
// cony from string.Join
func Cat(a ...string) string {
	switch len(a) {
	case 0:
		return ""
	case 1:
		return a[0]
	case 2:
		return a[0] + a[1]
	case 3:
		return a[0] + a[1] + a[2]
	case 4:
		return a[0] + a[1] + a[2] + a[3]
	case 5:
		return a[0] + a[1] + a[2] + a[3] + a[4]
	}
	n := 0
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}
	b := make([]byte, n)
	bp := copy(b, a[0])
	for _, s := range a[1:] {
		bp += copy(b[bp:], s)
	}
	return string(b)
}

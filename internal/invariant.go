package cli

func Invariant(expr bool, msg string) {
	if !expr {
		panic(msg)
	}
}

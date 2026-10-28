package e11

// Complexity16 has cognitive complexity 16.
func Complexity16(a, b, c, d int) int {
	n := 0
	for i := 0; i < a; i++ { // +1
		if i > b { // +2 (nesting 1)
			for j := 0; j < c; j++ { // +3 (nesting 2)
				if j > d { // +4 (nesting 3)
					if j > i { // +5 (nesting 4)
						n++
					}
				}
			}
		}
	}
	if a > 0 { // +1
		n++
	}
	return n
}

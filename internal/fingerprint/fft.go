package fingerprint

import "math"

// FFT computes the Cooley-Tukey radix-2 DIT FFT.
// Input length must be a power of 2.
// If it isn't, the caller should zero-pad before calling.
func FFT(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}

	// Iterative Cooley-Tukey — avoids deep recursion stack for large windows
	// and is significantly faster for WindowSize=2048.
	result := make([]complex128, n)
	copy(result, x)

	// Bit-reversal permutation
	bits := 0
	for tmp := n; tmp > 1; tmp >>= 1 {
		bits++
	}
	for i := 0; i < n; i++ {
		j := bitReverse(i, bits)
		if j > i {
			result[i], result[j] = result[j], result[i]
		}
	}

	// Butterfly operations
	for size := 2; size <= n; size <<= 1 {
		halfSize := size / 2
		angleStep := -2 * math.Pi / float64(size)
		for i := 0; i < n; i += size {
			for k := 0; k < halfSize; k++ {
				angle := angleStep * float64(k)
				w := complex(math.Cos(angle), math.Sin(angle))
				u := result[i+k]
				v := w * result[i+k+halfSize]
				result[i+k] = u + v
				result[i+k+halfSize] = u - v
			}
		}
	}

	return result
}

func bitReverse(x, bits int) int {
	result := 0
	for i := 0; i < bits; i++ {
		result = (result << 1) | (x & 1)
		x >>= 1
	}
	return result
}
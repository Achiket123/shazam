package fingerprint

import (
	"math"
)

// FFT computes the Fast Fourier Transform of the input slice.
// The length of x must be a power of 2.
func FFT(x []complex128) []complex128 {
	n := len(x)
	if n == 1 {
		return []complex128{x[0]}
	}

	// Split even and odd
	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = x[2*i]
		odd[i] = x[2*i+1]
	}

	// Recursive FFT
	Feven := FFT(even)
	Fodd := FFT(odd)

	// Combine
	result := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		angle := -2 * math.Pi * float64(k) / float64(n)

		num := complex(math.Cos(angle), math.Sin(angle))
		w := num
		t := w * Fodd[k]
		result[k] = Feven[k] + (t)
		result[k+n/2] = Feven[k] - t
	}
	return result
}

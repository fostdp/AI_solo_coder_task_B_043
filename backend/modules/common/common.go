package common

import "math"

func Round(v float64, digits int) float64 {
	pow := math.Pow(10, float64(digits))
	return math.Round(v*pow) / pow
}

func RoundFloat(v float64, digits int) float64 {
	return Round(v, digits)
}

func SafeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func MeanFloat64(arr []float64) float64 {
	if len(arr) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range arr {
		s += v
	}
	return s / float64(len(arr))
}

func IntToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []rune{}
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]rune{'-'}, digits...)
	}
	return string(digits)
}

func GaussElimination(A [][]float64, b []float64) ([]float64, bool) {
	n := len(A)
	if n == 0 || len(b) != n {
		return nil, false
	}
	aug := make([][]float64, n)
	for i := range A {
		aug[i] = make([]float64, n+1)
		copy(aug[i], A[i])
		aug[i][n] = b[i]
	}
	for col := 0; col < n; col++ {
		maxRow := col
		maxVal := math.Abs(aug[col][col])
		for row := col + 1; row < n; row++ {
			if math.Abs(aug[row][col]) > maxVal {
				maxVal = math.Abs(aug[row][col])
				maxRow = row
			}
		}
		if maxVal < 1e-10 {
			return nil, false
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]
		for row := col + 1; row < n; row++ {
			factor := aug[row][col] / aug[col][col]
			for k := col; k <= n; k++ {
				aug[row][k] -= factor * aug[col][k]
			}
		}
	}
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := aug[i][n]
		for j := i + 1; j < n; j++ {
			sum -= aug[i][j] * x[j]
		}
		if math.Abs(aug[i][i]) < 1e-12 {
			return nil, false
		}
		x[i] = sum / aug[i][i]
	}
	return x, true
}

func GradeFromValue(value, g1Limit, g2Limit, g3Limit float64) string {
	if value <= g1Limit {
		return "一级"
	} else if value <= g2Limit {
		return "二级"
	} else if value <= g3Limit {
		return "三级"
	}
	return "不合格"
}

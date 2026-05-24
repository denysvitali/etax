package pdf417ech

const (
	echColumns   = 13
	echRows      = 35
	echECLevel   = 4
	moduleHeight = 1
)

func calculateNumberOfRows(m, k, c int) int {
	r := ((m + 1 + k) / c) + 1
	if c*r >= (m + 1 + k + c) {
		r--
	}
	return r
}

func calcDimensions(dataWords, eccWords int) (cols, rows int) {
	return echColumns, echRows
}

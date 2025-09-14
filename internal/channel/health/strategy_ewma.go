package health

// 趋势平滑，适合高频场景
type EWMAStrategy struct {
	Alpha float64 // e.g. 0.1
}

func (e *EWMAStrategy) Update(current float64, success bool) float64 {
	var value float64
	if success {
		value = 100
	} else {
		value = 0
	}
	return e.Alpha*value + (1-e.Alpha)*current
}

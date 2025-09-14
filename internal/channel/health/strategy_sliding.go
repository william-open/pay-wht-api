package health

// 滑动窗口策略

type SlidingStrategy struct {
	StepUp   float64
	StepDown float64
}

func (s *SlidingStrategy) Update(current float64, success bool) float64 {
	if success {
		return min(current+s.StepUp, 100)
	}
	return max(current-s.StepDown, 0)
}

func min(a, b float64) float64 {
	if a < b {
		return a
	} else {
		return b
	}
}
func max(a, b float64) float64 {
	if a > b {
		return a
	} else {
		return b
	}
}

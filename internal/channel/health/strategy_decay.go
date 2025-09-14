package health

// 衰减策略 每次失败衰减 5%
type DecayStrategy struct {
	Factor float64 // e.g. 0.95
}

func (d *DecayStrategy) Update(current float64, success bool) float64 {
	if success {
		return current
	}
	updated := current * d.Factor
	if updated < 0 {
		return 0
	}
	return updated
}

package health

type SuccessRateStrategy interface {
	Update(current float64, success bool) float64
}

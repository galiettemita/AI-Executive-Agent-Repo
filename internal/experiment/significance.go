package experiment

import (
	"fmt"
	"math"
)

// SignificanceResult holds the outcome of a Welch's t-test.
type SignificanceResult struct {
	Significant      bool    `json:"significant"`
	PValue           float64 `json:"p_value"`
	TStatistic       float64 `json:"t_statistic"`
	DegreesOfFreedom float64 `json:"degrees_of_freedom"`
	ControlMean      float64 `json:"control_mean"`
	VariantMean      float64 `json:"variant_mean"`
	ControlN         int     `json:"control_n"`
	VariantN         int     `json:"variant_n"`
	Winner           string  `json:"winner"`
}

// WelchTTest performs Welch's unequal variance t-test on two independent samples.
func WelchTTest(control, variant []float64, targetP float64, minSamples int) (*SignificanceResult, error) {
	if len(control) < minSamples || len(variant) < minSamples {
		return nil, fmt.Errorf("welch_t_test: insufficient samples (control=%d, variant=%d, min=%d)",
			len(control), len(variant), minSamples)
	}

	n1, n2 := float64(len(control)), float64(len(variant))
	m1, v1 := MeanVariance(control)
	m2, v2 := MeanVariance(variant)

	se := math.Sqrt(v1/n1 + v2/n2)
	if se == 0 {
		return &SignificanceResult{
			Significant: false, PValue: 1.0, ControlMean: m1, VariantMean: m2,
			ControlN: len(control), VariantN: len(variant), Winner: "no_difference",
		}, nil
	}
	t := (m1 - m2) / se

	// Welch-Satterthwaite degrees of freedom
	numerator := math.Pow(v1/n1+v2/n2, 2)
	denominator := math.Pow(v1/n1, 2)/(n1-1) + math.Pow(v2/n2, 2)/(n2-1)
	df := numerator / denominator

	pValue := approxTTestPValue(math.Abs(t), df)

	winner := "no_difference"
	if pValue < targetP {
		if m2 > m1 {
			winner = "variant"
		} else {
			winner = "control"
		}
	}

	return &SignificanceResult{
		Significant:      pValue < targetP,
		PValue:           pValue,
		TStatistic:       t,
		DegreesOfFreedom: df,
		ControlMean:      m1,
		VariantMean:      m2,
		ControlN:         len(control),
		VariantN:         len(variant),
		Winner:           winner,
	}, nil
}

// MeanVariance computes sample mean and sample variance (Bessel's correction).
func MeanVariance(xs []float64) (mean, variance float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	for _, x := range xs {
		d := x - mean
		variance += d * d
	}
	if len(xs) > 1 {
		variance /= float64(len(xs) - 1)
	}
	return
}

func approxTTestPValue(t, df float64) float64 {
	x := df / (df + t*t)
	a, b := df/2.0, 0.5
	return incompleteBeta(x, a, b)
}

func incompleteBeta(x, a, b float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	if x > (a+1)/(a+b+2) {
		return 1 - incompleteBeta(1-x, b, a)
	}
	lbeta := lgamma(a+b) - lgamma(a) - lgamma(b)
	front := math.Exp(lbeta+a*math.Log(x)+b*math.Log(1-x)) / a

	var cfrac float64 = 1
	for i := 100; i >= 1; i-- {
		fi := float64(i)
		var num float64
		if i%2 == 0 {
			m := fi / 2
			num = m * (b - m) * x / ((a + 2*m - 1) * (a + 2*m))
		} else {
			m := (fi - 1) / 2
			num = -(a + m) * (a + b + m) * x / ((a + 2*m) * (a + 2*m + 1))
		}
		cfrac = 1 + num/cfrac
	}
	return front / cfrac
}

func lgamma(x float64) float64 {
	lg, _ := math.Lgamma(x)
	return lg
}

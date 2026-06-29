package icu

type RebalanceEnvelope struct {
	Low     RebalanceRat `json:"low"`
	Current RebalanceRat `json:"current"`
	High    RebalanceRat `json:"high"`
}

type RebalancePCHIP struct {
	low       RebalanceRat
	current   RebalanceRat
	high      RebalanceRat
	dLow      RebalanceRat
	dCurrent  RebalanceRat
	dHigh     RebalanceRat
	half      RebalanceRat
	boundLow  RebalanceRat
	boundHigh RebalanceRat
}

func NewRebalancePCHIP(envelope RebalanceEnvelope) RebalancePCHIP {
	half := NewRebalanceRatFromInt(1, 2)
	two := NewRebalanceRatFromInt(2, 1)
	zero := ZeroRebalanceRat()

	sLow := envelope.Current.Sub(envelope.Low).Quo(half)
	sHigh := envelope.High.Sub(envelope.Current).Quo(half)

	dLow := sLow
	dHigh := sHigh
	dCurrent := zero
	if sLow.Sign()*sHigh.Sign() > 0 {
		sum := sLow.Add(sHigh)
		if !sum.IsZero() {
			dCurrent = two.Mul(sLow).Mul(sHigh).Quo(sum)
		}
	}

	return RebalancePCHIP{
		low:       envelope.Low.clone(),
		current:   envelope.Current.clone(),
		high:      envelope.High.clone(),
		dLow:      dLow.clone(),
		dCurrent:  dCurrent.clone(),
		dHigh:     dHigh.clone(),
		half:      half,
		boundLow:  NewRebalanceRatFromInt(0, 1),
		boundHigh: NewRebalanceRatFromInt(1, 1),
	}
}

func (c RebalancePCHIP) Evaluate(level RebalanceRat) RebalanceRat {
	if level.Cmp(c.boundLow) <= 0 {
		return c.low.clone()
	}
	if level.Cmp(c.boundHigh) >= 0 {
		return c.high.clone()
	}
	if level.Cmp(c.half) <= 0 {
		t := level.Quo(c.half)
		return hermiteCubic(t, c.half, c.low, c.current, c.dLow, c.dCurrent)
	}
	t := level.Sub(c.half).Quo(c.half)
	return hermiteCubic(t, c.half, c.current, c.high, c.dCurrent, c.dHigh)
}

// hermiteCubic evaluates the cubic Hermite basis on a segment [x0,x1] of width
// h with values y0/y1 at the endpoints and tangents d0/d1. Parameter t is the
// local position in [0,1].
func hermiteCubic(t, width, y0, y1, d0, d1 RebalanceRat) RebalanceRat {
	one := NewRebalanceRatFromInt(1, 1)
	two := NewRebalanceRatFromInt(2, 1)
	three := NewRebalanceRatFromInt(3, 1)

	t2 := t.Mul(t)
	t3 := t2.Mul(t)

	h00 := two.Mul(t3).Sub(three.Mul(t2)).Add(one) // 2t^3 - 3t^2 + 1
	h10 := t3.Sub(two.Mul(t2)).Add(t)              // t^3 - 2t^2 + t
	h01 := three.Mul(t2).Sub(two.Mul(t3))          // -2t^3 + 3t^2
	h11 := t3.Sub(t2)                              // t^3 - t^2

	hd0 := width.Mul(d0)
	hd1 := width.Mul(d1)

	term0 := h00.Mul(y0)
	term1 := h10.Mul(hd0)
	term2 := h01.Mul(y1)
	term3 := h11.Mul(hd1)

	return term0.Add(term1).Add(term2).Add(term3)
}

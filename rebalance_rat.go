package icu

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const rebalanceRatSerialPrecision = 39

// rebalanceRatMaxLiteralLen bounds accepted decimal input length so unbounded
// big.Rat parsing cannot consume untrusted memory (CVE-2022-23772 mitigation).
const rebalanceRatMaxLiteralLen = 192

//nolint:recvcheck // JSON decoding needs a pointer receiver while arithmetic stays usable on exact value temporaries.
type RebalanceRat struct {
	value *big.Rat
}

func ZeroRebalanceRat() RebalanceRat {
	return RebalanceRat{value: new(big.Rat)}
}

func NewRebalanceRatFromInt(num, den int) RebalanceRat {
	return RebalanceRat{value: new(big.Rat).SetFrac64(int64(num), int64(den))}
}

func ParseRebalanceRat(input string) (RebalanceRat, error) {
	trimmed := strings.TrimSpace(input)
	if !validDecimalLiteral(trimmed) {
		return ZeroRebalanceRat(), errors.New("rebalance rat: decimal literal required")
	}
	value, ok := new(big.Rat).SetString(trimmed) //nolint:gosec // G113: input length and decimal form are validated above
	if !ok {
		return ZeroRebalanceRat(), errors.New("rebalance rat: parse failed")
	}
	return RebalanceRat{value: value}, nil
}

func validDecimalLiteral(value string) bool {
	if value == "" || len(value) > rebalanceRatMaxLiteralLen {
		return false
	}
	digits := value
	if digits[0] == '+' || digits[0] == '-' {
		digits = digits[1:]
	}
	if digits == "" {
		return false
	}
	dot := strings.IndexByte(digits, '.')
	if dot >= 0 {
		intPart := digits[:dot]
		fracPart := digits[dot+1:]
		if intPart == "" && fracPart == "" {
			return false
		}
		return allDigits(intPart) && allDigits(fracPart)
	}
	return allDigits(digits)
}

func allDigits(value string) bool {
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func (r RebalanceRat) IsZero() bool {
	return r.value.Sign() == 0
}

func (r RebalanceRat) Sign() int {
	return r.value.Sign()
}

func (r RebalanceRat) Cmp(other RebalanceRat) int {
	return r.value.Cmp(other.value)
}

func (r RebalanceRat) Equal(other RebalanceRat) bool {
	return r.value.Cmp(other.value) == 0
}

func (r RebalanceRat) Add(other RebalanceRat) RebalanceRat {
	return RebalanceRat{value: new(big.Rat).Add(r.value, other.value)}
}

func (r RebalanceRat) Sub(other RebalanceRat) RebalanceRat {
	return RebalanceRat{value: new(big.Rat).Sub(r.value, other.value)}
}

func (r RebalanceRat) Mul(other RebalanceRat) RebalanceRat {
	return RebalanceRat{value: new(big.Rat).Mul(r.value, other.value)}
}

func (r RebalanceRat) Quo(other RebalanceRat) RebalanceRat {
	if other.IsZero() {
		return ZeroRebalanceRat()
	}
	return RebalanceRat{value: new(big.Rat).Quo(r.value, other.value)}
}

func (r RebalanceRat) Float64() float64 {
	value, _ := r.value.Float64()
	return value
}

func (r RebalanceRat) DecimalString() string {
	text := r.value.FloatString(rebalanceRatSerialPrecision)
	return trimTrailingDecimalZeros(text)
}

func (r RebalanceRat) String() string {
	return r.DecimalString()
}

// NewRebalanceRatFromDyadic builds a RebalanceRat from a float64 that is
// exactly representable as a dyadic rational (denominator a power of two, e.g.
// sums/medians of integer weekly loads). It uses the shortest round-tripping
// decimal representation so no intermediate rounding is introduced. Values that
// are not exactly representable as dyadic rationals (e.g. 1/3) are rejected to
// keep rebalance math exact.
func NewRebalanceRatFromDyadic(value float64) (RebalanceRat, bool) {
	text := strconv.FormatFloat(value, 'f', -1, 64)
	rat, ok := new(big.Rat).SetString(text) //nolint:gosec // G113: text is the shortest round-trip of a bounded float64
	if !ok {
		return ZeroRebalanceRat(), false
	}
	denom := rat.Denom()
	if denom.Sign() <= 0 {
		return ZeroRebalanceRat(), false
	}
	one := big.NewInt(1)
	if new(big.Int).And(denom, new(big.Int).Sub(denom, one)).Sign() != 0 {
		return ZeroRebalanceRat(), false
	}
	return RebalanceRat{value: rat}, true
}

func (r RebalanceRat) clone() RebalanceRat {
	return RebalanceRat{value: new(big.Rat).Set(r.value)}
}

func marshalRebalanceRatString(value string) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal rebalance rat: %w", err)
	}

	return data, nil
}

// MarshalJSON serializes a RebalanceRat as its exact shortest decimal literal
// so proposals survive file round-trips without precision loss.
func (r RebalanceRat) MarshalJSON() ([]byte, error) {
	if r.value == nil {
		return marshalRebalanceRatString("0")
	}
	return marshalRebalanceRatString(r.DecimalString())
}

// UnmarshalJSON restores a RebalanceRat from a decimal literal produced by
// MarshalJSON (or any valid decimal input).
func (r *RebalanceRat) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("unmarshal rebalance rat: %w", err)
	}
	value, err := ParseRebalanceRat(text)
	if err != nil {
		return err
	}
	*r = value
	return nil
}

func (r RebalanceRat) raw() *big.Rat {
	return r.value
}

func trimTrailingDecimalZeros(value string) string {
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		end := len(value)
		for end > dot+1 && value[end-1] == '0' {
			end--
		}
		if value[end-1] == '.' {
			end--
		}
		return value[:end]
	}
	return value
}

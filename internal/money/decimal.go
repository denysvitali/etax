package money

import (
	"encoding/xml"
	"fmt"
	"math/big"
	"strings"
)

type Decimal struct {
	r *big.Rat
}

func Zero() Decimal {
	return Decimal{r: new(big.Rat)}
}

func One() Decimal {
	return Decimal{r: big.NewRat(1, 1)}
}

func FromInt(v int64) Decimal {
	return Decimal{r: big.NewRat(v, 1)}
}

func FromString(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Zero(), nil
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return Decimal{}, fmt.Errorf("invalid decimal %q", s)
	}
	return Decimal{r: r}, nil
}

func Must(s string) Decimal {
	d, err := FromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Decimal) rat() *big.Rat {
	if d.r == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Set(d.r)
}

func (d Decimal) Add(o Decimal) Decimal {
	return Decimal{r: new(big.Rat).Add(d.rat(), o.rat())}
}

func (d Decimal) Sub(o Decimal) Decimal {
	return Decimal{r: new(big.Rat).Sub(d.rat(), o.rat())}
}

func (d Decimal) Mul(o Decimal) Decimal {
	return Decimal{r: new(big.Rat).Mul(d.rat(), o.rat())}
}

func (d Decimal) Neg() Decimal {
	return Decimal{r: new(big.Rat).Neg(d.rat())}
}

func (d Decimal) Abs() Decimal {
	r := d.rat()
	if r.Sign() < 0 {
		r.Neg(r)
	}
	return Decimal{r: r}
}

func (d Decimal) IsZero() bool {
	return d.rat().Sign() == 0
}

func (d Decimal) Sign() int {
	return d.rat().Sign()
}

func (d Decimal) String() string {
	if d.r == nil {
		return "0.00"
	}
	return trimFixed(d.r.FloatString(6))
}

func (d Decimal) Fixed(scale int) string {
	return d.rat().FloatString(scale)
}

func (d Decimal) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Decimal) UnmarshalText(text []byte) error {
	parsed, err := FromString(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Decimal) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{Name: name, Value: d.String()}, nil
}

func trimFixed(s string) string {
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

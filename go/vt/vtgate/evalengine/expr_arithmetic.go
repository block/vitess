/*
Copyright 2023 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package evalengine

import (
	"strconv"

	"vitess.io/vitess/go/mysql/decimal"
	"vitess.io/vitess/go/sqltypes"
)

type (
	ArithmeticExpr struct {
		BinaryExpr
		Op opArith
	}

	NegateExpr struct {
		UnaryExpr
	}

	opArith interface {
		eval(left, right eval) (eval, error)
		compile(c *compiler, left, right IR) (ctype, error)
		String() string
	}

	opArithAdd    struct{}
	opArithSub    struct{}
	opArithMul    struct{}
	opArithDiv    struct{}
	opArithIntDiv struct{}
	opArithMod    struct{}
)

var _ IR = (*ArithmeticExpr)(nil)

var (
	_ opArith = (*opArithAdd)(nil)
	_ opArith = (*opArithSub)(nil)
	_ opArith = (*opArithMul)(nil)
	_ opArith = (*opArithDiv)(nil)
	_ opArith = (*opArithIntDiv)(nil)
	_ opArith = (*opArithMod)(nil)
)

func (b *ArithmeticExpr) eval(env *ExpressionEnv) (eval, error) {
	left, err := b.Left.eval(env)
	if left == nil || err != nil {
		return nil, err
	}

	right, err := b.Right.eval(env)
	if right == nil || err != nil {
		return nil, err
	}
	return b.Op.eval(left, right)
}

func (b *ArithmeticExpr) compile(c *compiler) (ctype, error) {
	return b.Op.compile(c, b.Left, b.Right)
}

func (op *opArithAdd) eval(left, right eval) (eval, error) {
	return addNumericWithError(left, right)
}
func (op *opArithAdd) String() string { return "+" }

func (op *opArithAdd) compile(c *compiler, left, right IR) (ctype, error) {
	lt, err := left.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip1 := c.compileNullCheck1(lt)

	rt, err := right.compile(c)
	if err != nil {
		return ctype{}, err
	}

	swap := false
	skip2 := c.compileNullCheck1r(rt)

	lt = c.compileToNumeric(lt, 2, sqltypes.Float64, true)
	rt = c.compileToNumeric(rt, 1, sqltypes.Float64, true)
	// The operands' types before compileNumericPriority swaps them, so each
	// stays with the expression it was compiled from.
	lt0, rt0 := lt, rt
	lt, rt, swap = c.compileNumericPriority(lt, rt)

	ct := ctype{Flag: nullableFlags(lt.Flag | rt.Flag), Col: collationNumeric}

	switch lt.Type {
	case sqltypes.Int64:
		c.asm.Add_ii()
		ct.Type = sqltypes.Int64
	case sqltypes.Uint64:
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.Add_ui(swap)
		case sqltypes.Uint64:
			c.asm.Add_uu()
		}
		ct.Type = sqltypes.Uint64
	case sqltypes.Decimal:
		if swap {
			c.compileToDecimal(rt, 2)
		} else {
			c.compileToDecimal(rt, 1)
		}
		c.asm.Add_dd()
		ct.Type = sqltypes.Decimal
		ct.Size, ct.Scale = additiveDecimalType(decimalOperand(left, lt0), decimalOperand(right, rt0))
	case sqltypes.Float64:
		if swap {
			c.compileToFloat(rt, 2)
		} else {
			c.compileToFloat(rt, 1)
		}
		c.asm.Add_ff()
		ct.Type = sqltypes.Float64
	}

	c.asm.jumpDestination(skip1, skip2)
	return ct, nil
}

func (op *opArithSub) eval(left, right eval) (eval, error) {
	return subtractNumericWithError(left, right)
}
func (op *opArithSub) String() string { return "-" }

func (op *opArithSub) compile(c *compiler, left, right IR) (ctype, error) {
	lt, err := left.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip1 := c.compileNullCheck1(lt)

	rt, err := right.compile(c)
	if err != nil {
		return ctype{}, err
	}

	skip2 := c.compileNullCheck1r(rt)
	lt = c.compileToNumeric(lt, 2, sqltypes.Float64, true)
	rt = c.compileToNumeric(rt, 1, sqltypes.Float64, true)

	ct := ctype{Flag: nullableFlags(lt.Flag | rt.Flag), Col: collationNumeric}
	switch lt.Type {
	case sqltypes.Int64:
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.Sub_ii()
			ct.Type = sqltypes.Int64
		case sqltypes.Uint64:
			c.asm.Sub_iu()
			ct.Type = sqltypes.Uint64
		case sqltypes.Float64:
			c.compileToFloat(lt, 2)
			c.asm.Sub_ff()
			ct.Type = sqltypes.Float64
		case sqltypes.Decimal:
			c.compileToDecimal(lt, 2)
			c.asm.Sub_dd()
			ct.Type = sqltypes.Decimal
			ct.Size, ct.Scale = additiveDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
		}
	case sqltypes.Uint64:
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.Sub_ui()
			ct.Type = sqltypes.Uint64
		case sqltypes.Uint64:
			c.asm.Sub_uu()
			ct.Type = sqltypes.Uint64
		case sqltypes.Float64:
			c.compileToFloat(lt, 2)
			c.asm.Sub_ff()
			ct.Type = sqltypes.Float64
		case sqltypes.Decimal:
			c.compileToDecimal(lt, 2)
			c.asm.Sub_dd()
			ct.Type = sqltypes.Decimal
			ct.Size, ct.Scale = additiveDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
		}
	case sqltypes.Float64:
		c.compileToFloat(rt, 1)
		c.asm.Sub_ff()
		ct.Type = sqltypes.Float64
	case sqltypes.Decimal:
		switch rt.Type {
		case sqltypes.Float64:
			c.compileToFloat(lt, 2)
			c.asm.Sub_ff()
			ct.Type = sqltypes.Float64
		default:
			c.compileToDecimal(rt, 1)
			c.asm.Sub_dd()
			ct.Type = sqltypes.Decimal
			ct.Size, ct.Scale = additiveDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
		}
	}

	if ct.Type == 0 {
		panic("did not compile?")
	}

	c.asm.jumpDestination(skip1, skip2)
	return ct, nil
}

func (op *opArithMul) eval(left, right eval) (eval, error) {
	return multiplyNumericWithError(left, right)
}

func (op *opArithMul) String() string { return "*" }

func (op *opArithMul) compile(c *compiler, left, right IR) (ctype, error) {
	lt, err := left.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip1 := c.compileNullCheck1(lt)

	rt, err := right.compile(c)
	if err != nil {
		return ctype{}, err
	}

	swap := false
	skip2 := c.compileNullCheck1r(rt)
	lt = c.compileToNumeric(lt, 2, sqltypes.Float64, true)
	rt = c.compileToNumeric(rt, 1, sqltypes.Float64, true)
	// The operands' types before compileNumericPriority swaps them, so each
	// stays with the expression it was compiled from.
	lt0, rt0 := lt, rt
	lt, rt, swap = c.compileNumericPriority(lt, rt)

	ct := ctype{Flag: nullableFlags(lt.Flag | rt.Flag), Col: collationNumeric}
	switch lt.Type {
	case sqltypes.Int64:
		c.asm.Mul_ii()
		ct.Type = sqltypes.Int64
	case sqltypes.Uint64:
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.Mul_ui(swap)
		case sqltypes.Uint64:
			c.asm.Mul_uu()
		}
		ct.Type = sqltypes.Uint64
	case sqltypes.Float64:
		if swap {
			c.compileToFloat(rt, 2)
		} else {
			c.compileToFloat(rt, 1)
		}
		c.asm.Mul_ff()
		ct.Type = sqltypes.Float64
	case sqltypes.Decimal:
		if swap {
			c.compileToDecimal(rt, 2)
		} else {
			c.compileToDecimal(rt, 1)
		}
		c.asm.Mul_dd()
		ct.Type = sqltypes.Decimal
		ct.Size, ct.Scale = multiplyDecimalType(decimalOperand(left, lt0), decimalOperand(right, rt0))
	}

	c.asm.jumpDestination(skip1, skip2)
	return ct, nil
}

func (op *opArithDiv) eval(left, right eval) (eval, error) {
	return divideNumericWithError(left, right, true)
}

func (op *opArithDiv) String() string { return "/" }

func (op *opArithDiv) compile(c *compiler, left, right IR) (ctype, error) {
	lt, err := left.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip1 := c.compileNullCheck1(lt)

	rt, err := right.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip2 := c.compileNullCheck1r(rt)

	lt = c.compileToNumeric(lt, 2, sqltypes.Float64, true)
	rt = c.compileToNumeric(rt, 1, sqltypes.Float64, true)

	ct := ctype{Col: collationNumeric, Flag: flagNullable}
	if lt.Type == sqltypes.Float64 || rt.Type == sqltypes.Float64 {
		ct.Type = sqltypes.Float64
		c.compileToFloat(lt, 2)
		c.compileToFloat(rt, 1)
		c.asm.Div_ff()
	} else {
		ct.Type = sqltypes.Decimal
		c.compileToDecimal(lt, 2)
		c.compileToDecimal(rt, 1)
		c.asm.Div_dd()
		ct.Size, ct.Scale = divideDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
	}
	c.asm.jumpDestination(skip1, skip2)
	return ct, nil
}

func (op *opArithIntDiv) eval(left, right eval) (eval, error) {
	return integerDivideNumericWithError(left, right)
}

func (op *opArithIntDiv) String() string { return "DIV" }

func (op *opArithIntDiv) compile(c *compiler, left, right IR) (ctype, error) {
	lt, err := left.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip1 := c.compileNullCheck1(lt)

	rt, err := right.compile(c)
	if err != nil {
		return ctype{}, err
	}

	skip2 := c.compileNullCheck1r(rt)
	lt = c.compileToNumeric(lt, 2, sqltypes.Decimal, true)
	rt = c.compileToNumeric(rt, 1, sqltypes.Decimal, true)

	ct := ctype{Type: sqltypes.Int64, Col: collationNumeric, Flag: flagNullable}
	switch lt.Type {
	case sqltypes.Int64:
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.IntDiv_ii()
		case sqltypes.Uint64:
			ct.Type = sqltypes.Uint64
			c.asm.IntDiv_iu()
		case sqltypes.Float64:
			c.asm.Convert_xd(2, 0, 0)
			c.asm.Convert_xd(1, 0, 0)
			c.asm.IntDiv_di()
		case sqltypes.Decimal:
			c.asm.Convert_xd(2, 0, 0)
			c.asm.IntDiv_di()
		}
	case sqltypes.Uint64:
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.IntDiv_ui()
		case sqltypes.Uint64:
			ct.Type = sqltypes.Uint64
			c.asm.IntDiv_uu()
		case sqltypes.Float64:
			c.asm.Convert_xd(2, 0, 0)
			c.asm.Convert_xd(1, 0, 0)
			c.asm.IntDiv_du()
		case sqltypes.Decimal:
			c.asm.Convert_xd(2, 0, 0)
			c.asm.IntDiv_du()
		}
	case sqltypes.Float64:
		switch rt.Type {
		case sqltypes.Decimal:
			c.asm.Convert_xd(2, 0, 0)
			c.asm.IntDiv_di()
		case sqltypes.Uint64:
			ct.Type = sqltypes.Uint64
			c.asm.Convert_xd(2, 0, 0)
			c.asm.Convert_xd(1, 0, 0)
			c.asm.IntDiv_du()
		default:
			c.asm.Convert_xd(2, 0, 0)
			c.asm.Convert_xd(1, 0, 0)
			c.asm.IntDiv_di()
		}
	case sqltypes.Decimal:
		switch rt.Type {
		case sqltypes.Decimal:
			c.asm.IntDiv_di()
		case sqltypes.Uint64:
			ct.Type = sqltypes.Uint64
			c.asm.Convert_xd(1, 0, 0)
			c.asm.IntDiv_du()
		default:
			c.asm.Convert_xd(1, 0, 0)
			c.asm.IntDiv_di()
		}
	}
	c.asm.jumpDestination(skip1, skip2)
	return ct, nil
}

func (op *opArithMod) eval(left, right eval) (eval, error) {
	return modNumericWithError(left, right, true)
}

func (op *opArithMod) String() string { return "DIV" }

func (op *opArithMod) compile(c *compiler, left, right IR) (ctype, error) {
	lt, err := left.compile(c)
	if err != nil {
		return ctype{}, err
	}
	skip1 := c.compileNullCheck1(lt)

	rt, err := right.compile(c)
	if err != nil {
		return ctype{}, err
	}

	skip2 := c.compileNullCheck1r(rt)
	lt = c.compileToNumeric(lt, 2, sqltypes.Float64, true)
	rt = c.compileToNumeric(rt, 1, sqltypes.Float64, true)

	ct := ctype{Col: collationNumeric, Flag: flagNullable}
	switch lt.Type {
	case sqltypes.Int64:
		ct.Type = sqltypes.Int64
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.Mod_ii()
		case sqltypes.Uint64:
			c.asm.Mod_iu()
		case sqltypes.Float64:
			ct.Type = sqltypes.Float64
			c.asm.Convert_xf(2)
			c.asm.Mod_ff()
		case sqltypes.Decimal:
			ct.Type = sqltypes.Decimal
			ct.Size, ct.Scale = moduloDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
			c.asm.Convert_xd(2, 0, 0)
			c.asm.Mod_dd()
		}
	case sqltypes.Uint64:
		ct.Type = sqltypes.Uint64
		switch rt.Type {
		case sqltypes.Int64:
			c.asm.Mod_ui()
		case sqltypes.Uint64:
			c.asm.Mod_uu()
		case sqltypes.Float64:
			ct.Type = sqltypes.Float64
			c.asm.Convert_xf(2)
			c.asm.Mod_ff()
		case sqltypes.Decimal:
			ct.Type = sqltypes.Decimal
			ct.Size, ct.Scale = moduloDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
			c.asm.Convert_xd(2, 0, 0)
			c.asm.Mod_dd()
		}
	case sqltypes.Decimal:
		ct.Type = sqltypes.Decimal
		switch rt.Type {
		case sqltypes.Float64:
			ct.Type = sqltypes.Float64
			c.asm.Convert_xf(2)
			c.asm.Mod_ff()
		default:
			ct.Size, ct.Scale = moduloDecimalType(decimalOperand(left, lt), decimalOperand(right, rt))
			c.asm.Convert_xd(1, 0, 0)
			c.asm.Mod_dd()
		}
	case sqltypes.Float64:
		ct.Type = sqltypes.Float64
		c.asm.Convert_xf(1)
		c.asm.Mod_ff()
	}

	c.asm.jumpDestination(skip1, skip2)
	return ct, nil
}

func (n *NegateExpr) eval(env *ExpressionEnv) (eval, error) {
	e, err := n.Inner.eval(env)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	return evalToNumeric(e, false).negate(), nil
}

func (expr *NegateExpr) compile(c *compiler) (ctype, error) {
	arg, err := expr.Inner.compile(c)
	if err != nil {
		return ctype{}, err
	}

	skip := c.compileNullCheck1(arg)
	arg = c.compileToNumeric(arg, 1, sqltypes.Float64, false)
	var neg sqltypes.Type

	switch arg.Type {
	case sqltypes.Int64:
		if arg.Flag&flagBit != 0 {
			neg = sqltypes.Float64
			c.asm.Neg_bit()
		} else {
			neg = sqltypes.Int64
			c.asm.Neg_i()
		}
	case sqltypes.Uint64:
		if arg.Flag&flagHex != 0 {
			neg = sqltypes.Float64
			c.asm.Neg_hex()
		} else {
			neg = sqltypes.Int64
			c.asm.Neg_u()
		}
	case sqltypes.Float64:
		neg = sqltypes.Float64
		c.asm.Neg_f()
	case sqltypes.Decimal:
		neg = sqltypes.Decimal
		c.asm.Neg_d()
	default:
		panic("unexpected Numeric type")
	}

	c.asm.jumpDestination(skip)
	return ctype{
		Type:  neg,
		Flag:  nullableFlags(arg.Flag),
		Size:  arg.Size,
		Scale: arg.Scale,
		Col:   collationNumeric,
	}, nil
}

// decimalType is the precision and scale of an operand or result of DECIMAL
// arithmetic. An unknown precision is 0, as it is in a ctype's Size, and makes
// the result's precision unknown too: a result that is narrower than its
// values would clamp them wherever it is coerced to its type.
type decimalType struct {
	precision, scale int32
}

// Integer digits MySQL counts for an integer of unknown width: an integer
// expression is a BIGINT 21 characters wide, which is 20 digits signed and
// 21 unsigned (COUNT(*) * 1.5 is DECIMAL(22,1)).
const (
	unknownSignedIntDigits   = 20
	unknownUnsignedIntDigits = 21
)

// decimalOperand is the DECIMAL type MySQL gives an operand of arithmetic
// with a DECIMAL result (Item::decimal_precision): a DECIMAL's precision,
// capped at the largest MySQL supports, and its scale; or the number of
// digits an integer has, with a scale of 0.
func decimalOperand(expr IR, ct ctype) decimalType {
	switch ct.Type {
	case sqltypes.Decimal:
		return decimalType{precision: min(ct.Size, decimal.MyMaxPrecision), scale: ct.Scale}
	case sqltypes.Int64, sqltypes.Uint64:
		return decimalType{precision: integerDigits(expr, ct)}
	}
	return decimalType{scale: ct.Scale}
}

// integerDigits is the number of digits MySQL counts for an integer operand:
// a literal's own digits, the digits a column of the integer's display width
// holds (the width less a sign, when the column is signed), or as many as an
// integer of unknown width can have.
func integerDigits(expr IR, ct ctype) int32 {
	switch expr := expr.(type) {
	case *Literal:
		switch v := expr.inner.(type) {
		case *evalInt64:
			u := uint64(v.i)
			if v.i < 0 {
				u = -u
			}
			return int32(len(strconv.FormatUint(u, 10)))
		case *evalUint64:
			return int32(len(strconv.FormatUint(v.u, 10)))
		}
	case *NegateExpr:
		return integerDigits(expr.Inner, ct)
	}
	signed := ct.Type == sqltypes.Int64
	switch {
	case ct.Size <= 0 && signed:
		return unknownSignedIntDigits
	case ct.Size <= 0:
		return unknownUnsignedIntDigits
	case signed:
		return max(ct.Size-1, 0)
	default:
		return ct.Size
	}
}

func (t decimalType) intDigits() int32 {
	return t.precision - t.scale
}

// additiveDecimalType is the type of l + r or l - r, as MySQL's
// Item_func_additive_op types it: room for the wider integer part and the
// wider scale, plus a digit for the carry. MySQL does not cap this precision
// (two DECIMAL(65,30) add up to a DECIMAL(66,30)).
func additiveDecimalType(l, r decimalType) (size, scale int32) {
	scale = max(l.scale, r.scale)
	if l.precision == 0 || r.precision == 0 {
		return 0, scale
	}
	return max(l.intDigits(), r.intDigits()) + scale + 1, scale
}

// multiplyDecimalType is the type of l * r, as MySQL's Item_func_mul types
// it: the sum of the precisions and of the scales, capped at the largest
// MySQL supports.
func multiplyDecimalType(l, r decimalType) (size, scale int32) {
	scale = min(l.scale+r.scale, decimal.MyMaxScale)
	if l.precision == 0 || r.precision == 0 {
		return 0, scale
	}
	return min(l.precision+r.precision, decimal.MyMaxPrecision), scale
}

// divideDecimalType is the type of l / r, as MySQL's Item_func_div types it:
// the dividend's precision and scale, each widened by div_precision_increment,
// with room for the divisor's scale (dividing by 0.01 multiplies by 100).
// Only the divisor's scale counts, so its precision may be unknown.
func divideDecimalType(l, r decimalType) (size, scale int32) {
	scale = min(l.scale+divPrecisionIncrement, decimal.MyMaxScale)
	if l.precision == 0 {
		return 0, scale
	}
	return min(l.precision+r.scale+divPrecisionIncrement, decimal.MyMaxPrecision), scale
}

// moduloDecimalType is the type of l % r, as MySQL's Item_func_mod types it:
// the larger precision and the larger scale, since the remainder is never
// larger than either operand.
func moduloDecimalType(l, r decimalType) (size, scale int32) {
	scale = max(l.scale, r.scale)
	if l.precision == 0 || r.precision == 0 {
		return 0, scale
	}
	return max(l.precision, r.precision), scale
}

func nullableFlags(flag typeFlag) typeFlag {
	return flag & (flagNull | flagNullable)
}

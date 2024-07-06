package micrograd

import "fmt"

type Op string

const (
	OpAdd Op = "+"
	OpSub Op = "-"
	OpMul Op = "*"
	OpDiv Op = "/"
	OpPow Op = "^"
)

type Value struct {
	data float64
	grad float64

	_op    Op
	_prev  []*Value
	_label string
}

func (v *Value) String() string {
	return fmt.Sprintf("Value{data=%v}", v.data)
}

type Option func(*Value)

func WithChildren(children ...*Value) Option {
	return func(v *Value) {
		v._prev = children
	}
}

func WithOp(op Op) Option {
	return func(v *Value) {
		v._op = op
	}
}

func WithLabel(label string) Option {
	return func(v *Value) {
		v._label = label
	}
}

func NewValue(data float64, opts ...Option) *Value {
	v := &Value{data: data}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

func (v *Value) GetValue() float64 {
	return v.data
}

func (v *Value) Add(v2 *Value, opts ...Option) *Value {
	o := append(opts, WithChildren(v, v2), WithOp(OpAdd))
	return NewValue(v.data+v2.data, o...)
}

func (v *Value) Sub(v2 *Value, opts ...Option) *Value {
	o := append(opts, WithChildren(v, v2), WithOp(OpSub))
	return NewValue(v.data-v2.data, o...)
	//return NewValue(v.data-v2.data, WithChildren(v, v2), WithOp(OpSub))
}

func (v *Value) Mul(v2 *Value, opts ...Option) *Value {
	o := append(opts, WithChildren(v, v2), WithOp(OpMul))
	return NewValue(v.data*v2.data, o...)
	//return NewValue(v.data*v2.data, WithChildren(v, v2), WithOp(OpMul))
}

func (v *Value) Div(v2 *Value, opts ...Option) *Value {
	o := append(opts, WithChildren(v, v2), WithOp(OpDiv))
	return NewValue(v.data/v2.data, o...)
	//return NewValue(v.data/v2.data, WithChildren(v, v2), WithOp(OpMul))
}

// backpropagation:
func (v *Value) Backward() {
}

package micrograd

import "fmt"

func ExampleDrawDot() {

	a := NewValue(2.0, WithLabel("a"))
	b := NewValue(-3.0, WithLabel("b"))
	c := NewValue(10.0, WithLabel("c"))
	e := a.Mul(b, WithLabel("e"))
	d := e.Add(c, WithLabel("d"))
	f := NewValue(-2.0, WithLabel("f"))
	L := d.Mul(f, WithLabel("L"))
	dot := DrawDot(L)
	fmt.Println(dot)
	// Output:
	// digraph {
	// 	0 [label="Value{data=2}"]
	// 	1 [label="Value{data=3}"]
	// 	2 [label="Value{data=5}"]
	// 	0 -> 2 [label="+"]
	// 	1 -> 2 [label="+"]
	// }
}

func xExample() {
	/*
			python version:

		a = Value(-4.0)
		b = Value(2.0)
		c = a + b
		d = a * b + b**3
		c += c + 1
		c += 1 + c + (-a)
		d += d * 2 + (b + a).relu()
		d += 3 * d + (b - a).relu()
		e = c - d
		f = e**2
		g = f / 2.0
		g += 10.0 / f
		print(f'{g.data:.4f}') # prints 24.7041, the outcome of this forward pass
		g.backward()
		print(f'{a.grad:.4f}') # prints 138.8338, i.e. the numerical value of dg/da
		print(f'{b.grad:.4f}') # prints 645.5773, i.e. the numerical value of dg/db
	*/

	/*
		a := NewValue(-4.0)
		b := NewValue(2.0)
		fmt.Println(a)
		fmt.Println(b)
		c := a.Add(b)
		d := a.Mul(b).Add(b.Pow(NewValue(3.0)))
		c = c.Add(c).Add(NewValue(1.0)).Add(NewValue(1.0).Add(c).Add(a.Neg()))
		d = d.Add(d.Mul(NewValue(2.0)).Add(b.Add(a).ReLU()))
		d = d.Add(d.Mul(NewValue(3.0)).Add(b.Sub(a).ReLU()))
		e := c.Sub(d)
		f := e.Pow(NewValue(2.0))
		g := f.Div(NewValue(2.0))
		g = g.Add(NewValue(10.0).Div(f))
		//println(g.GetValue()) // prints 24.7041, the outcome of this forward pass
		g.Backward()
		//println(a.grad) // prints 138.8338, i.e. the numerical value of dg/da
		//println(b.grad) // prints 645.5773, i.e. the numerical value of dg/db
		// Output:
		// Value{data=-4}
		// Value{data=2}
	*/
}

// fooOutput:
// 24.7041
// 138.8338
// 645.5773

package webkithost

import "testing"

func TestParsePickerTypes(t *testing.T) {
	types := parsePickerTypes(".png, public.jpeg\napplication/pdf")
	if len(types) != 3 {
		t.Fatalf("len(types) = %d, want 3", len(types))
	}
	for i, typ := range types {
		if typ.ID == 0 {
			t.Fatalf("types[%d] is nil", i)
		}
	}
}

func TestPickerTypeRejectsEmpty(t *testing.T) {
	if typ := pickerType("."); typ.ID != 0 {
		t.Fatalf("empty picker type = %v, want nil", typ)
	}
}

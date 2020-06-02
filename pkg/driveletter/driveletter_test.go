package driveletter

import "testing"

func TestString(t *testing.T) {
	cases := []struct {
		input  uint
		expect string
	}{
		{
			input:  1,
			expect: "a",
		}, {
			input:  2,
			expect: "b",
		}, {
			input:  26,
			expect: "z",
		}, {
			input:  27,
			expect: "aa",
		}, {
			input:  52,
			expect: "az",
		}, {
			input:  53,
			expect: "ba",
		}, {
			input:  100,
			expect: "cv",
		},
	}

	for _, c := range cases {
		var d DriveLetter
		d.num = c.input
		actual := d.String()

		if actual != c.expect {
			t.Errorf("on input '%d':", c.input)
			t.Errorf("unexpected result")
			t.Errorf("expected: %s", c.expect)
			t.Errorf("actual: %s", actual)
		}
	}
}

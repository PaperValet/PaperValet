package builtin

import "testing"

func TestUpdateMode(t *testing.T) {
	cases := []struct {
		args       []string
		force, now bool
	}{
		{nil, false, false},
		{[]string{"now"}, false, true},
		{[]string{"-f"}, true, false},
		{[]string{"--force"}, true, false},
		{[]string{"NOW", "-F"}, true, true},
		{[]string{"later"}, false, false},
	}
	for _, c := range cases {
		f, n := updateMode(c.args)
		if f != c.force || n != c.now {
			t.Errorf("%v: got force=%v now=%v", c.args, f, n)
		}
	}
}

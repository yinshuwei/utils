package utils

import "testing"

var abcd = "abcd"
var faces = "☺☻☹"
var dots = "1.2.3.4"

type CatTest struct {
	param  []string
	result string
}

var carttests = []CatTest{
	{[]string{""}, ""},
	{[]string{"", ""}, ""},
	{[]string{"", "", ""}, ""},
	{[]string{"a", ""}, "a"},
	{[]string{"a", "", ""}, "a"},
	{[]string{"a", "", "", ""}, "a"},
	{[]string{"", "", "", "a", "", "", "", "", "", ""}, "a"},
	{[]string{"", "", "b"}, "b"},
	{[]string{"a", "b"}, "ab"},
	{[]string{"a", "b", "c"}, "abc"},
	{[]string{"a", "b", "c", " ", " ", " "}, "abc   "},
	{[]string{"a", "b", "c", " ", "", " "}, "abc  "},
	{[]string{"a", "b", "c", " ", " ", ""}, "abc  "},
	{[]string{"", "a", "b"}, "ab"},
	{[]string{"a", "b", ""}, "ab"},
	{[]string{"a", "", "b"}, "ab"},

	{[]string{}, ""},
	{[]string{"a", "bcd"}, abcd},
	{[]string{"a", "b", "c", "d"}, abcd},
	{[]string{"a", "b", "c", "d"}, abcd},
	{[]string{"☺", "☻", "☹"}, faces},
	{[]string{"☺", "☻", "☹"}, faces},
	{[]string{"☺", "☻", "☹"}, faces},
	{[]string{"☺", "�", "☹"}, "☺�☹"},
	{nil, ""},
	{[]string{"", "abcd"}, abcd},
	{[]string{"abcd"}, abcd},
	{[]string{"1", "2", "3", "4"}, "1234"},
	{[]string{"1", ".2", ".3", ".4"}, dots},
	{[]string{"☺☻☹", ""}, faces},
	{[]string{faces}, faces},
	{[]string{"1", "2", "3 4"}, "123 4"},
	{[]string{"1", "2"}, "12"},
}

func TestCat(t *testing.T) {
	for _, v := range carttests {
		r := Cat(v.param...)
		if v.result != r {
			t.Errorf("Cat(%#v) = %q, want %q", v.param, r, v.result)

		}
	}
}

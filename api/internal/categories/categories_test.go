package categories

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"прогулки", "прогулки", true},
		{"Прогулки", "прогулки", true},
		{"  СПОРТ  ", "спорт", true},
		{"Напитки/Еда", "напитки/еда", true},
		{"Волонтерство", "волонтерство", true},
		{"романтика/свидания", "романтика/свидания", true},
		{"xyz", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, c := range cases {
		got, ok := Normalize(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("Normalize(%q) = (%q, %v), want (%q, %v)",
				c.in, got, ok, c.want, c.ok)
		}
	}
}

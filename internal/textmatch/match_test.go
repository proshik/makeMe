package textmatch

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"Hello", "hello"},
		{"  hello  ", "hello"},
		{"hello world", "hello world"},
		{"hello    world", "hello world"},
		{"hello!", "hello"},
		{"Hello?", "hello"},
		{"hello.", "hello"},
		{"hello, world", "hello, world"},
		{"ёжик", "ежик"},
		{"Ёлка", "елка"},
		{"   ", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Fatalf("Normalize(%q): got %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		input, expected string
		want            bool
	}{
		// Basic cases
		{"hello", "hello", true},
		{"Hello", "hello", true},
		{"  hello  ", "hello", true},
		{"hello!", "hello", true},
		{"hello", "world", false},
		{"", "hello", false},

		// Russian ё/е fold
		{"ёжик", "ежик", true},
		{"ежик", "ёжик", true},

		// Multi-translation alternatives via ;
		{"house", "house; home", true},
		{"home", "house; home", true},
		{"casa", "house; home", false},
		{"  HOME  ", "house; home", true},

		// Multi-translation alternatives via , (Russian dictionary style)
		{"история", "история, рассказ", true},
		{"рассказ", "история, рассказ", true},
		{"  Рассказ  ", "история, рассказ", true},
		{"повесть", "история, рассказ", false},
		// Whole multi-translation also matches (user typed the full phrase)
		{"история, рассказ", "история, рассказ", true},

		// Mixed delimiters
		{"big", "big, large; huge", true},
		{"large", "big, large; huge", true},
		{"huge", "big, large; huge", true},

		// Whitespace/punctuation tolerance with alternatives
		{"hello, world", "hello world; hi there", false}, // comma differs
		{"hello world", "hello, world; greetings", false},
		{"hi there", "hello world; hi there", true},

		// Case + accent normalization
		{"ПРИВЕТ", "привет", true},
	}
	for _, tc := range cases {
		t.Run(tc.input+"|"+tc.expected, func(t *testing.T) {
			if got := Match(tc.input, tc.expected); got != tc.want {
				t.Fatalf("Match(%q, %q): got %v, want %v", tc.input, tc.expected, got, tc.want)
			}
		})
	}
}

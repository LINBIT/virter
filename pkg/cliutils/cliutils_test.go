package cliutils_test

import (
	"fmt"
	"github.com/LINBIT/virter/pkg/cliutils"
	"reflect"
	"testing"
)

type testStruct struct {
	StrItem       string `arg:"str"`
	IntItem       int    `arg:"int"`
	DefaultString string `arg:"defaultStr,bla"`
	DefaultInt    int    `arg:"defaultInt,42"`
	YesOrNo       myBool `arg:"yes_or_no,yes"`
}

type myBool struct {
	b bool
}

func (m *myBool) UnmarshalText(text []byte) error {
	switch string(text) {
	case "yes":
		m.b = true
	case "no":
		m.b = false
	default:
		return fmt.Errorf("'yes' or 'no': %s", string(text))
	}
	return nil
}

func TestParse(t *testing.T) {
	tcases := []struct {
		name        string
		arg         string
		expected    testStruct
		expectError bool
	}{
		{
			name:        "empty is error",
			arg:         "",
			expectError: true,
		},
		{
			name: "full parse",
			arg:  "str=foo,int=-41,defaultStr=override,defaultInt=100,yes_or_no=no",
			expected: testStruct{
				StrItem:       "foo",
				IntItem:       -41,
				DefaultString: "override",
				DefaultInt:    100,
				YesOrNo:       myBool{false},
			},
		},
		{
			name: "test defaults",
			arg:  "str=foo2,int=-412",
			expected: testStruct{
				StrItem:       "foo2",
				IntItem:       -412,
				DefaultString: "bla",
				DefaultInt:    42,
				YesOrNo:       myBool{true},
			},
		},
		{
			name: "unmarshal error propagates",
			arg: "str=foo2,int=-412,yes_or_no=42",
			expectError: true,
		},
	}

	t.Parallel()
	for i := range tcases {
		c := tcases[i]
		t.Run(c.name, func(t *testing.T) {
			var actual testStruct
			err := cliutils.Parse(c.arg, &actual)
			if err != nil {
				if !c.expectError {
					t.Fatalf("got unexpected error: %v", err)
				}
				return
			}

			if c.expectError {
				t.Fatalf("got no error when one was expected")
			}

			if !reflect.DeepEqual(c.expected, actual) {
				t.Fatalf("objects not equal! expected: %+v, actual: %+v", c.expected, actual)
			}
		})
	}
}

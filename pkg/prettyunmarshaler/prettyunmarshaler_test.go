package prettyunmarshaler

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJsonUnmarshalError(t *testing.T) {
	type testCase struct {
		name               string
		data               []byte
		inErr              error
		expectErrorString  string
		expectPrettyString string
	}
	validData := []byte(`{"messages": ["Hello", "world!"]}`)
	invalidData := []byte(`{"messages": ["Hello", "world!"]`)
	for _, tc := range []testCase{
		{
			name:               "unknown error",
			data:               validData,
			inErr:              errors.New("unknown error"),
			expectErrorString:  "unknown error",
			expectPrettyString: "unknown error",
		},
		{
			name:               "unmarshal type error: no data",
			data:               nil,
			inErr:              &json.UnmarshalTypeError{Value: "foo", Type: reflect.TypeOf(""), Offset: 0},
			expectErrorString:  `json: cannot unmarshal foo into Go value of type string`,
			expectPrettyString: `json: cannot unmarshal foo into Go value of type string`,
		},
		{
			name:               "unmarshal type error: negative offset",
			data:               validData,
			inErr:              &json.UnmarshalTypeError{Value: "foo", Type: reflect.TypeOf(""), Offset: -1},
			expectErrorString:  `json: cannot unmarshal foo into Go value of type string`,
			expectPrettyString: `json: cannot unmarshal foo into Go value of type string`,
		},
		{
			name:               "unmarshal type error: greater than length",
			data:               validData,
			inErr:              &json.UnmarshalTypeError{Value: "foo", Type: reflect.TypeOf(""), Offset: int64(len(validData) + 1)},
			expectErrorString:  `json: cannot unmarshal foo into Go value of type string`,
			expectPrettyString: `json: cannot unmarshal foo into Go value of type string`,
		},
		{
			name:              "unmarshal type error: offset at beginning",
			data:              validData,
			inErr:             &json.UnmarshalTypeError{Value: "foo", Type: reflect.TypeOf(""), Offset: 0},
			expectErrorString: `json: cannot unmarshal foo into Go value of type string`,
			expectPrettyString: `json: cannot unmarshal foo into Go value of type string at offset 0 (indicated by <==)
 <== {
    "messages": [
        "Hello",
        "world!"
    ]
}`,
		},
		{
			name:              "unmarshal type error: offset at 1",
			data:              validData,
			inErr:             &json.UnmarshalTypeError{Value: "foo", Type: reflect.TypeOf(""), Offset: 1},
			expectErrorString: `json: cannot unmarshal foo into Go value of type string`,
			expectPrettyString: `json: cannot unmarshal foo into Go value of type string at offset 1 (indicated by <==)
{ <== 
    "messages": [
        "Hello",
        "world!"
    ]
}`,
		},
		{
			name:              "unmarshal type error: offset at end",
			data:              validData,
			inErr:             &json.UnmarshalTypeError{Value: "foo", Type: reflect.TypeOf(""), Offset: int64(len(validData))},
			expectErrorString: `json: cannot unmarshal foo into Go value of type string`,
			expectPrettyString: fmt.Sprintf(`json: cannot unmarshal foo into Go value of type string at offset %d (indicated by <==)
{
    "messages": [
        "Hello",
        "world!"
    ]
} <==`, len(validData)),
		},
		{
			name:               "syntax error: no data",
			data:               nil,
			inErr:              json.Unmarshal(invalidData, nil),
			expectErrorString:  `unexpected end of JSON input`,
			expectPrettyString: `unexpected end of JSON input`,
		},
		{
			name:               "syntax error: negative offset",
			data:               invalidData,
			inErr:              customOffsetSyntaxError(invalidData, -1),
			expectErrorString:  `unexpected end of JSON input`,
			expectPrettyString: `unexpected end of JSON input`,
		},
		{
			name:               "syntax error: greater than length",
			data:               invalidData,
			inErr:              customOffsetSyntaxError(invalidData, int64(len(invalidData)+1)),
			expectErrorString:  `unexpected end of JSON input`,
			expectPrettyString: `unexpected end of JSON input`,
		},
		{
			name:              "syntax error: offset at beginning",
			data:              invalidData,
			inErr:             customOffsetSyntaxError(invalidData, 0),
			expectErrorString: `unexpected end of JSON input`,
			expectPrettyString: `unexpected end of JSON input at offset 0 (indicated by <==)
 <== {"messages": ["Hello", "world!"]`,
		},
		{
			name:              "syntax error: offset at 1",
			data:              invalidData,
			inErr:             customOffsetSyntaxError(invalidData, 1),
			expectErrorString: `unexpected end of JSON input`,
			expectPrettyString: `unexpected end of JSON input at offset 1 (indicated by <==)
{ <== "messages": ["Hello", "world!"]`,
		},
		{
			name:              "syntax error: offset at end",
			data:              invalidData,
			inErr:             customOffsetSyntaxError(invalidData, int64(len(invalidData))),
			expectErrorString: `unexpected end of JSON input`,
			expectPrettyString: fmt.Sprintf(`unexpected end of JSON input at offset %d (indicated by <==)
{"messages": ["Hello", "world!"] <==`, len(invalidData)),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actualErr := NewJSONUnmarshalError(tc.data, tc.inErr)
			assert.Equal(t, tc.expectErrorString, actualErr.Error())
			assert.Equal(t, tc.expectPrettyString, actualErr.Pretty())
		})
	}
}

// customOffsetSyntaxError returns a json.SyntaxError with the given offset.
// json.SyntaxError does not have a public constructor, so we have to use
// json.Unmarshal to create one and then set the offset manually.
//
// If the data does not cause a syntax error, this function will panic.
func customOffsetSyntaxError(data []byte, offset int64) *json.SyntaxError {
	err := json.Unmarshal(data, nil).(*json.SyntaxError)
	err.Offset = offset
	return err
}

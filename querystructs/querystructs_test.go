package querystructs

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// TestBuildAnnotationMap tests the BuildAnnotationMap function.
func TestBuildAnnotationMap(t *testing.T) {
	type testStruct struct {
		Field1 string `json:"field1" xml:"fieldone"`
		Field2 int    `json:"field2" xml:"fieldtwo"`
		Field3 bool   `xml:"fieldthree"`
	}

	tests := []struct {
		input     any
		tagname   string
		expected  map[string]string
		expectErr error
	}{
		{
			input:   testStruct{},
			tagname: "json",
			expected: map[string]string{
				"Field1": "field1",
				"Field2": "field2",
			},
			expectErr: nil,
		},
		{
			input:   testStruct{},
			tagname: "xml",
			expected: map[string]string{
				"Field1": "fieldone",
				"Field2": "fieldtwo",
				"Field3": "fieldthree",
			},
			expectErr: nil,
		},
		{
			input:   &testStruct{},
			tagname: "json",
			expected: map[string]string{
				"Field1": "field1",
				"Field2": "field2",
			},
			expectErr: nil,
		},
		{
			input:     42, // Not a struct
			tagname:   "json",
			expected:  nil,
			expectErr: errors.New("the input needs to be struct"),
		},
	}

	for _, test := range tests {
		result, err := BuildAnnotationMap(test.input, test.tagname)
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("BuildAnnotationMap(%v, %s): expected %v, got %v", test.input, test.tagname, test.expected, result)
		}
		if err != nil && test.expectErr == nil {
			t.Errorf("BuildAnnotationMap(%v, %s): unexpected error: %v", test.input, test.tagname, err)
		} else if err == nil && test.expectErr != nil {
			t.Errorf("BuildAnnotationMap(%v, %s): expected error, got none", test.input, test.tagname)
		} else if err != nil && test.expectErr != nil && err.Error() != test.expectErr.Error() {
			t.Errorf("BuildAnnotationMap(%v, %s): expected error %v, got %v", test.input, test.tagname, test.expectErr, err)
		}
	}
}

// TestBuildNullableMap tests the buildNullableMap function with various cases.
func TestBuildNullableMap(t *testing.T) {

	type Address struct {
		Street sql.NullString `json:"street"`
		City   sql.NullString `json:"city"`
		Valid  bool           // This should not be considered as a valid nullable field
	}

	// Example struct with only non-nullable fields for testing
	type NonNullableStruct struct {
		Name string
		Age  int
	}

	tests := []struct {
		input    any
		expected map[string]bool
		wantErr  error
	}{
		{
			input: &Address{
				Street: sql.NullString{String: "123 Main St", Valid: true},
				City:   sql.NullString{String: "Anytown", Valid: false},
			},
			expected: map[string]bool{"Street": true, "City": false},
			wantErr:  nil,
		},
		{
			input: Address{
				Street: sql.NullString{String: "123 Main St", Valid: true},
				City:   sql.NullString{String: "Anytown", Valid: false},
			},

			expected: map[string]bool{"Street": true, "City": false},
			wantErr:  nil,
		},
		{
			input:    &NonNullableStruct{Name: "John Doe", Age: 30},
			expected: map[string]bool{},
			wantErr:  nil,
		},
		{
			input:    "not a struct",
			expected: nil,
			wantErr:  errors.New("the input needs to be struct"),
		},
	}

	for _, test := range tests {
		got, err := buildNullableMap(test.input)
		if test.wantErr == nil && err != nil {
			t.Errorf("buildNullableMap(%v): expected err: %v, got err: %v", test.input, test.wantErr, err)
		}
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("buildNullableMap(%v): expected map %v, got %v", test.input, test.expected, got)
			fmt.Println(test.input)
		}
	}
}

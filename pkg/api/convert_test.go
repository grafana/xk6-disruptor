package api

import (
	"reflect"
	"testing"
	"time"
)

func Test_Conversions(t *testing.T) {
	t.Parallel()
	type StructField struct {
		SubfieldInt    int64
		SubfieldString string
	}
	type TypedFields struct {
		FieldString   string
		FieldDuration time.Duration
		FieldInt      int64
		FieldFloat    float64
		FieldStruct   StructField
		FieldMap      map[string]string
		FieldArray    []string
	}

	testCases := []struct {
		description string
		value       interface{}
		target      interface{}
		expected    interface{}
		expectError bool
	}{
		{
			description: "String conversion",
			value:       "string",
			target:      new(string),
			expected:    "string",
			expectError: false,
		},
		{
			description: "Int conversion",
			value:       int64(1),
			target:      new(int64),
			expected:    int64(1),
			expectError: false,
		},
		{
			description: "Float to Int conversion",
			value:       float64(1.0),
			target:      new(int64),
			expected:    int64(1),
			expectError: false,
		},
		{
			description: "Invalid String to Int conversion",
			value:       "1.0",
			target:      new(int64),
			expected:    nil,
			expectError: true,
		},
		{
			description: "Float conversion",
			value:       float64(1.0),
			target:      new(float64),
			expected:    float64(1.0),
			expectError: false,
		},
		{
			description: "Int to Float conversion",
			value:       int64(1),
			target:      new(float64),
			expected:    float64(1.0),
			expectError: false,
		},
		{
			description: "Duration conversion",
			value:       "1s",
			target:      new(time.Duration),
			expected:    time.Second,
			expectError: false,
		},
		{
			description: "Invalid duration conversion (missing time unit)",
			value:       "1",
			target:      new(time.Duration),
			expected:    nil,
			expectError: true,
		},
		{
			description: "Time conversion",
			value:       "2006-01-02T15:04:05Z",
			target:      new(time.Time),
			expected: func() time.Time {
				t, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
				return t
			}(),
			expectError: false,
		},
		{
			description: "Invalid Time conversion (missing zone)",
			value:       "2006-01-02T15:04:05",
			target:      new(time.Time),
			expected:    nil,
			expectError: true,
		},
		{
			description: "Float conversion",
			value:       float64(1.0),
			target:      new(float64),
			expected:    float64(1.0),
			expectError: false,
		},
		{
			description: "Int to Float conversion",
			value:       int64(1.0),
			target:      new(float64),
			expected:    float64(1.0),
			expectError: false,
		},
		{
			description: "string array conversion",
			value:       []interface{}{"string1", "string2"},
			target:      &[]string{},
			expected:    []string{"string1", "string2"},
			expectError: false,
		},
		{
			description: "int array conversion",
			value:       []interface{}{1, 2},
			target:      &[]int64{},
			expected:    []int64{1, 2},
			expectError: false,
		},
		{
			description: "invalid int array conversion (mixed types)",
			value:       []interface{}{1, "2"},
			target:      &[]int64{},
			expected:    nil,
			expectError: true,
		},
		{
			description: "map  conversion",
			value: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			target: &map[string]string{},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expectError: false,
		},
		{
			description: "Struct field conversion",
			value: map[string]interface{}{
				"fieldString":   "string",
				"fieldInt":      int64(1),
				"fieldDuration": "1s",
				"fieldFloat":    float64(1.0),
				"fieldStruct": map[string]interface{}{
					"subfieldInt":    int64(0),
					"subfieldString": "string",
				},
				"fieldMap": map[string]interface{}{
					"key": "value",
				},
				"fieldArray": []interface{}{"string"},
			},
			target: &TypedFields{},
			expected: TypedFields{
				FieldString:   "string",
				FieldInt:      1,
				FieldDuration: time.Second,
				FieldFloat:    1.0,
				FieldStruct: StructField{
					SubfieldInt:    0,
					SubfieldString: "string",
				},
				FieldArray: []string{"string"},
				FieldMap: map[string]string{
					"key": "value",
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			err := Convert(tc.value, tc.target)

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			// expected failure
			if tc.expectError && err != nil {
				return
			}

			// target is a pointer, so we use reflect to get the value of the target
			// from its pointer
			target := reflect.ValueOf(tc.target).Elem().Interface()

			if !reflect.DeepEqual(tc.expected, target) {
				t.Errorf("expected: %v actual: %v", tc.expected, target)
				return
			}
		})
	}
}

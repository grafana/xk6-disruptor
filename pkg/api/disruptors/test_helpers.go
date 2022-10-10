package disruptors

import (
	"sort"
)

const (
	testNamespace = "test-ns"
)

// podDesc describes a pod for a test
type podDesc struct {
	name      string
	namespace string
	labels    map[string]string
}

// compareSortedArrays compares if two arrays of strings has the same elements
func compareStringArrays(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}

	if len(a) == 0 {
		return true
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

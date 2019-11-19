package orgbot

import (
	set "github.com/deckarep/golang-set"
)

// newStringSet converts the specified slice of strings to a set.Set of strings.
func newStringSet(strings []string) set.Set {
	s := set.NewSet()
	for _, v := range strings {
		s.Add(v)
	}
	return s
}

// stringSetToSlice converts the specified set.Set of strings to a slice.
func stringSetToSlice(stringSet set.Set) []string {
	var newArray []string
	for _, s := range stringSet.ToSlice() {
		newArray = append(newArray, s.(string))
	}

	return newArray
}

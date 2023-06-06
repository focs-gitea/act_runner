// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import "reflect"

// AreStrSlicesElemsEqual determines whether elements in two slices are equal (ignoring order).
func AreStrSlicesElemsEqual(s1, s2 []string) bool {
	set1 := make(map[string]struct{})
	set2 := make(map[string]struct{})

	for _, elem := range s1 {
		set1[elem] = struct{}{}
	}

	for _, elem := range s2 {
		set2[elem] = struct{}{}
	}

	if len(set1) != len(set2) {
		return false
	}

	return reflect.DeepEqual(set1, set2)
}

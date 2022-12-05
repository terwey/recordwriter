// SPDX-License-Identifier: MIT
// Copyright (c) 2022 Yorick Terweijden

package util

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func CleanHeader(header []string) []string {
	var buf []string
	uniq := make(map[string]int)

	// to keep compatibility with the previous header creation
	// we need to iterate once over the header and keep track if the name appeared before
	// if so, the second one to appear needs to have __2 appended, third __3 etc
	for _, name := range header {
		// strip all control characters from the column name
		var accepted []rune
		for _, r := range name {
			if !unicode.IsControl(r) {
				accepted = append(accepted, r)
			}
		}
		name = string(accepted)
		if _, ok := uniq[strings.ToLower(name)]; ok {
			uniq[strings.ToLower(name)]++
		} else {
			uniq[strings.ToLower(name)] = 1
		}

		// sqlite doesn't care about what characters appear inside a column name as long as it's quoted
		if uniq[strings.ToLower(name)] > 1 {
			buf = append(buf, strconv.Quote(fmt.Sprintf("%s__%d", name, uniq[strings.ToLower(name)])))
		} else {
			buf = append(buf, strconv.Quote(name))
		}
	}
	return buf
}

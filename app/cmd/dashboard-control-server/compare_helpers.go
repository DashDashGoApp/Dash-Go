package main

import (
	"cmp"
	"fmt"
	"strings"
)

func compareText(left, right any) int { return strings.Compare(fmt.Sprint(left), fmt.Sprint(right)) }
func compareFoldedText(left, right any) int {
	return strings.Compare(strings.ToLower(fmt.Sprint(left)), strings.ToLower(fmt.Sprint(right)))
}
func compareIntsDescending(left, right int) int { return cmp.Compare(right, left) }

package main

import "strings"

var (
	ans = [][]string{}
)

func checkValid(s []string, pos, n, total int) bool {
	for i := 0; i < n; i++ {
		if s[i][pos] == 'Q' {
			return false
		}

		left := pos - n + i
		if left >= 0 && s[i][left] == 'Q' {
			return false
		}

		right := pos + n - i
		if right < total && s[i][right] == 'Q' {
			return false
		}
	}
	return true
}

func fillRow(s []string, row, total int) {
	if row == total {
		dst := make([]string, len(s))
		copy(dst, s)
		ans = append(ans, dst)
		return
	}
	for i := 0; i < total; i++ {
		if checkValid(s, i, row, total) {
			newS := strings.Repeat(".", i) + "Q" + strings.Repeat(".", total-i-1)
			s = append(s, newS)
			fillRow(s, row+1, total)
			s = s[:len(s)-1]
		}
	}
}

func solveNQueens(n int) [][]string {
	ans = ans[:0]
	fillRow([]string{}, 0, n)
	return ans
}

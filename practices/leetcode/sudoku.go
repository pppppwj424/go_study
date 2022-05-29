package main

var (
	nums        = []byte{'1', '2', '3', '4', '5', '6', '7', '8', '9'}
	emptySymbol = byte('.')
)

func solveSudoku(board [][]byte) {
	fillUp(board, 0, 0)
}

func checkRow(board [][]byte, row int, c byte) bool {
	for i := 0; i < 9; i++ {
		if board[row][i] == c {
			return false
		}
	}
	return true
}

func checkCol(board [][]byte, col int, c byte) bool {
	for i := 0; i < 9; i++ {
		if board[i][col] == c {
			return false
		}
	}
	return true
}

func checkBlock(board [][]byte, row, col int, c byte) bool {
	startRow := 3 * (row / 3)
	startCol := 3 * (col / 3)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if board[startRow+i][startCol+j] == c {
				return false
			}
		}
	}
	return true
}

func check(board [][]byte, row, col int, c byte) bool {
	blockCheck := checkBlock(board, row, col, c)
	if !blockCheck {
		return false
	}

	rowCheck := checkRow(board, row, c)
	if !rowCheck {
		return false
	}

	colCheck := checkCol(board, col, c)
	if !colCheck {
		return false
	}
	return true
}

func fillUp(board [][]byte, i, j int) bool {
	if i == 9 {
		return true
	}

	nxtI := i + (j+1)/9
	nxtJ := (j + 1) % 9
	if board[i][j] == emptySymbol {
		for _, num := range nums {
			okay := check(board, i, j, num)
			if okay {
				board[i][j] = num
				done := fillUp(board, nxtI, nxtJ)
				if !done {
					board[i][j] = emptySymbol
				} else {
					return true
				}
			}
		}
	} else {
		done := fillUp(board, nxtI, nxtJ)
		if done {
			return true
		}
	}
	return false
}

// 1: gg -summary -n -g '(?s:.)' test.go
// 2: gg -summary -n aV '(?s:.)' test.go

// 4
// 5

/*
8
9
*/

package main // 12

func unused() { // 14
	_ = `line 15...
...and line16`
} /* 17*/

// 19

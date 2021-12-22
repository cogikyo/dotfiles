package main

import "fmt"

func main() {
	fgs := []string{
		"39",
		"30", "1;30",
		"31", "1;31",
		"32", "1;32",
		"33", "1;33",
		"34", "1;34",
		"35", "1;35",
		"36", "1;36",
		"37", "1;37",
	}
	bgs := []string{
		"49", "40", "100", "47", "41", "42", "43", "44", "45", "46",
	}
	text := " *** "

	fmt.Printf("%4s │ ", "")
	for _, bg := range bgs {
		fmt.Printf(" %3s  ", bg)
	}
	fmt.Println()

	for i := 0; i < 4+1; i++ {
		fmt.Printf("─")
	}
	fmt.Printf("┼")
	for i := 0; i < 1+len(text)*len(bgs)+(len(bgs)-1); i++ {
		fmt.Printf("─")
	}
	fmt.Println()

	for _, fg := range fgs {
		fmt.Printf("%4s │ ", fg)
		for _, bg := range bgs {
			fmt.Printf("\x1b[%sm\x1b[%sm%s\x1b[0m ", bg, fg, text)
		}
		fmt.Println()
	}
}

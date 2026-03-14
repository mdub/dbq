package cmd

import (
	_ "embed"
	"fmt"
)

//go:embed cheatsheet.md
var cheatsheet string

// CheatsheetCmd prints a usage cheatsheet
type CheatsheetCmd struct{}

func (c *CheatsheetCmd) Run() error {
	fmt.Print(cheatsheet)
	return nil
}

package main

import (
	"fmt"
	"sort"
)

func cmdAliases() {
	var keys []string
	for alias := range aliases {
		keys = append(keys, alias)
	}
	sort.Strings(keys)

	for _, alias := range keys {
		fmt.Printf("%-10s → %s\n", alias, aliases[alias])
	}
}

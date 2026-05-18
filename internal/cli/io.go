package cli

import "fmt"

func printHeader(first *bool, repoName, pluginName, kind string, n int) {
	if !*first {
		fmt.Println()
	}
	*first = false
	fmt.Printf("== %s :: %s (%d %ss) ==\n", repoName, pluginName, n, kind)
}

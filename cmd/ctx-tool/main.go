package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cxykevin/alkaid0-ctx-rulesys/rulesys"
)

func main() {
	verbose := false
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "-v" {
		verbose = true
		args = args[1:]
	}

	if len(args) > 0 {
		input := strings.Join(args, " ")
		processAndPrint(input, verbose)
		return
	}

	// 无参数：从 stdin 读到 EOF，整体作为输入处理
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var stdinBuf strings.Builder
	for scanner.Scan() {
		stdinBuf.WriteString(scanner.Text())
		stdinBuf.WriteByte('\n')
	}
	input := strings.TrimRight(stdinBuf.String(), "\n")
	if input != "" {
		processAndPrint(input, verbose)
	}
}

func processAndPrint(input string, verbose bool) {
	segments := rulesys.SplitString(input)

	if verbose {
		for i, seg := range segments {
			fmt.Printf("[%d] %s: %q\n", i, seg.Type, seg.Content)
		}
		return
	}

	colorMap := map[rulesys.SegmentType]string{
		rulesys.SegmentPrompt: "\033[44m",
		rulesys.SegmentCode:   "\033[42m",
		rulesys.SegmentLog:    "\033[41m",
	}
	reset := "\033[0m"

	for _, seg := range segments {
		color, ok := colorMap[seg.Type]
		if !ok {
			color = reset
		}
		fmt.Print(color + seg.Content + reset)
	}
	if !strings.HasSuffix(input, "\n") {
		fmt.Println()
	}
}

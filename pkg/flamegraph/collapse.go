package flamegraph

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// CollapsePerf converts perf script output to folded stack format.
// Input: perf script output with stack traces separated by blank lines.
// Output: "func1;func2;func3 count\n" format.
func CollapsePerf(r io.Reader, w io.Writer) {
	stacks := make(map[string]int)
	scanner := bufio.NewScanner(r)

	var currentStack []string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			// End of a stack trace
			if len(currentStack) > 0 {
				// Reverse the stack (perf outputs leaf-first)
				for i, j := 0, len(currentStack)-1; i < j; i, j = i+1, j-1 {
					currentStack[i], currentStack[j] = currentStack[j], currentStack[i]
				}
				key := strings.Join(currentStack, ";")
				stacks[key]++
				currentStack = nil
			}
			continue
		}

		// Stack frame lines start with whitespace and contain address + function
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "  ") {
			// Extract function name: "	ffffffff810a perf_event_task_tick+0xa ([kernel.kallsyms])"
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				funcName := fields[1]
				// Remove offset like "+0xa"
				if idx := strings.Index(funcName, "+"); idx > 0 {
					funcName = funcName[:idx]
				}
				currentStack = append(currentStack, funcName)
			}
		}
	}

	// Handle last stack if no trailing newline
	if len(currentStack) > 0 {
		for i, j := 0, len(currentStack)-1; i < j; i, j = i+1, j-1 {
			currentStack[i], currentStack[j] = currentStack[j], currentStack[i]
		}
		key := strings.Join(currentStack, ";")
		stacks[key]++
	}

	writeCollapsed(w, stacks)
}

// CollapseDtrace converts dtrace output to folded stack format.
func CollapseDtrace(r io.Reader, w io.Writer) {
	stacks := make(map[string]int)
	scanner := bufio.NewScanner(r)

	var currentStack []string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if len(currentStack) > 0 {
				// Dtrace stacks are already in the right order (root first)
				key := strings.Join(currentStack, ";")
				stacks[key]++
				currentStack = nil
			}
			continue
		}

		// Check if this is a count line (just a number)
		if isCountLine(trimmed) {
			if len(currentStack) > 0 {
				key := strings.Join(currentStack, ";")
				count := 1
				fmt.Sscanf(trimmed, "%d", &count)
				stacks[key] += count
				currentStack = nil
			}
			continue
		}

		// Stack frame - extract function name
		funcName := trimmed
		// Remove module info like `module`function
		if idx := strings.Index(funcName, "`"); idx >= 0 {
			funcName = funcName[idx+1:]
		}
		// Remove offset
		if idx := strings.Index(funcName, "+"); idx > 0 {
			funcName = funcName[:idx]
		}
		currentStack = append(currentStack, funcName)
	}

	writeCollapsed(w, stacks)
}

func isCountLine(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func writeCollapsed(w io.Writer, stacks map[string]int) {
	// Sort for deterministic output
	keys := make([]string, 0, len(stacks))
	for k := range stacks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(w, "%s %d\n", k, stacks[k])
	}
}

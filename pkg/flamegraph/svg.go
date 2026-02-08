package flamegraph

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
)

// SVGOptions configures the flame graph SVG output.
type SVGOptions struct {
	Title       string
	Width       int
	Height      int
	ColorScheme string // "hot", "cold", "mem"
}

// DefaultSVGOptions returns sensible defaults.
func DefaultSVGOptions() SVGOptions {
	return SVGOptions{
		Title:       "Flame Graph",
		Width:       1200,
		ColorScheme: "hot",
	}
}

// frame represents a stack frame in the flame graph tree.
type frame struct {
	name     string
	value    int
	children map[string]*frame
}

func newFrame(name string) *frame {
	return &frame{
		name:     name,
		children: make(map[string]*frame),
	}
}

// GenerateSVG renders collapsed stacks as an SVG flame graph.
func GenerateSVG(collapsed io.Reader, svg io.Writer, opts SVGOptions) error {
	if opts.Width == 0 {
		opts.Width = 1200
	}

	// Parse collapsed stacks into tree
	root := newFrame("root")
	var totalSamples int

	scanner := bufio.NewScanner(collapsed)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		stack := parts[0]
		count := 0
		fmt.Sscanf(parts[1], "%d", &count)
		if count == 0 {
			count = 1
		}
		totalSamples += count

		// Build tree
		frames := strings.Split(stack, ";")
		node := root
		for _, fname := range frames {
			child, ok := node.children[fname]
			if !ok {
				child = newFrame(fname)
				node.children[fname] = child
			}
			child.value += count
			node = child
		}
		root.value += count
	}

	if totalSamples == 0 {
		return fmt.Errorf("no samples found in collapsed stacks")
	}

	// Calculate dimensions
	frameHeight := 16
	fontSize := 12
	maxDepth := getMaxDepth(root, 0)
	chartHeight := (maxDepth + 2) * frameHeight
	headerHeight := 40
	totalHeight := chartHeight + headerHeight + 20

	if opts.Height == 0 {
		opts.Height = totalHeight
	}

	// Write SVG header
	fmt.Fprintf(svg, `<?xml version="1.0" standalone="no"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg1.1.dtd">
<svg version="1.1" width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">
<style>
  .func:hover { stroke:black; stroke-width:0.5; cursor:pointer; }
  text { font-family: monospace; font-size: %dpx; }
</style>
<rect x="0" y="0" width="%d" height="%d" fill="white"/>
<text x="%d" y="20" text-anchor="middle" style="font-size:16px; font-weight:bold;">%s</text>
<text x="%d" y="35" text-anchor="middle" style="font-size:12px; fill:#666;">(%d samples)</text>
`,
		opts.Width, opts.Height, fontSize,
		opts.Width, opts.Height,
		opts.Width/2, html.EscapeString(opts.Title),
		opts.Width/2, totalSamples)

	// Render frames bottom-up
	margin := 10
	chartWidth := opts.Width - 2*margin
	baseY := opts.Height - 20
	renderFrame(svg, root, margin, baseY, chartWidth, frameHeight, fontSize, totalSamples, 0, opts.ColorScheme)

	fmt.Fprintln(svg, "</svg>")
	return nil
}

func renderFrame(w io.Writer, f *frame, x, baseY, width, frameHeight, fontSize, totalSamples, depth int, scheme string) {
	if width < 1 || f.value == 0 {
		return
	}

	y := baseY - (depth * frameHeight)

	// Get color
	r, g, b := frameColor(depth, scheme)

	// Draw rectangle
	fmt.Fprintf(w, `<g class="func">
<rect x="%d" y="%d" width="%d" height="%d" fill="rgb(%d,%d,%d)" rx="1"/>
`, x, y-frameHeight, width, frameHeight-1, r, g, b)

	// Add text if frame is wide enough
	if width > 40 {
		label := f.name
		maxChars := (width - 4) / 7 // approximate char width
		if len(label) > maxChars {
			if maxChars > 3 {
				label = label[:maxChars-2] + ".."
			} else {
				label = ""
			}
		}
		if label != "" {
			fmt.Fprintf(w, `<text x="%d" y="%d" fill="black">%s</text>
`, x+2, y-4, html.EscapeString(label))
		}
	}

	pctStr := fmt.Sprintf("%.1f%%", float64(f.value)/float64(totalSamples)*100)
	fmt.Fprintf(w, `<title>%s (%d samples, %s)</title>
</g>
`, html.EscapeString(f.name), f.value, pctStr)

	// Sort children for deterministic output
	childNames := make([]string, 0, len(f.children))
	for name := range f.children {
		childNames = append(childNames, name)
	}
	sort.Strings(childNames)

	// Render children
	childX := x
	for _, name := range childNames {
		child := f.children[name]
		childWidth := int(float64(width) * float64(child.value) / float64(f.value))
		if childWidth < 1 {
			childWidth = 1
		}
		renderFrame(w, child, childX, baseY, childWidth, frameHeight, fontSize, totalSamples, depth+1, scheme)
		childX += childWidth
	}
}

func frameColor(depth int, scheme string) (int, int, int) {
	// Deterministic color based on depth
	switch scheme {
	case "cold":
		g := 50 + (depth*30)%150
		b := 150 + (depth*20)%100
		return 30, g, b
	case "mem":
		g := 190 + (depth*15)%60
		return 30, g, 30
	default: // "hot"
		r := 200 + (depth*15)%55
		g := 50 + (depth*40)%150
		return r, g, 30
	}
}

func getMaxDepth(f *frame, depth int) int {
	max := depth
	for _, child := range f.children {
		d := getMaxDepth(child, depth+1)
		if d > max {
			max = d
		}
	}
	return max
}


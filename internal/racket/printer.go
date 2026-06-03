package racket

import (
	"strconv"
	"strings"
)

// Format transpiles the abstract tree back into normalized Racket source.
func Format(root *Node) string {
	if root == nil {
		return ""
	}

	if root.Kind != KindProgram {
		return render(root, 0) + "\n"
	}

	var out strings.Builder
	wroteDatum := false
	for i, child := range root.Children {
		if child.Kind == KindError {
			continue
		}

		if child.Kind == KindLang {
			if wroteDatum {
				out.WriteString("\n")
			}
			out.WriteString("#lang ")
			out.WriteString(child.Value)
			if hasPrintableDatum(root.Children[i+1:]) {
				out.WriteString("\n\n")
			} else {
				out.WriteString("\n")
			}
			wroteDatum = true
			continue
		}

		if wroteDatum {
			out.WriteString("\n")
		}
		out.WriteString(render(child, 0))
		out.WriteString("\n")
		wroteDatum = true
	}

	return out.String()
}

func hasPrintableDatum(nodes []*Node) bool {
	for _, node := range nodes {
		if node.Kind != KindError {
			return true
		}
	}
	return false
}

func render(node *Node, indent int) string {
	if node == nil {
		return ""
	}

	switch node.Kind {
	case KindProgram:
		return strings.TrimRight(Format(node), "\n")
	case KindLang:
		return "#lang " + node.Value
	case KindAtom:
		return node.Value
	case KindString:
		return strconv.Quote(node.Value)
	case KindList:
		return renderSequence("(", ")", node.Children, indent)
	case KindVector:
		return renderSequence("#(", ")", node.Children, indent)
	case KindByteVector:
		return renderSequence("#u8(", ")", node.Children, indent)
	case KindQuote:
		return renderQuote(node, indent)
	case KindError:
		return ""
	default:
		return node.Value
	}
}

func renderQuote(node *Node, indent int) string {
	if len(node.Children) == 0 {
		return quotePrefix(node.Value)
	}

	return quotePrefix(node.Value) + render(node.Children[0], indent)
}

func quotePrefix(value string) string {
	switch value {
	case "quote":
		return "'"
	case "quasiquote":
		return "`"
	case "unquote":
		return ","
	case "unquote-splicing":
		return ",@"
	default:
		return "'" + value + " "
	}
}

func renderSequence(open, close string, children []*Node, indent int) string {
	printable := printableChildren(children)
	if len(printable) == 0 {
		return open + close
	}

	parts := make([]string, 0, len(printable))
	for _, child := range printable {
		parts = append(parts, render(child, indent+len(open)))
	}

	oneLine := open + strings.Join(parts, " ") + close
	if len(oneLine)+indent <= 88 && !containsNewline(oneLine) {
		return oneLine
	}

	var out strings.Builder
	out.WriteString(open)
	out.WriteString(parts[0])
	for _, part := range parts[1:] {
		out.WriteString("\n")
		out.WriteString(strings.Repeat(" ", indent+2))
		out.WriteString(part)
	}
	out.WriteString(close)
	return out.String()
}

func printableChildren(children []*Node) []*Node {
	printable := make([]*Node, 0, len(children))
	for _, child := range children {
		if child.Kind == KindError {
			continue
		}
		printable = append(printable, child)
	}
	return printable
}

func containsNewline(value string) bool {
	return strings.Contains(value, "\n")
}

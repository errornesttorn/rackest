package racket

import (
	"fmt"
	"strings"
)

// Parse reads as much Racket reader syntax as possible into an abstract tree.
func Parse(source string) *ParseResult {
	p := &parser{
		source: source,
		line:   1,
		column: 1,
	}

	root := &Node{Kind: KindProgram}
	p.skipIgnorable()
	if lang := p.parseLangDirective(); lang != nil {
		root.Children = append(root.Children, lang)
	}

	for {
		p.skipIgnorable()
		if p.eof() {
			break
		}

		node := p.parseDatum()
		if node == nil {
			break
		}
		root.Children = append(root.Children, node)
	}

	root.End = p.pos
	return &ParseResult{Root: root, Diagnostics: p.diagnostics}
}

type parser struct {
	source      string
	pos         int
	line        int
	column      int
	diagnostics []Diagnostic
}

func (p *parser) parseLangDirective() *Node {
	if !strings.HasPrefix(p.source[p.pos:], "#lang") {
		return nil
	}

	after := p.pos + len("#lang")
	if after < len(p.source) && !isWhitespace(p.source[after]) {
		return nil
	}

	start := p.pos
	p.advanceN(len("#lang"))
	for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
		p.advance()
	}

	valueStart := p.pos
	for !p.eof() && p.peek() != '\n' && p.peek() != '\r' {
		p.advance()
	}
	value := strings.TrimSpace(p.source[valueStart:p.pos])
	if value == "" {
		p.addDiagnostic("missing language after #lang")
	}

	for !p.eof() && (p.peek() == '\n' || p.peek() == '\r') {
		p.advance()
	}

	return &Node{
		Kind:  KindLang,
		Value: value,
		Start: start,
		End:   p.pos,
	}
}

func (p *parser) parseDatum() *Node {
	p.skipIgnorable()
	if p.eof() {
		return nil
	}

	start := p.pos
	switch p.peek() {
	case '(':
		return p.parseDelimited(KindList, start, "(", ")")
	case '[':
		return p.parseDelimited(KindList, start, "[", "]")
	case '{':
		return p.parseDelimited(KindList, start, "{", "}")
	case ')', ']', '}':
		got := p.advance()
		message := fmt.Sprintf("unexpected closing delimiter %q", string(got))
		p.addDiagnosticAt(message, start)
		return &Node{Kind: KindError, Value: message, Start: start, End: p.pos}
	case '"':
		return p.parseString()
	case '\'':
		return p.parsePrefix("quote", start, 1)
	case '`':
		return p.parsePrefix("quasiquote", start, 1)
	case ',':
		if p.hasPrefix(",@") {
			return p.parsePrefix("unquote-splicing", start, 2)
		}
		return p.parsePrefix("unquote", start, 1)
	case '#':
		if p.hasPrefix("#(") {
			return p.parseDelimited(KindVector, start, "#(", ")")
		}
		if p.hasPrefix("#u8(") {
			return p.parseDelimited(KindByteVector, start, "#u8(", ")")
		}
	}

	return p.parseAtom()
}

func (p *parser) parseDelimited(kind Kind, start int, open, close string) *Node {
	p.advanceN(len(open))

	node := &Node{Kind: kind, Start: start}
	for {
		p.skipIgnorable()
		if p.eof() {
			message := fmt.Sprintf("missing closing delimiter %q", close)
			p.addDiagnosticAt(message, start)
			node.Children = append(node.Children, &Node{Kind: KindError, Value: message, Start: p.pos, End: p.pos})
			node.End = p.pos
			return node
		}

		if p.hasPrefix(close) {
			p.advanceN(len(close))
			node.End = p.pos
			return node
		}

		if isClosingDelimiter(p.peek()) {
			got := p.advance()
			message := fmt.Sprintf("mismatched closing delimiter %q, expected %q", string(got), close)
			p.addDiagnosticAt(message, p.pos-1)
			node.Children = append(node.Children, &Node{Kind: KindError, Value: message, Start: p.pos - 1, End: p.pos})
			node.End = p.pos
			return node
		}

		child := p.parseDatum()
		if child == nil {
			node.End = p.pos
			return node
		}
		node.Children = append(node.Children, child)
	}
}

func (p *parser) parsePrefix(value string, start, width int) *Node {
	p.advanceN(width)
	node := &Node{Kind: KindQuote, Value: value, Start: start}
	child := p.parseDatum()
	if child == nil {
		message := fmt.Sprintf("missing datum after %s", value)
		p.addDiagnosticAt(message, start)
		child = &Node{Kind: KindError, Value: message, Start: p.pos, End: p.pos}
	}
	node.Children = append(node.Children, child)
	node.End = child.End
	return node
}

func (p *parser) parseString() *Node {
	start := p.pos
	p.advance()

	var value strings.Builder
	for !p.eof() {
		c := p.advance()
		switch c {
		case '"':
			return &Node{Kind: KindString, Value: value.String(), Start: start, End: p.pos}
		case '\\':
			if p.eof() {
				message := "unfinished string escape"
				p.addDiagnosticAt(message, p.pos)
				return &Node{Kind: KindString, Value: value.String(), Start: start, End: p.pos}
			}
			value.WriteString(p.readEscape())
		default:
			value.WriteByte(c)
		}
	}

	message := "missing closing string quote"
	p.addDiagnosticAt(message, start)
	return &Node{Kind: KindString, Value: value.String(), Start: start, End: p.pos}
}

func (p *parser) readEscape() string {
	c := p.advance()
	switch c {
	case 'a':
		return "\a"
	case 'b':
		return "\b"
	case 'f':
		return "\f"
	case 'n':
		return "\n"
	case 'r':
		return "\r"
	case 't':
		return "\t"
	case 'v':
		return "\v"
	case '"':
		return "\""
	case '\\':
		return "\\"
	default:
		return string(c)
	}
}

func (p *parser) parseAtom() *Node {
	start := p.pos
	if p.peek() == '|' {
		p.advance()
		escaped := false
		for !p.eof() {
			c := p.advance()
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '|' {
				return &Node{Kind: KindAtom, Value: p.source[start:p.pos], Start: start, End: p.pos}
			}
		}

		message := "missing closing | for symbol"
		p.addDiagnosticAt(message, start)
		return &Node{Kind: KindAtom, Value: p.source[start:p.pos], Start: start, End: p.pos}
	}

	for !p.eof() && !isDelimiter(p.peek()) && !p.startsComment() {
		p.advance()
	}

	if start == p.pos {
		c := p.advance()
		message := fmt.Sprintf("unexpected character %q", string(c))
		p.addDiagnosticAt(message, start)
		return &Node{Kind: KindError, Value: message, Start: start, End: p.pos}
	}

	return &Node{Kind: KindAtom, Value: p.source[start:p.pos], Start: start, End: p.pos}
}

func (p *parser) skipIgnorable() {
	for !p.eof() {
		switch {
		case isWhitespace(p.peek()):
			p.advance()
		case p.peek() == ';':
			p.skipLineComment()
		case p.hasPrefix("#|"):
			p.skipBlockComment()
		case p.hasPrefix("#;"):
			p.advanceN(2)
			p.skipIgnorable()
			if p.parseDatum() == nil {
				p.addDiagnostic("datum comment is missing a datum")
			}
		default:
			return
		}
	}
}

func (p *parser) skipLineComment() {
	for !p.eof() && p.peek() != '\n' {
		p.advance()
	}
}

func (p *parser) skipBlockComment() {
	start := p.pos
	p.advanceN(2)
	depth := 1
	for !p.eof() && depth > 0 {
		switch {
		case p.hasPrefix("#|"):
			p.advanceN(2)
			depth++
		case p.hasPrefix("|#"):
			p.advanceN(2)
			depth--
		default:
			p.advance()
		}
	}
	if depth > 0 {
		p.addDiagnosticAt("missing closing |# for block comment", start)
	}
}

func (p *parser) startsComment() bool {
	return p.peek() == ';' || p.hasPrefix("#|") || p.hasPrefix("#;")
}

func (p *parser) addDiagnostic(message string) {
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Message: message,
		Offset:  p.pos,
		Line:    p.line,
		Column:  p.column,
	})
}

func (p *parser) addDiagnosticAt(message string, offset int) {
	line, column := lineColumn(p.source, offset)
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Message: message,
		Offset:  offset,
		Line:    line,
		Column:  column,
	})
}

func (p *parser) eof() bool {
	return p.pos >= len(p.source)
}

func (p *parser) peek() byte {
	return p.source[p.pos]
}

func (p *parser) hasPrefix(prefix string) bool {
	return strings.HasPrefix(p.source[p.pos:], prefix)
}

func (p *parser) advance() byte {
	c := p.source[p.pos]
	p.pos++
	if c == '\n' {
		p.line++
		p.column = 1
	} else {
		p.column++
	}
	return c
}

func (p *parser) advanceN(n int) {
	for i := 0; i < n && !p.eof(); i++ {
		p.advance()
	}
}

func isWhitespace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func isDelimiter(c byte) bool {
	if isWhitespace(c) {
		return true
	}

	switch c {
	case '(', ')', '[', ']', '{', '}', '"', '\'', '`', ',', ';':
		return true
	default:
		return false
	}
}

func isClosingDelimiter(c byte) bool {
	return c == ')' || c == ']' || c == '}'
}

func lineColumn(source string, offset int) (int, int) {
	line := 1
	column := 1
	for i := 0; i < len(source) && i < offset; i++ {
		if source[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return line, column
}

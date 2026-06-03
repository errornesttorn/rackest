package racket

// Kind identifies the abstract node type used by the editor.
type Kind string

const (
	KindProgram    Kind = "program"
	KindLang       Kind = "lang"
	KindList       Kind = "list"
	KindVector     Kind = "vector"
	KindByteVector Kind = "byte-vector"
	KindQuote      Kind = "quote"
	KindAtom       Kind = "atom"
	KindString     Kind = "string"
	KindError      Kind = "error"
)

// Node is a recoverable abstract tree node for Racket reader syntax.
type Node struct {
	Kind     Kind
	Value    string
	Children []*Node
	Start    int
	End      int
}

// Diagnostic describes a recoverable syntax problem found while parsing.
type Diagnostic struct {
	Message string
	Offset  int
	Line    int
	Column  int
}

// ParseResult contains the parsed tree and any syntax diagnostics.
type ParseResult struct {
	Root        *Node
	Diagnostics []Diagnostic
}

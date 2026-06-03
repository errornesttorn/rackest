package app

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"rackest/internal/racket"
)

const starterRacket = `#lang racket

(define (square x)
  (* x x))

(displayln (square 5))
`

const maxTreeLabelRunes = 80

// Run starts the Rackest desktop application.
func Run() {
	a := fyneapp.NewWithID("dev.rackest.editor")
	w := a.NewWindow("Rackest")

	editor := newEditor(w)
	source := editor.sourcePane()
	tree := editor.treePane()

	split := container.NewHSplit(source, tree)
	split.Offset = 0.62

	editor.installFileActions()
	w.SetContent(split)
	w.Resize(fyne.NewSize(1100, 720))
	w.CenterOnScreen()
	w.ShowAndRun()
}

type editor struct {
	window     fyne.Window
	entry      *sourceEntry
	tree       *astTreeView
	result     *racket.ParseResult
	currentURI fyne.URI
	loading    bool
}

func newEditor(window fyne.Window) *editor {
	entry := newSourceEntry()
	entry.Wrapping = fyne.TextWrapOff
	entry.Scroll = container.ScrollBoth
	entry.TextStyle = fyne.TextStyle{Monospace: true}
	entry.SetMinRowsVisible(30)
	tree := newASTTreeView()

	e := &editor{
		window: window,
		entry:  entry,
		tree:   tree,
	}

	entry.OnChanged = e.sourceChanged
	entry.onFocusLost = e.normalizeSourceFromTree
	e.loadSource(starterRacket, nil)

	return e
}

func (e *editor) sourcePane() fyne.CanvasObject {
	return container.NewBorder(
		widget.NewLabel("Racket source"),
		nil,
		nil,
		nil,
		container.NewStack(e.entry),
	)
}

func (e *editor) treePane() fyne.CanvasObject {
	return container.NewBorder(
		widget.NewLabel("Abstract tree"),
		nil,
		nil,
		nil,
		container.NewStack(e.tree.Widget()),
	)
}

func (e *editor) installFileActions() {
	openShortcut := &desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl}
	saveShortcut := &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}
	saveAsShortcut := &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}

	openItem := fyne.NewMenuItem("Open", e.openFile)
	openItem.Shortcut = openShortcut

	saveItem := fyne.NewMenuItem("Save", e.saveFile)
	saveItem.Shortcut = saveShortcut

	saveAsItem := fyne.NewMenuItem("Save As", e.saveFileAs)
	saveAsItem.Shortcut = saveAsShortcut

	fileMenu := fyne.NewMenu("File", openItem, saveItem, saveAsItem)
	e.window.SetMainMenu(fyne.NewMainMenu(fileMenu))

	canvas := e.window.Canvas()
	canvas.AddShortcut(openShortcut, func(fyne.Shortcut) {
		e.openFile()
	})
	canvas.AddShortcut(saveShortcut, func(fyne.Shortcut) {
		e.saveFile()
	})
	canvas.AddShortcut(saveAsShortcut, func(fyne.Shortcut) {
		e.saveFileAs()
	})
}

func (e *editor) openFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, e.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			dialog.ShowError(err, e.window)
			return
		}

		e.loadSource(string(content), reader.URI())
		e.updateTitle()
	}, e.window)
}

func (e *editor) saveFile() {
	if e.currentURI == nil || e.currentURI.Scheme() != "file" {
		e.saveFileAs()
		return
	}

	if err := os.WriteFile(e.currentURI.Path(), []byte(e.transpiledSource()), 0o644); err != nil {
		dialog.ShowError(err, e.window)
		return
	}
	e.updateTitle()
}

func (e *editor) saveFileAs() {
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, e.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()

		if _, err := io.WriteString(writer, e.transpiledSource()); err != nil {
			dialog.ShowError(err, e.window)
			return
		}

		e.currentURI = writer.URI()
		e.updateTitle()
	}, e.window)
	saveDialog.SetFileName("untitled.rkt")
	saveDialog.Show()
}

func (e *editor) updateTitle() {
	if e.currentURI == nil {
		e.window.SetTitle("Rackest")
		return
	}
	e.window.SetTitle(fmt.Sprintf("Rackest - %s", e.currentURI.Name()))
}

func (e *editor) loadSource(source string, uri fyne.URI) {
	result := racket.Parse(source)
	normalized := racket.Format(result.Root)

	e.loading = true
	e.entry.SetText(normalized)
	e.loading = false

	e.currentURI = uri
	e.result = racket.Parse(normalized)
	e.tree.Update(e.result)
}

func (e *editor) sourceChanged(source string) {
	if e.loading {
		return
	}

	e.result = racket.Parse(source)
	e.tree.Update(e.result)
}

func (e *editor) normalizeSourceFromTree() {
	if e.loading || e.result == nil {
		return
	}

	normalized := racket.Format(e.result.Root)
	if normalized == e.entry.Text {
		return
	}

	e.loading = true
	e.entry.SetText(normalized)
	e.loading = false

	e.result = racket.Parse(normalized)
	e.tree.Update(e.result)
}

func (e *editor) transpiledSource() string {
	if e.result == nil {
		return e.entry.Text
	}
	return racket.Format(e.result.Root)
}

type sourceEntry struct {
	*widget.Entry
	onFocusLost func()
}

func newSourceEntry() *sourceEntry {
	return &sourceEntry{Entry: widget.NewMultiLineEntry()}
}

func (e *sourceEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusLost != nil {
		e.onFocusLost()
	}
}

type astTreeView struct {
	tree     *widget.Tree
	labels   map[string]string
	children map[string][]string
}

func newASTTreeView() *astTreeView {
	view := &astTreeView{
		labels:   map[string]string{"root": "program"},
		children: map[string][]string{"root": nil},
	}

	view.tree = widget.NewTree(
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			return view.children[id]
		},
		func(id widget.TreeNodeID) bool {
			return len(view.children[id]) > 0
		},
		func(bool) fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return label
		},
		func(id widget.TreeNodeID, _ bool, object fyne.CanvasObject) {
			object.(*widget.Label).SetText(view.labels[id])
		},
	)
	view.tree.Root = "root"
	view.tree.HideSeparators = true

	return view
}

func (v *astTreeView) Widget() fyne.CanvasObject {
	return v.tree
}

func (v *astTreeView) Update(result *racket.ParseResult) {
	labels := map[string]string{"root": "program"}
	children := map[string][]string{"root": nil}

	if result != nil {
		next := 0
		if len(result.Diagnostics) > 0 {
			diagnosticsID := "diagnostics"
			labels[diagnosticsID] = fmt.Sprintf("diagnostics (%d)", len(result.Diagnostics))
			children["root"] = append(children["root"], diagnosticsID)
			for _, diagnostic := range result.Diagnostics {
				id := fmt.Sprintf("diagnostic-%d", next)
				next++
				labels[id] = fmt.Sprintf("line %d:%d %s", diagnostic.Line, diagnostic.Column, diagnostic.Message)
				children[diagnosticsID] = append(children[diagnosticsID], id)
			}
		}

		for _, child := range result.Root.Children {
			id := fmt.Sprintf("node-%d", next)
			next++
			v.addNode(labels, children, id, child, &next)
			children["root"] = append(children["root"], id)
		}
	}

	v.labels = labels
	v.children = children
	v.tree.Refresh()
	v.tree.OpenAllBranches()
}

func (v *astTreeView) addNode(labels map[string]string, children map[string][]string, id string, node *racket.Node, next *int) {
	labels[id] = nodeLabel(node)
	for _, child := range node.Children {
		childID := fmt.Sprintf("node-%d", *next)
		*next = *next + 1
		v.addNode(labels, children, childID, child, next)
		children[id] = append(children[id], childID)
	}
}

func nodeLabel(node *racket.Node) string {
	var label string
	switch node.Kind {
	case racket.KindProgram:
		label = "program"
	case racket.KindLang:
		label = "#lang " + node.Value
	case racket.KindList:
		label = fmt.Sprintf("list (%d)", len(node.Children))
	case racket.KindVector:
		label = fmt.Sprintf("vector (%d)", len(node.Children))
	case racket.KindByteVector:
		label = fmt.Sprintf("byte-vector (%d)", len(node.Children))
	case racket.KindQuote:
		label = node.Value
	case racket.KindAtom:
		label = "atom " + node.Value
	case racket.KindString:
		label = "string " + strconv.Quote(node.Value)
	case racket.KindError:
		label = "error " + node.Value
	default:
		label = string(node.Kind)
	}
	return truncateTreeLabel(label)
}

func truncateTreeLabel(label string) string {
	runes := []rune(label)
	if len(runes) <= maxTreeLabelRunes {
		return label
	}
	return string(runes[:maxTreeLabelRunes-3]) + "..."
}

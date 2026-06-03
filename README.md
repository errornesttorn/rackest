# Rackest

Rackest is a native Linux desktop experiment for constructing Racket code from an editable indent tree.

The first step is a minimal Fyne text editor:

- Racket source on the left.
- Abstract syntax tree preview on the right.
- A resizable split between both panes.
- File open/save actions from the File menu.
- Keyboard shortcuts: `Ctrl+O` opens a file, `Ctrl+S` saves, and `Ctrl+Shift+S`
  saves as a new file.

The editor parses Racket reader syntax into an abstract tree, then transpiles
that tree back into normalized Racket source. Opening a file parses it first and
shows the normalized printer output. While typing, the tree updates immediately;
the source text is only normalized again when the editor loses focus.

## Run

```sh
go run -buildvcs=false ./cmd/rackest
```

The `-buildvcs=false` flag avoids Go VCS stamping issues in workspaces where `.git`
metadata is unavailable or synthetic.

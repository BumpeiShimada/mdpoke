# mdpoke

`mdpoke` is a terminal Markdown viewer for poking around long Markdown documents.

It focuses on reading one Markdown file at a time, with a main rendered document view, an optional heading outline, search, link focus, link copy, drag-selected text copy, and a searchable key guide.

## Install

```sh
go install github.com/BumpeiShimada/mdpoke/cmd/mdpoke@latest
```

If `mdpoke` is not found after installing, add Go's bin directory to your shell `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

For zsh, put that line in `~/.zshrc`.

For local development:

```sh
git clone https://github.com/BumpeiShimada/mdpoke.git
cd mdpoke
go install ./cmd/mdpoke
```

## Run

```sh
mdpoke README.md
```

When developing locally without installing:

```sh
go run ./cmd/mdpoke -- README.md
```

## Homebrew

The project can be distributed through Homebrew once tagged releases are available. The expected path is to publish release archives, add a formula that builds `./cmd/mdpoke`, and then install it with `brew install BumpeiShimada/tap/mdpoke`.

## Keys

| Key | Action |
| --- | --- |
| `j` / `k`, arrow keys | Scroll or move the outline selection |
| `g` / `G` | Jump to top / bottom |
| `o` | Toggle the outline pane |
| `Enter` | Follow the focused Markdown internal link |
| `/` | Search rendered text |
| `n` / `N` | Move to next / previous search match |
| `Tab` / `Shift+Tab` | Focus next / previous link |
| `y` | Copy selected text, the focused link, or the first link on the current line |
| Mouse wheel | Scroll |
| Drag | Select rendered text; press `y` after release to copy it |
| Click | Copy an external link or confirm an internal Markdown jump |
| `?` | Open the searchable key guide |
| `Esc` | Cancel the current mode or clear highlights/selection |
| `q` / `Ctrl+C` | Quit |

Internal Markdown anchors such as `#heading-name` can be followed with `Enter`.
External links are intentionally copy-first: focus them with the keyboard and press `y`, or click them to copy.
You can also drag across rendered document text, release, and press `y` to copy the selected plain text. Copy actions show a short `Copied` popup, which can be closed with any key or an outside click.

## Scope

`mdpoke` is a viewer, not an editor. It does not include file browsing; use it with shell tools, `fzf`, or your editor.

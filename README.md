# mdpoke

`mdpoke` is a terminal Markdown viewer for poking around Markdown documents.

"Poking around" means that you can:

- Read with an optional heading outline
- Drag to select text and release to copy
- Toggle checkboxes
- Jump around files with search
- ...and more!

## Install

```sh
brew install BumpeiShimada/tap/mdpoke
```

NB! not sure if the binary for Windows or Linux works yet.

## Highlights

### Read with an optional heading outline (Open/Close with `o` key)

<img width="1032" height="692" alt="first" src="https://github.com/user-attachments/assets/08e920f5-7c34-4026-91f7-b42a63fe709f" />

### Drag to select text and release to copy

<img width="1032" height="692" alt="fifth" src="https://github.com/user-attachments/assets/7c47e356-7aed-47da-b064-da3596fdd992" />

### Toggle checkboxes with mouse click / Focus with `Tab` -> toggle with `Space` or `Enter`

<img width="1032" height="692" alt="third" src="https://github.com/user-attachments/assets/0b9d23e2-c25d-4e8d-86a3-4f6ed040a7d3" />

### Jump around files with search

<img width="1032" height="692" alt="second" src="https://github.com/user-attachments/assets/4c2711a1-ab3f-4d1f-9638-b612b0d47753" />

## Run

```sh
mdpoke README.md
```

Useful reload and file options:

```sh
mdpoke --no-watch README.md
mdpoke --max-size 10485760 README.md
mdpoke --follow-symlinks README.md
mdpoke --help
```

Use `--no-watch` when automatic reloads are not desired, `--max-size` to tighten or raise the read limit, and `--follow-symlinks` only when the link target is trusted.

## Keys

| Key | Action |
| --- | --- |
| `j` / `k`, arrow keys | Scroll or move the outline selection |
| `g` / `G` | Jump to top / bottom |
| `o` | Toggle the outline pane |
| `Enter` / `Space` | Toggle the focused checkbox |
| `/` | Search rendered text |
| `n` / `N` | Move to next / previous search match |
| `Tab` / `Shift+Tab` | Focus next / previous checkbox |
| `y` | Copy the focused link, or the first link on the current line |
| Mouse wheel | Scroll |
| Drag | Select rendered text; release to copy it immediately |
| Click | Toggle a checkbox, copy an external link, or confirm an internal Markdown jump when clicking the rendered link text |
| `?` | Open the searchable key guide |
| `Esc` | Cancel the current mode or clear highlights/selection |
| `q` / `Ctrl+C` | Quit |

## Safety And Limits

By default, `mdpoke` watches the opened file for changes, refuses symlinked Markdown files, limits reads to 20 MiB, and strips terminal control characters before rendering or parsing links/headings.

## Other functions

### Jump into internal Markdown anchors with a confirmation prompt

<img width="1032" height="692" alt="sixth" src="https://github.com/user-attachments/assets/c7f619d2-884b-49cc-b35f-db8ccf9cb952" />

### Copy links with mouse click

<img width="1032" height="692" alt="forth" src="https://github.com/user-attachments/assets/4ed4380d-c7b4-4b60-86b8-291daa10cd50" />

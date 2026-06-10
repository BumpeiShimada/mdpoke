# mdpoke Test Fixture

This file is intentionally shaped to exercise the main mdpoke interactions:
outline navigation, search, links, internal anchors, nested lists, code blocks,
quotes, tables, task lists, images-as-text, HTML, and long wrapped text.

Use it like this:

```sh
go run ./cmd/mdpoke -- testdata/fixtures/comprehensive.md
```

## Quick Checklist

- Press `o` and move through the outline with `j` / `k`.
- Click headings in the outline and scroll the outline with the mouse wheel.
- Press `/fixture` and move matches with `n` / `N`.
- Confirm every visible `fixture` match is highlighted, while the focused match is more prominent.
- Confirm focused matches, links, and outline jumps land near the vertical center of the viewport.
- Press `/TARGET` to confirm case-insensitive search.
- Press `tab` repeatedly to focus checkboxes.
- Press `enter` on [Jump Target](#jump-target).
- Press `y` on [External Example](https://example.com) to copy the URL.
- Press `?` and search for `copy`.

## Heading Gallery

This section checks heading color, weight, spacing, and outline nesting.

### H3: Product Surface

#### H4: Reader Mode

##### H5: Compact Detail

###### H6: Smallest Styled Heading

The renderer styles H1 through H6 deliberately. H6 should be readable and should still appear in the outline tree.

###### H6: Another Small Heading

This second H6 checks repeated H6 rendering and outline navigation at the deepest CommonMark heading level.

## Heading Shapes

### Heading With `inline code`

### Heading With Punctuation: alpha/beta, gamma?

### Heading With 日本語

### Heading With Numbers 123

### Heading With Emoji Text

Emoji shortcode and unicode should stay readable: :sparkles: ✨

## Links

Internal links:

- [Jump Target](#jump-target)
- [Nested Lists](#nested-lists)
- [Table Samples](#table-samples)
- [Code Samples](#code-samples)
- [Heading With Punctuation](#heading-with-punctuation-alpha-beta-gamma)

External links:

- [External Example](https://example.com)
- [Go](https://go.dev)
- [Charm](https://charm.sh)
- [GitHub Markdown Guide](https://docs.github.com/en/get-started/writing-on-github)

Mixed prose link line:
Read [the jump target](#jump-target), then copy [the external URL](https://example.com/path?query=fixture).

Reference style link:
This sentence uses [a reference link][fixture-reference].

[fixture-reference]: https://example.com/reference-fixture

Autolink:
<https://example.com/autolink-fixture>

## Search Samples

fixture appears on this line.

This sentence has fixture twice: fixture.

fixture fixture fixture on one line should highlight all three visible matches.

Case-insensitive search should also find FIXTURE.

The word target appears here, and the TARGET word appears again.

Searching for `日本語` should land on the Japanese heading and this 日本語 sentence.

Centering check:

Before the focused search target.

Spacer line 01.

Spacer line 02.

Spacer line 03.

Spacer line 04.

Spacer line 05.

focused-center-fixture should land near the middle of the viewport when searched.

Spacer line 06.

Spacer line 07.

Spacer line 08.

Spacer line 09.

Spacer line 10.

After the focused search target.

## Nested Lists

- Level one bullet
  - Level two bullet
    - Level three bullet with [inline link](#code-samples)
      - Level four bullet
        - Level five bullet
- Another top-level bullet with a long sentence that should wrap cleanly when the terminal is narrow and still preserve the visual indentation of the bullet item after wrapping.
- Bullet containing inline styles: **strong**, _emphasis_, `code`, ~~strikethrough~~.

1. First numbered item
   1. Nested numbered item
      1. Deep numbered item
2. Second numbered item
   - Mixed bullet under numbered item
     - Mixed nested bullet under numbered item

Task list:

- [x] Completed fixture case
- [ ] Pending fixture case
- [ ] Another task with [task link](#links)

Checkbox toggle cases:

- [ ] Toggle this pending fixture checkbox with tab then space or enter
- [x] Toggle this completed fixture checkbox by clicking the checkbox
- [ ] Toggle this nested fixture checkbox group
    - [ ] Nested pending checkbox
    - [x] Nested completed checkbox

Inline code at the start of bullet items:

- `aaa`
    - `xxx`
- `bbb`
    - `yyy`
- `111`
    - `000`
- `222`
    - `000`

## Centering Targets

Use this section to verify viewport centering when jumping.

### Link Center Target

Follow [this internal link](#outline-center-target) and confirm the target heading appears near the viewport middle.

### Outline Center Target

Select this heading from the outline with keyboard or mouse. The selected heading should move near the middle of the main viewport.

### Search Center Target

Search for `center-me-fixture`. The focused search result should be near the middle, and all other fixture matches should remain highlighted.

center-me-fixture

## Code Samples

Inline code: `mdpoke testdata/fixtures/comprehensive.md`

### Go

#### Tiny Program

##### Function Body

```go
package main

import "fmt"

type Viewer struct {
	Name string
}

func main() {
	viewer := Viewer{Name: "mdpoke"}
	fmt.Printf("%s fixture\n", viewer.Name)
}
```

### JavaScript

```javascript
const fixture = ["outline", "search", "links"].map((item) => item.toUpperCase());
console.log(fixture.join(", "));
```

### TypeScript

```typescript
type Command = "outline" | "search" | "copy";

const run = (command: Command): string => {
  return `running ${command}`;
};

console.log(run("outline"));
```

### Python

```python
from dataclasses import dataclass

@dataclass
class Fixture:
    name: str
    enabled: bool = True

def render(items: list[str]) -> str:
    return ", ".join(item.upper() for item in items)

print(render(["heading", "code", "outline"]))
```

### Ruby

```ruby
Fixture = Struct.new(:name, :enabled, keyword_init: true)

items = %w[heading code outline]
puts items.map(&:upcase).join(", ")
```

### Rust

```rust
#[derive(Debug)]
struct Fixture<'a> {
    name: &'a str,
}

fn main() {
    let fixture = Fixture { name: "mdpoke" };
    println!("{fixture:?}");
}
```

### Shell

```sh
set -euo pipefail

for target in docs/plan.md testdata/fixtures/comprehensive.md; do
  echo "checking ${target}"
done
```

### Indented Code Block In A List

1. mac で git を使えるようにする

    ```sh
    xcode-select --install
    ```

### JSON

```json
{
  "name": "mdpoke",
  "features": ["outline", "search", "syntax-highlight"],
  "enabled": true
}
```

### YAML

```yaml
name: mdpoke
features:
  - outline
  - search
  - syntax-highlight
enabled: true
```

### TOML

```toml
name = "mdpoke"
enabled = true

[features]
outline = true
search = true
syntax_highlight = true
```

### SQL

```sql
select id, title
from documents
where title like '%fixture%'
order by updated_at desc;
```

### CSS

```css
.heading {
  color: #89b4fa;
  font-weight: 700;
}
```

### HTML

```html
<article>
  <h1>Fixture</h1>
  <a href="#jump-target">Jump target</a>
</article>
```

### Markdown In Code

```markdown
## Markdown In Code

- This should not become an outline entry.
- [This should not become a real link](#nope)
```

### Unknown Language

```madeuplang
keyword maybe_highlighted? nope probably_plain
```

### Short Code Block Width

The top and bottom rules should hug the content instead of spanning the full viewport.

```text
tiny
longer but still compact
```

### Long Code Block Wrapping

The long line below should force the block to use the available width and wrap cleanly.

```javascript
const intentionallyLongFixtureLine = "abcdefghijklmnopqrstuvwxyz0123456789-abcdefghijklmnopqrstuvwxyz0123456789-abcdefghijklmnopqrstuvwxyz0123456789";
```

## Quotes

> A quoted paragraph with a [quoted link](#jump-target).
>
> - quoted bullet
> - another quoted bullet

Nested quote:

> First level quote.
>
> > Second level quote with `inline code`.

## Table Samples

| Feature | Key | Expected Result |
| --- | --- | --- |
| Outline | `o` | Right pane opens and closes |
| Outline click | mouse click | Clicked heading is selected |
| Outline scroll | mouse wheel | Outline scrolls when hovered |
| Search | `/fixture` | Matches are highlighted |
| Links | `tab` | Link line is focused |
| Copy | `y` | URL is copied |
| Help | `?` | Searchable guide opens |

Wide table:

| Column One | Column Two | Column Three | Column Four |
| --- | --- | --- | --- |
| short | medium content | longer content that may wrap | final cell |
| alpha | beta | gamma | delta |

Content-sized table:

| A | Longer Label | Count |
| --- | --- | --- |
| x | compact | 1 |
| yy | still small | 22 |

Wrapping table:

| Name | Description | Notes |
| --- | --- | --- |
| alpha | abcdefghijklmnopqrstuvwxyz0123456789-abcdefghijklmnopqrstuvwxyz0123456789 | this row should wrap when the terminal is narrow |
| beta | short | compact |

## Thematic Breaks

Above the break.

---

Below the break.

## Images And HTML

Image syntax should render as readable text:

![Alt text for fixture image](https://example.com/image.png)

Inline HTML should not break the viewer:

<kbd>o</kbd> toggles the outline, and <mark>fixture</mark> marks text in HTML.

## Long Wrapping

This is a deliberately long paragraph designed to wrap across multiple terminal widths. It includes the word fixture, a link to [Nested Lists](#nested-lists), and enough text to make visual wrapping obvious without needing any special terminal setup or external files. The paragraph keeps going so that narrow terminal windows reveal whether the wrap calculation, line mapping, search highlighting, and link focus behavior still feel coherent.

## Jump Target

If an internal anchor worked, pressing `enter` on the Jump Target link should land near this heading.

fixture target final line. External reload check: append a `Reload marker` line from another process and the viewer should refresh soon after the write.

Source line break check first line
Source line break check second line without trailing spaces
Source line break check third line without trailing spaces

## Regression Coverage Additions

This section keeps focused regression cases close to the manual fixture without changing earlier raw line numbers.

### Checkbox And Bullet Combinations

- [ ] fixture-checkbox-with-child-bullets: parent task with plain child bullets
  - fixture-child-bullet-one: child bullet stays visually separate from the parent task
  - fixture-child-bullet-two: second child bullet keeps its own rendered line mapping
- [x] fixture-checkbox-with-child-checkboxes: parent task with nested checkbox items
  - [ ] fixture-nested-checkbox-pending: nested pending checkbox remains independently focusable
  - [x] fixture-nested-checkbox-complete: nested completed checkbox remains independently focusable
* [ ] fixture-star-checkbox: star bullet task renders and toggles as a checkbox
+ [x] fixture-plus-checkbox: plus bullet task renders and toggles as a checkbox
- [ ] fixture-long-checkbox-ja: これは幅の狭い端末で複数行に折り返されることを確認するための長いチェックボックス項目です。クリックしても隣の項目に吸い寄せられず、この項目だけが切り替わる必要があります。
- [ ] fixture-long-checkbox-ja-second: これは同じ接頭辞と似た長さを持つ二つ目の長いチェックボックス項目です。行対応が前の項目に巻き戻らず、この二つ目の項目に対応する必要があります。

### Japanese Wrapping Samples

日本語長文fixture-ja-long-paragraph: これは日本語の長い段落が空白の少ない状態でも自然に折り返され、折り返しマーカーとコピー時の結合が破綻しないことを確認するためのダミーテキストです。かな交じり文と英字fixtureを混ぜ、検索、行対応、表示幅計算が同時に安定することを確認します。

日本語連続文字fixture-ja-unbroken: あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわをんあいうえおかきくけこさしすせそたちつてと

### Wrapped URL And Copy Samples

- fixture-wrapped-url-line: https://example.com/path/to/日本語リソース名/with/very/long/fixture/url/that/wraps/across/terminal/widths
- fixture-copy-boundary-list-one: first list item should copy as its own line
- fixture-copy-boundary-list-two: second list item should not collapse into the previous item

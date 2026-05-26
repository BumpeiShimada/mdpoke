# mdpoke plan

## Vision

`mdpoke` は Go 製の TUI Markdown viewer / navigator。
`treemd` の「Markdown を構造として読む」発想をベースにしつつ、普段の読書を邪魔しない、最小で直感的なビューアを目指す。

単なる `less` 的な本文表示ではなく、Markdown の見出し、リンク、コードブロック、TODO、表などを「移動しやすい情報構造」として扱う。
ただし常に多機能であることは目指さない。必要なときに必要なものだけ出ることを美学とする。

## Product Goals

- Markdown ファイルをターミナル上で気持ちよく読める
- 見出しは必要なときだけ表示し、本文表示を主役にする
- 見出しを選択すると本文ビューが即座に追従する
- キーボード操作は単発キー中心で、覚える負担が少ない
- いつでも `?` からキーマップや操作を検索できる
- マウススクロールとマウス選択が自然に使える
- リンクへ素早く移動、またはコピーできる
- ドラッグ選択した表示テキストを `y` でクリップボードへコピーできる
- Go のエコシステムで保守しやすく、拡張しやすい
- 文字列検索で読みたい箇所へ素早く戻れる

## Non-Goals for MVP

- Markdown エディタにはしない
- Obsidian / Logseq の完全代替は狙わない
- WebView やブラウザベース UI は使わない
- ファイル検索やファイルブラウザは入れない。fzf など外部ツールに任せる
- 最初から全 Markdown 拡張記法の完全対応を狙わない

## Current Implementation Snapshot

2026-05 時点の実装は、単一ファイル viewer としての MVP 機能をほぼ満たしている。

- CLI は `cmd/mdpoke/main.go` に集約し、`mdpoke <markdown-file>` だけを受け付ける
- TUI 状態管理、入力処理、本文 / アウトライン / モーダル / ヘルプ描画は `internal/app/model.go` にまとまっている
- Markdown レンダリング、goldmark による見出し・リンク抽出、Glamour style、custom code/table block 処理は `internal/markdown` にある
- 本文、アウトライン、検索、リンク、マウスクリック、ドラッグテキスト選択、クリップボードコピーは Go テストでカバーしている
- 外部 URL のクリックはブラウザで開かずコピー確認モーダルを出す。Markdown 内アンカーのクリックはジャンプ確認モーダルを出す
- 本文のドラッグ操作は表示文字単位で選択し、リリース後に `y` で選択範囲のプレーンテキストをコピーする
- 本文左側の読みやすさ用マージンは選択対象に含めない
- アウトラインを `j` / `k` またはクリックでフォーカスしたときは、対象見出しを本文ビューの上寄りに移動し、その行全体をアウトライン選択色で一時ハイライトする

## Core Experience

MVP は本文ビューを基本画面にする。
見出しは必要なときだけ右側に表示する。

- メイン: レンダリング済み本文
- 右ペイン: 必要時だけ出るアウトライン
- フッター: 現在位置、ファイル名、主要キー操作
- 右下ガイド: 現在の状態で入力可能な主要キーを常に見せる控えめなヒント

想定操作:

- `j` / `k` or arrow keys: 上下スクロール
- `h` / `l` or left / right: 階層移動、リンク候補移動、必要に応じたペイン操作
- `o`: アウトライン表示のトグル
- `Enter`: フォーカス中の Markdown 内リンクへ移動する
- `/`: 本文検索
- `n` / `N`: 検索結果の次 / 前
- `tab` / `shift+tab`: リンク候補の次 / 前へ移動
- `y`: ドラッグ選択中の表示テキスト、選択中リンク、現在行の URL の順でコピー
- `g` / `G`: 先頭 / 末尾へ移動
- `?`: キーマップ / 操作ガイドをポップアップ表示
- `q`: 終了
- mouse drag: 本文の表示テキストを文字単位で選択する。リリース後 `y` でコピーする

キーマップは `lazygit` や `spf` の使用感に寄せる。
複雑な同時押しや Command キー前提の操作は避け、よくある単発キーで完結させる。
右下ガイドは `lazygit` 的に、いま押せるキーを文脈に合わせて表示する。
`?` を押すと `spf` 的な綺麗なポップアップで操作一覧を見られる。

## MVP Features

### Markdown Rendering

- Markdown ファイルを読み込み、TUI 上に整形表示する
- 見出し、リスト、コードブロック、引用、リンクを最低限読みやすく表示する
- 見出しは H1〜H6 まで階層に応じて視覚的に区別する
- H1 は画面上で文書タイトルとして強く、H2〜H6 は段階的に控えめな色・接頭辞・余白で表現する
- bullet point の階層構造を正しくインデントして表示する
- treemd で見られるような「ネストした bullet point が階層化されず平坦に見える」問題を再現しない
- ターミナル幅に応じて本文を折り返す
- コードブロックは本文と明確に区別できるように罫線または背景で囲う
- コードブロックの上下罫線は各ブロック内の最長行より少し長い幅にし、端末幅を超える場合は最大幅に収めて折り返す
- fenced code block の言語指定を Chroma に渡し、Go / JavaScript / TypeScript / Python / Ruby / Rust / shell / JSON / YAML / Markdown など Chroma が対応する言語をできるだけ広くハイライトする
- Markdown table は各カラムの最長セルより少し広い幅を基本にし、端末幅に収まらない場合はカラム幅を均等化してセル内容を折り返す
- Glamour の custom style と Chroma theme を使い、Web の Markdown viewer で一般的な「見出しの階層感」「コードブロックの枠」「控えめな本文色」を TUI に落とし込む

### Outline Navigation

- Markdown AST から見出しツリーを構築する
- `o` で右ペインに見出し階層を表示 / 非表示する
- アウトライン表示は H1〜H6 の階層に従い、インデントで親子関係が分かるようにする
- 見出し選択中は、本文ビューが選択中の見出し位置に追従する
- `j` / `k` またはアウトライン上のクリックで選択中見出しへジャンプする
- アウトライン上では選択中の見出しだけを強くハイライトし、現在本文位置の補助表示は選択表示と競合しない控えめな扱いにする
- アウトラインを閉じると本文だけの表示に戻る
- アウトライン上の見出しはマウスクリックで選択・ジャンプできる
- アウトライン表示中は、マウスホイールがアウトライン領域上にある場合はアウトラインをスクロールし、本文領域上にある場合は本文をスクロールする

### Search

- `/` でインクリメンタルではない通常検索を開始する
- `Enter` で検索確定
- `n` / `N` で移動
- 検索中は本文側にヒット箇所をハイライトする

### Links

- Markdown 内のリンクを検出する
- 本文中のリンク候補をフォーカスできる
- `tab` / `shift+tab` でリンク候補を前後に移動する
- リンクをフォーカスすると本文ビューがリンク位置へスクロールし、該当リンクをハイライトする
- `Enter` でフォーカス中の Markdown 内リンクへ移動する
- `y` で選択中リンクの URL をクリップボードへコピーする
- リンク未フォーカス時に `y` を押した場合は、現在行の最初のリンクをコピー対象にする
- リンク移動は Markdown 内リンクを優先する
- 外部 URL は開くよりコピーを優先する

### Mouse

- マウスホイールで本文をスクロールできる
- アウトライン表示中はアウトライン側もマウスでスクロールできる
- 本文の左ドラッグで表示文字単位の選択範囲を作る
- ドラッグ選択中は本文の選択範囲をハイライトし、フッターに選択文字数を表示する
- ドラッグをリリースしたあと `y` で選択範囲のプレーンテキストをコピーする
- `y` でコピーしたあとは内容を表示せず、短い `Copied` モーダルだけを表示する
- コピー完了モーダルは任意キーまたはモーダル外クリックで閉じられる
- 表示上のリンク位置をクリックすると、外部 URL はコピー確認モーダル、Markdown 内リンクはジャンプ確認モーダルを出す。ジャンプ確認モーダルもモーダル外クリックで閉じられる
- リンククリックの確定は mouse release で行い、ドラッグ選択と衝突しないようにする
- リンクフォーカス中は `y` でコピー、`Enter` で Markdown 内リンク移動を行う

### Guide

- 画面右下に小さな文脈ガイドを常時表示する
- ガイドには現在の状態で入力可能な主要キーだけを表示する
- 通常時は `o outline`, `/ search`, `? help`, `q quit` などを表示する
- 検索結果があるときは `n next`, `N prev` を表示する
- アウトライン表示中は `j/k move`, `o close` などを表示する
- ドラッグテキスト選択中は `y copy text`, `esc clear` を表示する
- リンク未選択時は `tab url focus` を表示する
- リンク選択中は `enter follow`, `y copy`, `tab next` などを表示する
- `?` でキーマップ / 操作ガイドのポップアップを開く
- ガイドは `spf` のように、中央または右寄せの綺麗なウィンドウとして表示する
- ガイド内で文字列検索できる
- 検索対象はキー、操作名、説明文にする
- ガイド表示中も単発キーで閉じられる

### File Input

- まずは単一ファイル指定をサポートする

```sh
mdpoke README.md
```

- 引数なしの場合はヘルプを表示する
- 存在しないファイル、ディレクトリ指定、読み取り不可ファイルはエラーメッセージを出して終了する

## Suggested Tech Stack

- Language: Go
- TUI framework: Bubble Tea
- Components: Bubbles
- Styling: Lip Gloss
- Markdown rendering: Glamour
- Markdown parsing / AST: goldmark
- CLI args: 標準 `flag` から開始。必要になったら Cobra などを検討する

理由:

- Go に慣れている前提と相性がよい
- Charmbracelet 系は TUI の見た目、単発キーマップ、マウスイベント処理を作りやすい
- Glamour は Markdown のターミナル表示に強い
- goldmark は AST を使ったアウトライン生成に向いている

### Stack Selection Notes

`mdpoke` では、TUI の中核は Charmbracelet 系で揃え、Markdown の構造解析は goldmark に寄せる。
理由は、本文ビュー、右側アウトライン、文脈ガイド、検索可能ヘルプ、マウスイベントを「状態に応じて表示が変わるアプリ」として作りたいから。

| Area | Selected | Main Competitors | Why Selected |
| --- | --- | --- | --- |
| TUI framework | Bubble Tea | tview, tcell, gocui | Elm Architecture 的な `model/update/view` で状態遷移を整理しやすい。文脈ガイド、検索モード、アウトライン表示、リンクフォーカスなど状態が多いアプリに向く |
| TUI components | Bubbles | tview built-in widgets, 自作 | viewport, textinput, help, key binding などが Bubble Tea と自然に組み合わさる。全部自作するよりMVPが早い |
| Styling/layout | Lip Gloss | tview style, raw ANSI, termenv | border, padding, width, color を宣言的に書ける。spf ライクなポップアップや控えめな右下ガイドを作りやすい |
| Markdown rendering | Glamour | goldmark custom renderer, gomarkdown, blackfriday | ターミナル向け Markdown レンダリングをすぐ使える。初期MVPでは最短。ただし nested list は golden test で必ず検証する |
| Markdown parsing / AST | goldmark | gomarkdown, blackfriday, Glamour 内部処理 | CommonMark 準拠、拡張可能、AST が扱いやすい。見出し、リンク、frontmatter などの抽出に向く |
| CLI args | standard `flag` | Cobra, urfave/cli, Kong | MVP は `mdpoke <file>` だけなので標準で十分。サブコマンドが欲しくなったら再検討 |
| Clipboard | small clipboard adapter | atotto/clipboard, golang.design/x/clipboard, OS command | `y` コピーだけを抽象化する。依存は実装時に最小で選ぶ |

#### Bubble Tea vs tview vs tcell vs gocui

Bubble Tea を第一候補にする。

- Bubble Tea: 状態駆動のアプリを作りやすい。`viewer`, `outline`, `search`, `help`, `link focus` のような状態を明示的に扱える。Charmbracelet 系で見た目の統一感も出しやすい
- tview: table, tree, form, modal など完成済み widget が豊富。業務ツールや管理画面には強い。ただし `mdpoke` は既成 widget より本文レンダリングと独自キーマップが主役なので、少し重く感じる可能性がある
- tcell: 低レベルで柔軟。描画や入力を細かく制御できる。ただし viewer のアプリ状態、検索、ヘルプ、レイアウトまで自作量が増える
- gocui: シンプルな pane ベース UI に向く。lazygit でも歴史的に使われている文脈がある。ただし現在の `mdpoke` では、綺麗なポップアップや状態駆動の操作ガイドを作るなら Bubble Tea のほうが進めやすい

#### Glamour vs custom renderer

MVP では Glamour を使うが、レンダリング品質はテストで固定する。

- Glamour: Markdown をターミナル向けに綺麗に表示する近道。テーマも扱いやすい
- goldmark custom renderer: nested list やリンク位置の対応を完全に制御しやすいが、実装量が増える
- 方針: まず Glamour で始める。nested bullet point やリンク位置の扱いで限界が見えたら、goldmark AST から mdpoke 専用 renderer を段階的に作る

#### goldmark vs gomarkdown vs blackfriday

goldmark を第一候補にする。

- goldmark: CommonMark 準拠、AST が拡張しやすい。見出し・リンク・リスト構造を正確に扱いたい `mdpoke` と相性がよい
- gomarkdown: parser / HTML renderer として実績はあるが、今回欲しいのは HTML 出力より TUI 内部状態用の構造解析
- blackfriday: 広く使われてきたが、CommonMark 準拠や外部拡張性、リスト周りの信頼性という点では今回の優先度に合いにくい

#### Risk Management

- Charmbracelet 系に寄せると、見た目と開発体験は良いが、細かい terminal cell 制御は tcell 直書きより弱い
- Glamour の出力と goldmark AST の raw 位置はズレる可能性があるため、リンクフォーカスやクリック判定は段階的に精度を上げる
- nested list は treemd で気になった既知の不満点なので、最初から golden test を持つ
- もし Bubble Tea + Glamour で link hit testing や nested list 表示が難しい場合は、TUI framework は維持したまま renderer だけ自作へ寄せる

### LSP Decision

MVP では LSP は使わない。

理由:

- `mdpoke` は viewer であり、編集、補完、診断、rename などの IDE 的機能を持たない
- 必要な情報は Markdown AST から取れる
- 見出し、リンク、検索、行位置、frontmatter 程度ならローカル解析のほうが単純で速い
- LSP を入れると server lifecycle、workspace 管理、通信、diagnostics などの複雑さが増える
- ファイル検索は fzf に任せるため、workspace indexer としての LSP も不要

将来的に「Markdown の参照解決を複数ファイルで賢くしたい」「diagnostics を出したい」「外部 editor と連携したい」となったら、LSP client/server 連携を再検討する。
それまでは goldmark AST と軽量なローカル index で進める。

### Renderer Decision

treemd と同じ renderer / 表示方式を採用する場合でも、bullet point のネスト表示は個別に検証する。
同じ不具合を避けるため、Markdown rendering はライブラリをそのまま信じ切らず、少なくとも nested list の snapshot / golden test を持つ。

特に確認するケース:

- `-` / `*` / `+` のネスト
- numbered list のネスト
- bullet list と numbered list の混在
- 2 spaces / 4 spaces / tab によるインデント
- 長い bullet item の折り返し
- list item 内の code span / link / emphasis
- list item 内の fenced code block

## Current Architecture

```text
cmd/mdpoke
  main.go
internal/app
  model.go
  model_test.go
internal/markdown
  custom_blocks.go
  document.go
  document_test.go
testdata/fixtures
  comprehensive.md
```

### Main Flow

1. CLI 引数から Markdown ファイルパスを受け取る
2. ファイルを読み込む
3. goldmark で AST を解析し、アウトラインとリンクを抽出する
4. Glamour と custom block renderer で本文表示用の文字列を生成する
6. Bubble Tea の model に document / outline / links / viewport state を渡す
7. TUI イベントループで移動、検索、ジャンプ、コピー、マウス操作、ドラッグテキスト選択を処理する

### Important State

- opened file path
- raw Markdown text
- rendered lines
- outline nodes
- outline visibility
- selected outline index
- current scroll position
- current heading
- search query
- search matches
- detected links
- selected link index
- text selection start / end / drag anchor
- help modal visibility
- help search query
- modal state for copy and internal-jump confirmation

## Data Model Draft

```go
type Document struct {
    Path     string
    Raw      string
    Rendered string
    Outline  []Heading
    Links    []Link
}

type Heading struct {
    Level      int
    Text       string
    Line       int
    Parent     int
    Children   []int
    Collapsed  bool
}

type SearchMatch struct {
    Line  int
    Start int
    End   int
}

type Link struct {
    Text string
    URL  string
    Line int
    StartColumn int
    EndColumn   int
}
```

`Heading.Line` は raw Markdown 上の行番号を基準にする。
Glamour のレンダリング後行番号とはズレる可能性があるため、現在は raw 行と rendered 行の対応表を作り、見出しやリンク付近へスクロールする。

`Link.Line` / `StartColumn` / `EndColumn` も raw Markdown 上の位置を基準にする。
マウスクリック時は raw 行対応と表示テキスト上の水平位置から候補を絞り、同じ行に複数リンクがある場合もできるだけクリック位置に近いリンクを選ぶ。
ドラッグテキスト選択は raw Markdown ではなく rendered 表示位置を基準にし、コピー時は ANSI を除いたプレーンテキストを使う。
rendered 表示の左マージンは読みやすさのための余白として扱い、コピー対象にもハイライト対象にも含めない。

## UX Direction

見た目は派手すぎず、でも触っていて気持ちよいものにする。
最小限であることを美学にする。

- 通常時は本文だけを広く見せる
- アウトラインは右側に必要時だけ表示する
- 見出しレベルをインデントと控えめな色で表現する
- 現在見出し、検索ヒット、選択中ノードは明確に区別する
- フッターは控えめに常時表示し、迷子にならないようにする
- 右下に文脈に応じた小さなガイドを常時表示する
- ガイドは「今押せるキー」を優先し、無関係な操作は表示しすぎない
- `?` のガイドで全キーバインドを検索できる
- `lazygit` / `spf` 的に、画面を見れば単発キーで次の行動が分かる状態を目指す

## Future Ideas

- Back / Forward navigation
- TODO / checkbox 一覧
- code block 一覧
- table of contents only mode
- frontmatter 表示
- ripgrep ベースの全文検索
- GitHub Flavored Markdown 対応強化
- テーマ切り替え
- 設定ファイル
- 複数ファイル間リンク解決
- ドラッグ選択の端末外オートスクロール
- Public release and Homebrew tap distribution. See [Public Release Plan](public-release-plan.md).

## Milestones

### Milestone 1: Minimal Viewer

- [x] Go module 初期化
- [x] `mdpoke <file>` で起動
- [x] Markdown を TUI に表示
- [x] nested bullet point が階層化されて表示されることを確認する
- [x] スクロール、終了、リサイズ対応
- [x] マウスホイールスクロール対応

### Milestone 2: Outline Navigation

- [x] 見出し抽出
- [x] `o` で右側アウトラインを表示 / 非表示
- [x] 見出し選択時の本文追従
- [x] 見出し選択とジャンプ
- [x] 現在見出しのハイライト

### Milestone 3: Search and Polish

- [x] `/` 検索
- [x] `n` / `N` 移動
- [x] リンク検出
- [x] Markdown 内リンク移動
- [x] `tab` / `shift+tab` によるリンクフォーカス
- [x] マウスクリックによるリンクコピー / Markdown 内リンク確認
- [x] `y` でリンクコピー
- [x] ドラッグテキスト選択と `y` による選択テキストコピー
- [x] 文脈に応じた右下ガイド表示
- [x] 検索可能なヘルプポップアップ
- [x] エラー表示と空状態の改善
- [ ] README スクリーンショット整備

## Open Questions

- Markdown レンダリングの正確性と操作性のどちらを優先するか
- 設定ファイルを MVP に含めるか
- `lazygit` / `spf` にどこまでキーマップを寄せるか
- ガイドポップアップの見た目を `spf` にどこまで寄せるか
- 外部 URL を将来的にブラウザで開くか、コピー専用に留めるか
- リンクの表示位置と raw Markdown 位置の対応精度をどこまで上げるか
- テキスト選択コピーは rendered 表示テキストのままがよいか、raw Markdown 由来のテキストも選べるようにするか

## Test Scenarios

Go のテストは table-driven tests を基本にする。
入力 Markdown、操作、期待される outline / links / rendered text / guide items をケース表として並べ、仕様の増加に強い形にする。

- nested bullet point が階層化されて表示される
- nested numbered list が階層化されて表示される
- bullet list と numbered list の混在が崩れない
- 長い list item の折り返し後もインデントが維持される
- list item 内の link をフォーカス、コピー、Markdown 内移動できる
- 本文をドラッグ選択して `y` で表示テキストをコピーできる
- 複数行選択時に左マージンの空白がコピーされない
- コピー完了モーダルが内容を表示せず、外側クリックで閉じる
- ドラッグ選択がリンククリックより優先される
- 検索結果があるときだけ右下ガイドに `n next`, `N prev` が出る
- 通常時の右下ガイドに `o outline` が出る
- ドラッグ選択中の右下ガイドに `y copy text` が出る
- `?` のガイド内検索でキー、操作名、説明文を検索できる

優先して table-driven にする対象:

- Markdown outline extraction
- Markdown link extraction
- nested list rendering / golden output
- search match calculation
- context guide item selection
- key handling state transitions
- mouse text selection and copy priority

## Working Assumptions

- まずは Go + Bubble Tea 系で作る
- MVP は単一 Markdown ファイル viewer
- 編集機能は入れない
- treemd は直接の移植対象ではなく、思想・体験の参考にする
- 見出しペインは常時表示ではなく、必要なときだけ右側に出す
- キーマップは単発キー中心にする
- マウススクロールは MVP に含める
- Markdown 内リンク移動とリンクコピーを実装する
- リンクフォーカスはキーボードで行い、マウスクリックは外部 URL コピーまたは Markdown 内リンク確認を行う
- マウスクリックはリンクをブラウザで開かない
- 本文ドラッグ選択は rendered 表示テキストを対象にし、コピー時は ANSI を除いたプレーンテキストを使う
- ファイル検索は実装せず、fzf など外部ツールとの組み合わせを前提にする
- `?` ガイドは文脈右下ヒントと検索可能ポップアップの両方を実装する

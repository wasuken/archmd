# archmd

AGENTS.md の運用ブロックと ARCHITECTURE.md（現在の構成）で、archmd が無くても文書を維持できます。ARCHITECTURE.md 内には全12節のマーカー行が常駐し、見出しとヒントを持ちます。マーカー行は削除・編集せず、書くことがない節は見出しと本文だけを取り除きます。空の節を埋めるときは、マーカー内の見出しを使って直下に H2 を書きます。変更と同じタスクで該当節を更新し、リポジトリで確認できた内容だけを書きます。

archmd は配置と検証を補助する軽量 CLI。`init` で2ファイルを配置し、`check` で整合を検証、`prompt` でエージェントへの更新依頼を出力します。`split` / `build` は文書の分割・再生成、`remove` は文書と運用ブロックの撤去を補助します。Go 1.22 以上と標準ライブラリだけで動作し、実行時に LLM やネットワークへアクセスしません。

```sh
go build -o archmd .
./archmd init --max-lines 300
./archmd prompt --max-lines 300 --dir .
./archmd check --max-lines 300 --max-section-lines 80
./archmd split
./archmd build
./archmd template
./archmd notices
./archmd version
./archmd help
./archmd remove
```

全コマンドで `--dir <path>` を指定できます（サブコマンドの前後どちらでも可）。`init` は ARCHITECTURE.md に既知の順序で全12節のマーカーを配置し、必須4節（project-structure、high-level-diagram、core-components、project-identification）だけに H2 見出しを置きます。他の8節はマーカーのみです。ARCHITECTURE.md.template は新規作成せず、既存ファイルも読み書きしません。既存の ARCHITECTURE.md、AGENTS.md、CLAUDE.md は変更しません。既存のエージェント設定があれば追記用ブロックを標準出力へ出します。共通ルールは `os.UserConfigDir()/architecture-md/common.md` から読み込みます。

common.md には全プロジェクト共通のルール（例えばドキュメントの言語）を書きます。init が生成するブロックと prompt の出力に含まれます。`archmd help` の Configuration 節で実パスと存在状況（found / not found）を確認できます。init は毎回このパスの案内を標準エラーに出します。

## Examples

`archmd init --write-agents` は既存の AGENTS.md（無ければ CLAUDE.md）にブロックを追記し、既にブロックがあれば何もしません。

ARCHITECTURE.md を更新する例（`<agent>` を利用するエージェントのコマンドに置き換えます）：

```sh
archmd prompt | <agent> && archmd check
archmd prompt | claude -p && archmd check
archmd prompt | codex exec - && archmd check
```

書き込み権限のフラグは各エージェントのドキュメントを確認してください。init は ARCHITECTURE.md を新規作成した場合だけ、骨組みをエージェントに埋めてもらう Next 案内を標準エラーの末尾に出します。

`prompt` は更新依頼、編集モード、init と共通のルール、現在の check 診断、終了条件を英語で標準出力へ出します。ファイルは作成・変更せず、外部コマンドも呼びません。出力を AI エージェントへの指示に利用できます。

`remove` は削除対象と警告を表示する dry-run です。`--yes` を指定すると ARCHITECTURE.md、ARCHITECTURE.md.template、`.architecture/`、AGENTS.md（無ければ CLAUDE.md）の archmd ブロックを削除します。ブロック内の手書きの追記も削除されます。`--keep-architecture-md` で ARCHITECTURE.md を残せます（先頭の生成ヘッダは除去）。ユーザー設定ディレクトリの common.md には触りません。

`.architecture/` がある場合はその中の Markdown を編集し、`archmd build` を実行してください。`check` は生成物とのバイト単位の一致も検証します。終了コードは正常時 0、チェックの ERROR またはファイル操作失敗時 1、引数の誤りで 2 です。

検証: `go test ./...`、`go vet ./...`、`gofmt -l .`。

テンプレートの先頭コメントとエージェント設定の archmd ブロックには、生成した archmd の版が入ります。`check` はテンプレートまたはブロックの版が実行中の archmd より古い場合、`INFO [outdated-template]` / `INFO [outdated-block]` を出します。これらは情報通知で、ERROR が無ければ終了コードは 0 です。版の刻印が無い既存ファイルには通知しません。

更新は自動で行いません。テンプレートの差分は `archmd template` の出力と見比べて手で反映してください。ブロックも必要な変更を確認して手で反映します。`init` は既存テンプレートに触りません。

旧形式の文書は移行せず、作り直してください。`archmd check` は旧形式のマーカーを `WARN [legacy-format]`、残っている ARCHITECTURE.md.template を `WARN [legacy-template]` で通知します。消す前に、コードから再生成できない記述（Key decisions など）を退避してください。その後、次の3コマンドで作り直します。

```sh
archmd remove --yes
archmd init --write-agents
archmd prompt | <agent>
```

ARCHITECTURE.md.template も `remove --yes` で削除されます。init はこのファイルに触りません。prompt は作り直しの手順を追加せず、check 結果から legacy-format と legacy-template の警告を除外します。欠落した節の `WARN [missing-marker]` と版の古いテンプレートの `INFO [outdated-template]` は従来どおり出力します。v0.x の間は破壊的変更を許容します。

## Template changes

- 0.2.0: マーカーに見出しとヒントを入れ、全12節のマーカーを ARCHITECTURE.md 内に常駐させる。.template は新規作成しない。unused-section を missing-marker に置き換え、マーカー行は行数に数えない。旧形式からの移行は作り直し（上記3コマンド）。.template も remove で削除。消す前に、コードから再生成できない記述（Key decisions など）は退避すること。
- 0.1.0: テンプレート先頭に版を刻印。古いテンプレート・ブロックを `check` の INFO で通知。

## ライセンスと出典

コードと同梱テンプレートは [Apache License, Version 2.0](LICENSE) で提供します。
元テンプレートは [timajwilliams/architecture](https://github.com/timajwilliams/architecture) です。
出典、参照コミット、変更内容は [NOTICE](NOTICE) に記録しています。
元の Web・README には MIT 表記がありますが、本プロジェクトでは元リポジトリの LICENSE 本文に従い Apache-2.0 として扱います。

ソースやバイナリの再配布には `LICENSE` と `NOTICE` を同梱してください。
バイナリ単体からも `archmd notices` で出典・変更表示とライセンス全文を取得できます。
生成したテンプレートや更新ルールを再配布するときも、先頭の表示を保持し、次のように全文を同梱してください。
生成先の既存の `LICENSE` を置き換える必要はありません。

```sh
archmd notices > ARCHMD-NOTICES.txt
```

テンプレートに記載するプロジェクト自体のコードに、このライセンスを適用する指定ではありません。
生成物を変更した場合は、該当ファイルに変更した旨を追記してください。

## tips

ドキュメントを日本語にしたい場合にcommon.mdに挿入するPROMPT例:

```
ドキュメントとコメントは基本日本語で書く。ARCHITECTURE.md の見出し(H2)も日本語に翻訳してよい。ただし `<!-- arch:... -->` のIDマーカーは絶対に変更しない。
```

## Assumptions

- 行数は LF で区切った物理行数から、解析されたマーカー行（新旧両形式・未知 ID・重複を含む）を除く。総行数（`lines:`・`--max-lines`）と節ごとの行数（`--max-section-lines`）の両方に適用し、フェンス内のマーカー例は通常の行として数える。末尾 LF は余分な空行に数えず、空ファイルは 0 行とする。解析時は CRLF と CR を LF に正規化するが、drift は元のバイト列で比較する。
- 行数制限は正の整数のみ受け付け、省略した総行数制限は無制限とする。引数エラー以外のファイル操作失敗は終了コード 1 とし、check では io コードで報告する。
- マーカーは行全体が指定形式に一致する非空 ID とし、新形式の最初の ` | ` より前を ID として扱う。heading・hint は検証しない。フェンス付きコードブロック内のマーカーと H2 は解析対象外とする。H2 は前後の空白を除いた `##` 単独、または `##` の直後に半角空白かタブがある行とする。マーカー後の空行は読み飛ばし、最初の非空行が H2 なら正常、次のマーカーまたは EOF なら空の節として正常とする。空の節は必須 ID でも許可する。
- AGENTS.md と CLAUDE.md が両方存在する場合、init のブロック有無は両方を調べ、check の言及検出は AGENTS.md を優先する。
- build は直下の通常ファイルで拡張子が `.md` のものだけをファイル名順に結合する。`00-header.md` がある場合は split の形式とみなし、生成ヘッダや区切りを加えず、改行・末尾改行の有無を含めてバイト列を保持する。これが無い手動構成では従来どおり末尾 LF を一つに揃え、ファイル間に空行を一つ置き、生成ヘッダを付ける。空ディレクトリでは生成ヘッダだけを出力する。
- split は空のヘッダ部も 00-header.md に保存し、空の節も各1ファイルに分割する。空行は元の範囲に保持する。既存の生成ヘッダも保持し、split → build で元のバイト列と一致させる。重複 ID と Windows を含む安全なファイル名にできない ID は書き込み前にエラーとする。
- 既知 ID に異なる H2 が対応していても追加の診断は出さない。common.md の欠落以外の読み込みエラーは無視せず報告する。
- prompt は check に ERROR があっても指示文の出力に成功すれば終了コード 0 とし、--max-lines は埋め込む check にも適用する。check 出力からは指定された診断行と lines 行を含め、summary 行は含めない。
- remove は複数の正常なブロックを全て除去し、入れ子・余分なマーカーも不正として全対象の変更前に中止する。マーカーを含む行を両端とし、直後の空白文字だけの行も空行として一つだけ除去する。マーカーが無い設定ファイルは内容が空白だけでも保持する。
- remove の --keep-architecture-md は生成ヘッダ以外のバイト列と改行を保持する。対象直下のシンボリックリンクと想定外のファイル種別は、対象外への書き込みを避けるため全対象の変更前にエラーとする。OS の削除・書き込み失敗時のロールバックは行わない。
- init の Next 案内は ARCHITECTURE.md の新規作成を含む処理が正常終了した場合だけ出す。common.md のパス解決・存在確認で欠落以外のエラーがあれば、help と init は終了コード 1 で報告する。
- init の標準出力は運用ブロックだけという従来の契約を維持し、本文の指定変更に限って golden を更新する（本文変更と変更前のバイト列維持は両立しない）。
- remove は ARCHITECTURE.md.template も削除する。--keep-architecture-md を指定した場合もテンプレートは削除する。

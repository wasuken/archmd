#!/usr/bin/env bash
# archmd のリリース: パッチ版を1つ上げ、AGENTS.md のブロックを新しい版で作り直し、
# コミット・タグ・push まで一括で行う。
# 使い方: ./release.sh            (push 前に Y/n で確認)
#         ./release.sh --no-push  (ローカルでコミットとタグまで)
set -euo pipefail

PUSH=1
[[ "${1:-}" == "--no-push" ]] && PUSH=0

REMOTE=origin
BRANCH=main
BIN="$(mktemp -d)/archmd"
TAG_CREATED=""

die() {
  echo "ERROR: $*" >&2
  exit 1
}

# 途中で失敗したら、ローカルに作ったタグを消す(push 前なら影響はローカルだけ)
cleanup() {
  local code=$?
  if [[ $code -ne 0 && -n "$TAG_CREATED" ]]; then
    git tag -d "$TAG_CREATED" >/dev/null 2>&1 || true
    echo "失敗したのでローカルタグ $TAG_CREATED を削除した。コミットが残っていれば git reset --hard HEAD~1 で戻せる" >&2
  fi
}
trap cleanup EXIT

cd "$(git rev-parse --show-toplevel)"

# --- 前提チェック ---
[[ "$(git branch --show-current)" == "$BRANCH" ]] || die "$BRANCH ブランチで実行して"
[[ -z "$(git status --porcelain)" ]] || die "作業ツリーが汚れている"
git fetch -q "$REMOTE" --tags
[[ "$(git rev-parse HEAD)" == "$(git rev-parse "$REMOTE/$BRANCH")" ]] || die "$REMOTE/$BRANCH と HEAD がずれている。pull か push を先に"

# --- 次の版を決める(最新タグのパッチ +1) ---
LAST="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"
[[ -n "$LAST" ]] || die "vX.Y.Z 形式のタグが無い"
IFS=. read -r MAJ MIN PAT <<<"${LAST#v}"
NEXT="v${MAJ}.${MIN}.$((PAT + 1))"
VER="${NEXT#v}"
echo "版: $LAST -> $NEXT"

# --- 検証 ---
go vet ./...
[[ -z "$(gofmt -l .)" ]] || die "gofmt が必要なファイルがある: $(gofmt -l .)"
go test ./...

# --- 仮タグを打ち、クリーンな状態でビルドして版を焼き込む ---
git tag "$NEXT"
TAG_CREATED="$NEXT"
go build -o "$BIN" .
[[ "$("$BIN" version)" == "$VER" ]] || die "ビルドした版が $("$BIN" version)(期待: $VER)"

# --- AGENTS.md のブロックを作り直す(ブロック外の行はそのまま残す) ---
# 注意: ブロックには ~/.config/architecture-md/common.md の内容も入る
if [[ -f AGENTS.md ]]; then
  awk '/<!-- archmd:start/{skip=1} !skip{print} /<!-- archmd:end -->/{skip=0}' AGENTS.md >AGENTS.md.tmp
  mv AGENTS.md.tmp AGENTS.md
  [[ -s AGENTS.md ]] || rm AGENTS.md # ブロックだけのファイルだったら消して、init に作らせる
fi
"$BIN" init --write-agents >/dev/null
grep -q "<!-- archmd:start v${VER} -->" AGENTS.md || die "AGENTS.md のブロックが v${VER} になっていない"

# --- コミットし、タグを付け直す ---
git add AGENTS.md
if git diff --cached --quiet; then
  echo "AGENTS.md に差分なし(コミットは作らない)"
else
  git commit -q -m "release: ${NEXT}"
  git tag -f "$NEXT" >/dev/null
fi

# --- 付け直したタグでもう一度ビルドし、check が 0 ERROR / 0 WARN / 0 INFO か確認 ---
go build -o "$BIN" .
[[ "$("$BIN" version)" == "$VER" ]] || die "再ビルドの版が $("$BIN" version)(期待: $VER)"
OUT="$("$BIN" check)"
echo "$OUT"
grep -q '^summary: 0 ERROR, 0 WARN, 0 INFO$' <<<"$OUT" || die "check に診断が残っている"

# --- push ---
if [[ $PUSH -eq 0 ]]; then
  TAG_CREATED="" # 成功したのでタグは残す
  echo "ローカルで $NEXT を作成した(push はしていない)"
  exit 0
fi

read -r -p "$REMOTE に $BRANCH と $NEXT を push する? [Y/n] " ans
if [[ "${ans:-Y}" =~ ^[Yy]$ ]]; then
  git push -q "$REMOTE" "$BRANCH"
  git push -q "$REMOTE" "$NEXT"
  TAG_CREATED=""
  echo "リリース完了: $NEXT"
else
  TAG_CREATED=""
  echo "push は中止した。ローカルにコミットとタグ $NEXT は残っている"
  echo "やり直すなら: git tag -d $NEXT && git reset --hard $REMOTE/$BRANCH"
fi

#!/usr/bin/env bash
# Location: ~/rename-to-skoll.sh — repo-র বাইরে রাখবে (যেমন ~/), skoll-cli-র ভিতর থেকে চালাবে
# skoll-cli  ->  skoll-cli   (repo name)
# skoll      ->  skoll       (command / binary / config dir / env var)
#
# চালানো:
#   cd skoll-cli
#   bash ~/rename-to-skoll.sh
#
# GitHub-এ repo-র নাম বদলানো script-এর কাজ না — push করার আগে
# github.com/masudibnbelat/skoll-cli/settings-এ গিয়ে Repository name
# skoll-cli -> skoll-cli করে নেবে, নাহলে push fail করবে।
# Script-টা দুবার চালালেও ক্ষতি নেই (দ্বিতীয়বার আর কিছু বদলাবে না)।
set -euo pipefail

# 0. repo root check
[ -f main.go ] && [ -f go.mod ] || { echo "STOP: skoll-cli ফোল্ডারের ভিতর থেকে চালাও (main.go / go.mod পাওয়া যায়নি)"; exit 1; }

echo "== 1/5 ফাইলের নাম বদলাচ্ছি"
if [ -f skoll_test.go ]; then
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then git mv skoll_test.go skoll_test.go; else mv skoll_test.go skoll_test.go; fi
fi
rm -f skoll skoll.exe skoll skoll.exe

echo "== 2/5 লেখার ভিতরে নাম বদলাচ্ছি (skoll-cli -> skoll-cli, skoll -> skoll)"
if sed --version >/dev/null 2>&1; then SED_INPLACE=(-i); else SED_INPLACE=(-i ''); fi
find . -type f -not -path './.git/*' -not -path './dist/*' -print0 |
while IFS= read -r -d '' f; do
  grep -Iq . "$f" || continue
  sed "${SED_INPLACE[@]}" \
    -e 's/skoll-cli/skoll-cli/g' \
    -e 's/SKOLL/SKOLL/g' \
    -e 's/Skoll/Skoll/g' \
    -e 's/skoll/skoll/g' \
    "$f"
done

echo "== 3/5 যাচাই — 'skoll' আর কোথাও থাকলো কি না"
LEFT=$(grep -rni 'skoll' --exclude-dir=.git . 2>/dev/null | wc -l || true)
LEFT=$(echo "$LEFT" | tr -d ' ')
echo "   বাকি: $LEFT"
if [ "$LEFT" != "0" ]; then grep -rni 'skoll' --exclude-dir=.git . || true; fi

echo "== 4/5 পুরনো cache/keystore ফোল্ডার সরাচ্ছি (~/.skoll -> ~/.skoll)"
if [ -d "$HOME/.skoll" ] && [ ! -d "$HOME/.skoll" ]; then
  mv "$HOME/.skoll" "$HOME/.skoll"
  echo "   moved: ~/.skoll -> ~/.skoll (keystore, JDK, Flutter, Gradle cache সব একসাথে গেল — আবার download লাগবে না)"
elif [ -d "$HOME/.skoll" ] && [ -d "$HOME/.skoll" ]; then
  echo "   SKIP: ~/.skoll আগেই আছে — ~/.skoll নিজে দেখে মুছবে কি রাখবে ঠিক করো"
fi
rm -f "$HOME/.local/bin/skoll"

echo "== 5/5 git remote বদলাচ্ছি"
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  OLD=$(git remote get-url origin 2>/dev/null || true)
  if [ -n "$OLD" ]; then
    git remote set-url origin https://github.com/masudibnbelat/skoll-cli.git
    echo "   remote: $OLD -> https://github.com/masudibnbelat/skoll-cli.git"
  fi
fi

if command -v go >/dev/null 2>&1; then
  echo "== build check"
  go build -o skoll .
  ./skoll --version
else
  echo "== go পাওয়া যায়নি — PC-তে 'go build -o skoll .' চালিয়ে দেখবে"
fi

echo ""
echo "== DONE. এরপর:"
echo "  1. GitHub-এ repo rename করো: github.com/masudibnbelat/skoll-cli/settings -> skoll-cli"
echo "  2. cd .. && mv skoll-cli skoll-cli && cd skoll-cli"
echo "  3. git add -A && git commit -m \"Rename skoll-cli to skoll-cli (command: skoll)\" && git push"
echo "  4. Tag: 'git tag | tail -3' দেখো — v1.0.5 আগেই থাকলে পরের release v1.0.6, না থাকলে v1.0.5"
echo "  5. নতুন install: curl -fsSL https://raw.githubusercontent.com/masudibnbelat/skoll-cli/main/install.sh | bash"

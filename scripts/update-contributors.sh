#!/usr/bin/env bash
set -euo pipefail

REPO="eugenioenko/ttt"
OUTPUT="internal/app/contributors.json"

contributors=$(gh api "repos/$REPO/contributors?per_page=100" --paginate \
  -q '.[] | select(.type == "User") | .login' | jq -R . | jq -s .)

reporters=$(gh api "repos/$REPO/issues?state=all&per_page=100" --paginate \
  -q '.[] | select(.pull_request == null and .user.type == "User") | .user.login' |
  sort | uniq -c | sort -k1,1nr -k2,2 | awk '{print $2}' | jq -R . | jq -s .)

jq -n --argjson c "$contributors" --argjson r "$reporters" \
  '{contributors: $c, reporters: ($r - $c)}' > "$OUTPUT"

echo "$OUTPUT: $(jq '.contributors | length' "$OUTPUT") contributors, $(jq '.reporters | length' "$OUTPUT") reporters"

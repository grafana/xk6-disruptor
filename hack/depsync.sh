#!/usr/bin/env bash

set -eo pipefail

tmpdir=$(mktemp -d)

go run ./hack/depsync 2> "$tmpdir/deps.log" > "$tmpdir/sync.sh"
bash "$tmpdir/sync.sh" 2>&1 | tee -a "$tmpdir/go-get.log"
go mod tidy

cat <<EOF > depsync-pr-body.txt
This automated PR aligns the following dependency mismatches with k6 core:
\`\`\`
$(cat "$tmpdir/deps.log")
\`\`\`

The following commands were run to align the dependencies:
\`\`\`shell
$(cat "$tmpdir/sync.sh")
\`\`\`

And produced the following output:
\`\`\`
$(cat "$tmpdir/go-get.log")
\`\`\`

Due to a limitation of GitHub Actions, to run CI checks for this PR, close it and reopen it again.
EOF

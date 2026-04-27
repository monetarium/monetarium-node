#!/bin/bash
#
# Tag all Go submodules with annotated tags.
#
# Usage: ./tag_submodules.sh [VERSION] [MESSAGE]
#   VERSION defaults to v1.0.19
#   MESSAGE defaults to "fix: remove non-functional HTTP seeder"

set -euo pipefail

VERSION="${1:-v1.0.19}"
MESSAGE="${2:-fix: remove non-functional HTTP seeder}"

find . -name "go.mod" -not -path "./vendor/*" | while read modfile; do
    module=$(dirname "$modfile" | sed 's|^\./||')

    # Skip root module (go.mod in current directory)
    if [ "$module" = "." ]; then
        continue
    fi

    tag="${module}/${VERSION}"
    echo "Creating annotated tag: $tag"
    git tag -a "$tag" -m "$MESSAGE"
done

echo ""
echo "All annotated submodule tags created. Push them with:"
echo "git push origin --tags"

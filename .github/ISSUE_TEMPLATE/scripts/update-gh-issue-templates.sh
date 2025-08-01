# If run on OSX, default bash < 4 and does not support declare -A, also bsd awk does not support multiline strings
#!/usr/bin/env bash

set -euo pipefail

# Create an array of labels so they can be added in to the issue templates
declare -a LABELS

for README in $(find ./docs/sources/reference/components -name '*.md' -and -not -name '*index.md' -print0); do
    # The find ends up with an empty string in some OSes
    if [[ -z "${README}" ]]; then
        continue
    fi
    FILENAME=${README##*/}
    LABEL_NAME="${FILENAME%.*}"
    TYPE=$(echo "${LABEL_NAME}" | cut -f1 -d '.' )
    
    if (( "${#LABEL_NAME}" > 50 )); then
        echo "'${LABEL_NAME}' exceeds GitHubs 50-character limit on labels, skipping"
        continue
    fi
    LABELS+=("${LABEL_NAME}")
done

content="$(printf -- "`printf "      - %s\n" "${LABELS[@]}"`")\n      # End components list"

# replace the text in the .github/ISSUE_TEMPLATE/*.yaml files between "# Start components list" and "# End components list" with the LABELS array using awk
for TEMPLATE in $(find .github/ISSUE_TEMPLATE -name '*.yaml'); do
    echo "Updating ${TEMPLATE} with component labels"
    awk -v content="${content}" '
        BEGIN { in_section = 0 }
        /# Start components list/ { in_section = 1; print; next }
        /# End components list/ { in_section = 0; print content; next }
        !in_section { print }
    ' "${TEMPLATE}" > "${TEMPLATE}.tmp" && mv "${TEMPLATE}.tmp" "${TEMPLATE}"
    echo "Updated ${TEMPLATE} successfully"
done

echo "Issue templates updated successfully."


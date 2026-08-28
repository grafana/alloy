# If run on OSX, default bash < 4 and does not support declare -A, also bsd awk does not support multiline strings

set -euo pipefail

# Create an array of component names so they can be added in to the issue templates
declare -a LIST

for README in $(find ./docs/sources/reference/components -name '*.md' ! -name '*index.md'); do
    # The find ends up with an empty string in some OSes
    if [[ -z "${README}" ]]; then
        continue
    fi
    FILENAME=${README##*/}
    COMPONENT_NAME="${FILENAME%.*}"

    if (( "${#COMPONENT_NAME}" > 50 )); then
        echo "'${COMPONENT_NAME}' exceeds GitHubs 50-character limit on labels, skipping"
        continue
    fi

    LIST+=("${COMPONENT_NAME}")
done

# Ensure LIST array has been populated
if [ ${#LIST[@]} -eq 0 ]; then
    echo "No components were found. Exiting script."
    exit 1
fi

IFS=$'\n' LIST=($(LC_ALL=C sort <<<"${LIST[*]}"))
# Reset IFS to default
unset IFS

# Format the list properly
LABELS_LIST="$(printf "      - %s\n" "${LIST[@]}")"
# Append the # End components list comment to the end of the list
LABELS_LIST="${LABELS_LIST}\n      # End components list"

# Create a temporary file with the content
echo -e "${LABELS_LIST}" > /tmp/labels_content.txt

# Process the templates
for TEMPLATE in $(find .github/ISSUE_TEMPLATE -name '*.yaml'); do
    echo "Updating ${TEMPLATE} with component labels"

    # Use awk to replace the section in each template
    awk -v labels_file="/tmp/labels_content.txt" '
        BEGIN { in_section = 0 }
        /# Start components list/ { in_section = 1; print; next }
        /# End components list/ { in_section = 0; while ((getline line < labels_file) > 0) print line; next }
        !in_section { print }
    ' "${TEMPLATE}" > "${TEMPLATE}.tmp" && mv "${TEMPLATE}.tmp" "${TEMPLATE}"

    echo "Updated ${TEMPLATE} successfully"
done

echo "Issue templates updated successfully."


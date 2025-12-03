set -euo pipefail

declare -A COLORS

COLORS["beyla"]="#483C32"
COLORS["database_observability"]="#6666FF"
COLORS["discovery"]="#1A8CFF"
COLORS["faro"]="#50C878"
COLORS["local"]="#FF794D"
COLORS["loki"]="#B30059"
COLORS["mimir"]="#F9DE22"
COLORS["otelcol"]="#800080"
COLORS["prometheus"]="#E91B7B"
COLORS["pyroscope"]="#336600"
COLORS["remote"]="#4B5320"

FALLBACK_COLOR="#999999"

for README in $(find ./docs/sources/reference/components -name '*.md' ! -name '*index.md'); do
    # The find ends up with an empty string in some OSes
    if [[ -z "${README}" ]]; then
        continue
    fi
    FILENAME=${README##*/}
    LABEL_NAME="c/${FILENAME%.*}"
    TYPE=$(echo "${FILENAME}" | cut -f1 -d '.' )

    if (( "${#LABEL_NAME}" > 50 )); then
        echo "'${LABEL_NAME}' exceeds GitHubs 50-character limit on labels, skipping"
        continue
    fi

    echo "Creating label '${LABEL_NAME}' with color ${COLORS["${TYPE}"]:-${FALLBACK_COLOR}}"
    gh label create "${LABEL_NAME}" -c "${COLORS["${TYPE}"]:-${FALLBACK_COLOR}}" --force
done

echo "Component labels updated successfully."

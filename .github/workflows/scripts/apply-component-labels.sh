set -euo pipefail

if [[ -z "${ISSUE:-}" || -z "${BODY:-}" ]]; then
  echo "Missing one of ISSUE, or BODY, please ensure all are set."
  exit 0
fi

LABELS=""

COMPONENTS_SECTION_START=$( (echo "${BODY}" | grep -n '### Component(s)' | awk '{ print $1 }' | grep -oE '[0-9]+') || echo '-1' )
BODY_COMPONENTS=""

if [[ "${COMPONENTS_SECTION_START}" != '-1' ]]; then
  BODY_COMPONENTS=$(echo "${BODY}" | sed -n $((COMPONENTS_SECTION_START+2))p)
fi

for COMPONENT in ${BODY_COMPONENTS}; do
  # Components are delimited by ', ' and the for loop separates on spaces, so remove the extra comma.
  COMPONENT=${COMPONENT//,/}
  LABEL_NAME="c/${COMPONENT}"

  if (( "${#LABEL_NAME}" > 50 )); then
    echo "'${LABEL_NAME}' exceeds GitHubs 50-character limit on labels, skipping"
    continue
  fi

  if [[ -n "${LABELS}" ]]; then
    LABELS+=","
  fi
  LABELS+="${LABEL_NAME}"
done

if [[ -n "${LABELS}" ]]; then
  echo "Adding the following labels: ${LABELS}"
  gh issue edit "${ISSUE}" --add-label "${LABELS}" || true
else
  echo "No labels were found to add"
fi

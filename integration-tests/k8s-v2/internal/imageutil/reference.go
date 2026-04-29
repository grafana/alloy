package imageutil

import (
	"fmt"
	"strings"
)

func SplitReference(imageRef string) (string, string, error) {
	if imageRef == "" {
		return "", "", fmt.Errorf("image reference is empty")
	}
	if strings.Contains(imageRef, "@") {
		return "", "", fmt.Errorf("digest image references are not supported, use repository:tag")
	}
	lastSlash := strings.LastIndex(imageRef, "/")
	lastColon := strings.LastIndex(imageRef, ":")
	if lastColon <= lastSlash || lastColon == len(imageRef)-1 {
		return "", "", fmt.Errorf("missing image tag in %q", imageRef)
	}
	return imageRef[:lastColon], imageRef[lastColon+1:], nil
}

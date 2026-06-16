//go:build !slim

package flowcmd

// Register grafana-agent static-mode integrations for the full build. Excluded
// from `slim` builds to drop their heavy dependency trees (vmware/azure/gcp
// exporters and friends). slim has no counterpart file: the registry is simply
// left empty, which is safe because slim only runs native Alloy components.
import _ "github.com/grafana/alloy/internal/static/integrations/install"

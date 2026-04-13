package main

import (
	"log"

	"github.com/grafana/alloy/configprovider/opampprovider"
	"github.com/spf13/cobra"
)

// registerOpAMPValidationURIStash wires PersistentPreRun on the otel command so the
// --config/--set resolver URI list is copied for opampprovider merged-remote validation.
func registerOpAMPValidationURIStash(otelCmd *cobra.Command) {
	prev := otelCmd.PersistentPreRunE
	otelCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		stashOpAMPValidationURIs(cmd)
		if prev != nil {
			return prev(cmd, args)
		}
		return nil
	}
}

func stashOpAMPValidationURIs(cmd *cobra.Command) {
	cfgURIs, setURIs, err := otelConfigResolverParts(cmd)
	if err != nil {
		log.Printf("opampprovider: stash resolver URIs skipped: %v", err)
		opampprovider.SetStashedResolverURIsFromCLI(nil)
		return
	}
	uris := append(append([]string(nil), cfgURIs...), setURIs...)
	opampprovider.SetStashedResolverURIsFromCLI(uris)
}

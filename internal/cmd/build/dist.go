//go:build mage

package main

func AlloyDarwinArm64() error {
	return buildAlloy(
		"darwin",
		"arm64",
		"netgo builtinassets",
		"",
	)
}

func AlloyDarwinAmd64() error {
	return buildAlloy(
		"darwin",
		"amd64",
		"netgo builtinassets",
		"",
	)
}

func AlloyLinuxs390x() error {
	return buildAlloy(
		"linux",
		"s390x",
		"netgo builtinassets promtail_journal_enabled",
		"",
	)
}

func AlloyLinuxPpc64le() error {
	return buildAlloy(
		"linux",
		"ppc64le",
		"netgo builtinassets promtail_journal_enabled",
		"",
	)
}

func AlloyLinuxArm64() error {
	return buildAlloy(
		"linux",
		"ppc64le",
		"netgo builtinassets promtail_journal_enabled",
		"",
	)
}

func AlloyLinuxAmd64() error {
	return buildAlloy(
		"linux",
		"amd64",
		"netgo builtinassets promtail_journal_enabled",
		"",
	)
}

func AlloyLinuxAmd64BoringCrypto() error {
	return buildAlloyFull(
		"linux",
		"amd64",
		"netgo builtinassets promtail_journal_enabled",
		"",
		"boringcrypto",
	)
}

func AlloyWindowsAmd64() error {
	return buildAlloy(
		"windows",
		"amd64",
		"builtinassets",
		"",
	)
}

func AlloyFreebsdAmd64() error {
	return buildAlloy(
		"freebsd",
		"amd64",
		"netgo builtinassets",
		"",
	)
}

package memorylimiter

import "github.com/alecthomas/units"

func memoryLimitToUpstream(b units.Base2Bytes) uint32 { return uint32(b / units.Mebibyte) }

func memoryLimitFromUpstream(mib uint32) units.Base2Bytes {
	return units.Base2Bytes(mib) * units.Mebibyte
}

func memorySpikeLimitToUpstream(b units.Base2Bytes) uint32 { return uint32(b / units.Mebibyte) }

func memorySpikeLimitFromUpstream(mib uint32) units.Base2Bytes {
	return units.Base2Bytes(mib) * units.Mebibyte
}

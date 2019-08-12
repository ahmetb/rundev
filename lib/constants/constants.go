package constants

const (
	HdrRundevChecksum             = `rundev-checksum`
	HdrRundevPatchPreconditionSum = `rundev-apply-if-checksum`
	HdrRundevClientSecret         = `rundev-client-secret`

	MimeDumbRepeat       = `application/vnd.rundev.repeat`
	MimeChecksumMismatch = `application/vnd.rundev.checksumMismatch+json`
	MimePatch            = `application/vnd.rundev.patch+tar`
	MimeProcessError     = `application/vnd.rundev.procError+json`

	WhiteoutDeleteSuffix = ".whiteout.del"
)

package constants

const (
	HdrRundevChecksum             = `rundev-checksum`
	HdrRundevPatchPreconditionSum = `rundev-apply-if-checksum`

	MimeDumbRepeat       = `application/vnd.rundev.repeat`
	MimeChecksumMismatch = `application/vnd.rundev.checksumMismatch+json`
	MimePatch            = `application/vnd.rundev.patch+tar`

	WhiteoutDeleteSuffix = ".whiteout.del"
)

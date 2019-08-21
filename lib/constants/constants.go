// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

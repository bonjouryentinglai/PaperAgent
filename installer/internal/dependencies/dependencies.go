// SPDX-License-Identifier: MIT
package dependencies

import "github.com/bonjouryentinglai/paper-agent/installer/internal/releasebundle"

type Dependency struct {
	Name     string
	Filename string
	Artifact releasebundle.Artifact
}

var (
	XOVI = Dependency{
		Name:     "XOVI",
		Filename: "xovi-aarch64.tar.gz",
		Artifact: releasebundle.Artifact{
			URL:    "https://github.com/asivery/rm-xovi-extensions/releases/download/v19-23052026/xovi-aarch64.tar.gz",
			SHA256: "32d64d1262ddc984e3235c7d0340a398fe6d5b3efa6a979865f5977b32630d27",
			Bytes:  6163890,
		},
	}
	AppLoad = Dependency{
		Name:     "AppLoad",
		Filename: "appload-aarch64.zip",
		Artifact: releasebundle.Artifact{
			URL:    "https://github.com/asivery/rm-appload/releases/download/v0.5.3/appload-aarch64.zip",
			SHA256: "032e3f2c57a004aba4425894758e4b542c67590efd222e3b3d5141124c45e84d",
			Bytes:  4118708,
		},
	}
	RMShot = Dependency{
		Name:     "rm-shot",
		Filename: "rm-shot-aarch64.so",
		Artifact: releasebundle.Artifact{
			URL:    "https://github.com/rmitchellscott/rm-shot/releases/download/v1.2.0/rm-shot-aarch64.so",
			SHA256: "1526fb4a0582c26801afc2861eeafd651286ef0f23bbdb372700dd025f66f786",
			Bytes:  75968,
		},
	}
	Node = Dependency{
		Name:     "Node.js",
		Filename: "node-v22.22.3-linux-arm64.tar.gz",
		Artifact: releasebundle.Artifact{
			URL:    "https://nodejs.org/download/release/v22.22.3/node-v22.22.3-linux-arm64.tar.gz",
			SHA256: "cc8bc82b2dd0b595c3b95a4c3c9c8c350907cff011afbdee3d1379e812e1e3e3",
			Bytes:  56757052,
		},
	}
	TripleTap = Dependency{
		Name:     "xovi-tripletap",
		Filename: "xovi-tripletap-869497aa61435448bf0077fbf75fb264dcba92c5.tar.gz",
		Artifact: releasebundle.Artifact{
			URL:    "https://github.com/rmitchellscott/xovi-tripletap/archive/869497aa61435448bf0077fbf75fb264dcba92c5.tar.gz",
			SHA256: "e89352f1a9626dc723946365c35ffd3732052e00a89e1c198bc5d0e145bb82f3",
			Bytes:  77209,
		},
	}
)

func All() []Dependency {
	return []Dependency{XOVI, AppLoad, RMShot, Node, TripleTap}
}

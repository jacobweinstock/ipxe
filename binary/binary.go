package binary

import _ "embed"

//go:embed ipxe.efi
var IpxeEFI []byte

//go:embed undionly.kpxe
var Undionly []byte

//go:embed snp.efi
var SNP []byte

// Files are the ipxe binaries to be embedded.
var Files = map[string][]byte{
	"undionly.kpxe": Undionly,
	"ipxe.efi":      IpxeEFI,
	"snp.efi":       SNP,
	// "snp-nolacp.efi": snpNolacp,
}

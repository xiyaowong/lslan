package scan

import "strings"

func init() {
	ouiTable = make(map[string]string, 16384)
	for line := range strings.SplitSeq(ouiDB, "\n") {
		if prefix, name, ok := strings.Cut(line, ":"); ok && len(prefix) == 6 && name != "" {
			ouiTable[prefix] = name
		}
	}
}

var ouiTable map[string]string

// OUIVendor resolves a MAC address (any common separator format) to a
// vendor name via the embedded OUI table; "" when unknown.
func OUIVendor(mac string) string {
	var nib [6]byte // first three octets as nibble values
	n := 0
	for i := 0; i < len(mac) && n < 6; i++ {
		c := mac[i]
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			continue
		}
		nib[n] = v
		n++
	}
	if n < 6 {
		return ""
	}
	const hexd = "0123456789abcdef"
	// U/L bit is bit 1 of the first octet, i.e. of its low nibble.
	if nib[1]&0x02 != 0 {
		// Locally administered address (second bit of the first octet):
		// never matches a registry OUI, skip the lookup.
		return ""
	}
	return ouiTable[string([]byte{
		hexd[nib[0]], hexd[nib[1]], hexd[nib[2]],
		hexd[nib[3]], hexd[nib[4]], hexd[nib[5]],
	})]
}

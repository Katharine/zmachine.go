package zmachine

import "fmt"

type ZString struct {
	chars []byte
	z     *ZMachine
}

// Creates a ZString by reading from the memory of machine starting at address
// and continuing until the end-of-string marker is reached.
func ZStringFromMemory(machine *ZMachine, address int) ZString {
	s := make([]byte, 0)
	for {
		word := machine.number(address)
		s = append(s, byte((word&0x7C00)>>10), byte((word&0x03E0)>>5), byte(word&0x001F))
		if word&0x8000 != 0 {
			break
		}
		address += 2
	}
	return ZString{s, machine}
}

func (this *ZString) ZSCIIString() ZSCIIString {
	return this.toZSCII(true)
}

func (this *ZString) Chars() []byte {
	return this.chars
}

func (this *ZString) Size() int {
	return len(this.chars)
}

func (this *ZString) toZSCII(expand bool) ZSCIIString {
	zscii := make([]byte, 0)
	alphabet := 0
	last_alphabet := 0
	temporary := false

	a0 := [26]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'}
	a1 := [26]byte{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	var a2 [26]byte
	if this.z.version == 1 {
		a2 = [26]byte{' ', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', ',', '!', '?', '_', '#', '\'', '"', '/', '\\', '<', '-', ':', '(', ')'}
	} else {
		a2 = [26]byte{' ', '\n', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', ',', '!', '?', '_', '#', '\'', '"', '/', '\\', '-', ':', '(', ')'}
	}
	alphabets := [3][26]byte{a0, a1, a2}

	for i := 0; i < len(this.chars); i++ {
		zchar := this.chars[i]
		if (this.z.version < 3 && zchar == 2) || zchar == 4 {
			last_alphabet = alphabet
			alphabet = (alphabet + 1) % 3
			temporary = (zchar == 2 || this.z.version >= 3)
		} else if (this.z.version < 3 && zchar == 3) || zchar == 5 {
			last_alphabet = alphabet
			alphabet = (alphabet + 2) % 3
			temporary = (zchar == 3 || this.z.version >= 3)
		} else {
			if zchar == 0 {
				zscii = append(zscii, 32)
			} else if zchar == 1 && this.z.version == 1 {
				zscii = append(zscii, 13)
			} else if zchar <= 3 && this.z.version >= 2 && expand {
				if zchar == 1 || this.z.version >= 3 {
					i += 1
					offset := int(this.chars[i])
					zchar_abbr := this.z.zString(int(this.z.number(int(this.z.abbreviationStart)+2*((32*(int(zchar)-1))+offset))), true)
					abbr := zchar_abbr.toZSCII(false)
					zscii = append(zscii, abbr.Bytes()...)
				}
			} else if zchar == 6 && alphabet == 2 {
				if i+2 >= len(this.chars) {
					break
				}
				high := this.chars[i+1] << 5
				low := this.chars[i+2]
				i += 2
				result := high | low
				zscii = append(zscii, result)
			} else if zchar >= 6 {
				index := zchar - 6
				result := alphabets[alphabet][index]
				zscii = append(zscii, result)
			} else {
				panic(fmt.Sprintf("Unknown Z-character %d!", zchar))
			}
			if temporary {
				alphabet = last_alphabet
			}
		}
	}

	return ZSCIIString{zscii, this.z}
}

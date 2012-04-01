package zmachine

var extraCharacters = [69]rune{
	'ä', 'ö', 'ü',
	'Ä', 'Ö', 'Ü',
	'ß', '«', '»',
	'ë', 'ï', 'ÿ',
	'Ë', 'Ï', 'á',
	'é', 'í', 'ó',
	'ú', 'ý', 'Á',
	'É', 'Í', 'Ó',
	'Ú', 'Ý', 'à',
	'è', 'ì', 'ò',
	'ù', 'À', 'È',
	'Ì', 'Ò', 'Ù',
	'â', 'ê', 'î',
	'ô', 'û', 'Â',
	'Ê', 'Î', 'Ô',
	'Û', 'å', 'Å',
	'ø', 'Ø', 'ã',
	'ñ', 'õ', 'Ã',
	'Ñ', 'Õ', 'æ',
	'Æ', 'ç', 'Ç',
	'þ', 'ð', 'Þ',
	'Ð', '£', 'œ',
	'Œ', '¡', '¿',
}

type ZSCIIString struct {
	bytes []byte
	z     *ZMachine
}

func (this *ZSCIIString) Bytes() []byte {
	return this.bytes
}

func (this *ZSCIIString) Size() int {
	return len(this.bytes)
}

func (this *ZSCIIString) String() string {
	s := make([]rune, len(this.bytes))
	for i, char := range this.bytes {
		switch {
		case char >= 32 && char <= 126:
			s[i] = rune(char)
		case char >= 155 && char <= 251:
			s[i] = extraCharacters[char-155]
		case char == 13 || char == 10:
			s[i] = '\n'
		}
	}
	return string(s)
}

func (this *ZSCIIString) ZString(size int) ZString {
	zcharLimit := size / 2 * 3
	zchars := make([]byte, zcharLimit)

	i := 0
	for _, v := range this.bytes {
		z := zcharFromZSCIIChar(v)
		if i+len(z) > zcharLimit {
			break
		}
		zchars[i] = z[0]
		i++
		if len(z) == 2 {
			zchars[i] = z[1]
			i++
		}
	}

	for ; i < zcharLimit; i++ {
		zchars[i] = 5
	}

	words := make([]byte, size)
	for i, j := 0, 0; i < zcharLimit && j < len(words); i, j = i+3, j+2 {
		word := uint16(0)
		if i >= zcharLimit-3 {
			word |= 0x8000 // End of string flag.
		}

		word |= uint16(zchars[i]) << 10
		word |= uint16(zchars[i+1]) << 5
		word |= uint16(zchars[i+2])

		high := byte(word >> 8)
		low := byte(word)

		words[j] = high
		words[j+1] = low
	}

	return ZString{zchars, this.z}
}

func zcharFromZSCIIChar(char byte) []byte {
	specialCases := map[byte]byte{
		'\n': 1,
		'.':  12,
		',':  13,
		'!':  14,
		'?':  15,
		'_':  16,
		'#':  17,
		'\'': 18,
		'"':  19,
		'/':  20,
		'\\': 21,
		'-':  22,
		':':  23,
		'(':  24,
		')':  25,
	}

	switch {
	case char >= 'a' && char <= 'z':
		return []byte{char - 'a' + 6}
	case char >= 'A' && char <= 'Z':
		return []byte{4, char - 'A' + 6}
	case char == ' ':
		return []byte{0}
	case char >= '0' && char <= '9':
		return []byte{5, char - '0' + 8}
	}
	return []byte{5, specialCases[char]}
}

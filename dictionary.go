package zmachine

import "bytes"

func (this *ZMachine) locateStringInDictionary(bytes []byte) int {
	index := this.dictionaryLength / 2
	lowerBound := uint16(0)
	upperBound := this.dictionaryLength
	k := uint16(this.dictionaryEntryLength)
	start := this.dictionaryStart + uint16(this.memory[this.dictionaryStart]) + 4
	for {
		direction := 0
		for j := uint16(0); j < 4; j++ {
			chr := this.memory[start+index*k+j]
			if chr > bytes[j] {
				direction = -1
				break
			} else if chr < bytes[j] {
				direction = 1
				break
			}
		}

		switch {
		case direction == 0:
			return int(start + index*k)
		case upperBound == lowerBound || index == lowerBound || index == upperBound:
			return 0
		case direction < 0:
			upperBound = index
		case direction > 0:
			lowerBound = index
		}
		index = (lowerBound + upperBound) / 2
	}
	panic("One does not simply exit the infinite loop!")
}

func (this *ZMachine) tokeniseZSCII(table int, zscii ZSCIIString) {
	words := make([][]byte, 0, this.memory[table])
	nextWord := make([]byte, 0, 6)
	wordStarts := make([]byte, 0, this.memory[table])
	lastNewWord := byte(0)

	// Split the input into words separated by any separators present and spaces
	for i, char := range zscii.Bytes() {
		if char == 32 || bytes.Contains(this.wordSeparators, []byte{char}) {
			if len(nextWord) > 0 {
				words = append(words, nextWord)
				wordStarts = append(wordStarts, lastNewWord)
				nextWord = make([]byte, 0, 6)
			}

			// We include the separator iff it's not a space.
			if char != 32 {
				words = append(words, []byte{char})
				wordStarts = append(wordStarts, byte(i))
			}
			lastNewWord = byte(i) + 1
		} else {
			nextWord = append(nextWord, char)
		}
	}
	// If we were working on a word when we fell out of the loop, finish it off.
	if len(nextWord) > 0 {
		words = append(words, nextWord)
		wordStarts = append(wordStarts, lastNewWord)
	}

	// Store the number of words.
	this.memory[table+1] = byte(len(words))
	for i, word := range words {
		// The available space is given in the first byte of the table.
		// Be sure we don't overrun it.
		if byte(i) > this.memory[table] {
			break
		}

		zsciistring := ZSCIIString{word, this}
		zstring := zsciistring.ZString(4)
		pos := this.locateStringInDictionary(zstring)

		// Stuff the relevant information in.
		this.setNumber(table+i*4+2+0, uint16(pos))     // Bytes 0-1: Position in dictionary
		this.memory[table+i*4+2+2] = byte(len(word))   // Byte 2: Length of word in ZSCII string
		this.memory[table+i*4+2+3] = wordStarts[i] + 1 // Byte 3: Start of word in ZSCII string
	}
}

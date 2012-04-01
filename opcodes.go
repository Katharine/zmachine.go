package zmachine

import "strings"

var imp1op = []func(*ZMachine, uint16){}

var imp2op = []func(*ZMachine, uint16, uint16){}

var impvop = []func(*ZMachine, ...uint16){
	// call
	func(this *ZMachine, args ...uint16) {
		routine := this.unpackAddress(args[0])
		// Method 0 does nothing and returns 0.
		if routine == 0 {
			this.store(0)
			return
		}

		varcount := this.memory[routine]
		if varcount > 15 {
			panic("Calling address without a routine")
		}

		// Store information so we can get back here.
		this.pc++
		this.callStack.Push(uint16(((0x7F >> uint(len(args)-1)) << 8) | varcount)) // Only used for creating save files
		this.callStack.Push(uint16(this.memory[this.pc]))                          // The variable in which to store the return value
		this.callStack.Push(uint16(this.pc >> 16))                                 // The location to return to (minus one, actually)...
		this.callStack.Push(uint16(this.pc & 0xFFFF))                              // ...split over two words
		this.callStack.Push(uint16(this.stack.Size()))                             // Where to truncate the call stack on returning

		// Push the arguments to the routine we're calling onto the stack.
		// Any arguments not provided are filled with the destination's declared defaults.
		for i := byte(0); i < varcount; i++ {
			if i < byte(len(args)-1) {
				this.stack.Push(args[i+1])
			} else {
				this.stack.Push(this.number(int(routine) + 2*int(i) + 1))
			}
		}

		// Jump to the target routine (which starts varcount words after the given address)
		this.pc = routine + int(varcount)*2
	},

	// storew
	func(this *ZMachine, args ...uint16) {
		arr, index, value := int(args[0]), int(args[1]), uint16(args[2])
		this.setNumber(arr+2*index, value)
	},

	// storeb
	func(this *ZMachine, args ...uint16) {
		arr, index, value := args[0], args[1], byte(args[2])
		this.memory[arr+index] = value
	},

	// put_prop (TODO)
	nil,

	// read
	func(this *ZMachine, args ...uint16) {
		text, parse := int(args[0]), int(args[1])
		input, ok := <-this.input
		if !ok {
			panic("Input channel not okay!")
		}

		maxlength := int(this.memory[text]) + 1
		var read string
		if len(input) > maxlength {
			read = input[0:maxlength]
		} else {
			read = input
		}
		read = strings.ToLower(read)
		zscii := ZSCIIString{[]byte(read), this}
		copy(this.memory[text+1:text+maxlength+1], zscii.Bytes())
		this.memory[text+zscii.Size()+1] = 0 // Terminate string with null byte.
		this.tokeniseZSCII(parse, zscii)
	},
}

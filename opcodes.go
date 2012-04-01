package zmachine

import (
	"math/rand"
	"strings"
	"time"
)

var imp0op = []func(*ZMachine){
	// rtrue
	func(this *ZMachine) {
		this.returnFromRoutine(1)
	},

	// rfalse
	func(this *ZMachine) {
		this.returnFromRoutine(0)
	},

	// print
	func(this *ZMachine) {
		zchars := this.zString(this.pc+1, false)
		this.pc += (zchars.Size() / 3) * 2
		zscii := zchars.ZSCIIString()
		this.output <- zscii.String()
	},

	// print_ret
	func(this *ZMachine) {
		// Copied from print above because it can't call it ("initialization loop")
		zchars := this.zString(this.pc+1, false)
		this.pc += (zchars.Size() / 3) * 2
		zscii := zchars.ZSCIIString()
		this.output <- zscii.String()

		this.output <- "\n"
		this.returnFromRoutine(1)
	},

	// xyzzy
	func(this *ZMachine) {
		// Nothing happens.
	},

	// save (TODO)
	func(this *ZMachine) {
		this.branch(false)
	},

	// restore (TODO)
	func(this *ZMachine) {
		this.branch(false)
	},

	// restart (TODO)
	nil,

	// ret_popped
	func(this *ZMachine) {
		this.returnFromRoutine(this.stack.Pop())
	},

	// quit (TODO)
	nil,

	// new_line
	func(this *ZMachine) {
		this.output <- "\n"
	},

	// set_status
	func(this *ZMachine) {
		// Unimplemented.
	},

	// verify
	func(this *ZMachine) {
		// This is supposed to check for corruption.
		// Unimplemented.
		this.branch(true)
	},
}

var imp1op = []func(*ZMachine, uint16){
	// jz
	func(this *ZMachine, a uint16) {
		this.branch(a == 0)
	},

	// get_sibling
	func(this *ZMachine, obj uint16) {
		sibling := uint16(this.getObjectSibling(byte(obj)))
		this.store(sibling)
		this.branch(sibling != 0)
	},

	// get_child
	func(this *ZMachine, obj uint16) {
		child := uint16(this.getObjectChild(byte(obj)))
		this.store(child)
		this.branch(child != 0)
	},

	// get_parent
	func(this *ZMachine, obj uint16) {
		parent := uint16(this.getObjectParent(byte(obj)))
		this.store(parent)
		this.branch(parent != 0)
	},

	// get_prop_len
	func(this *ZMachine, address uint16) {
		if address == 0 {
			this.store(0)
		} else {
			this.store(uint16(this.memory[address-1]/32 + 1))
		}
	},

	// inc
	func(this *ZMachine, variable uint16) {
		v := byte(variable)
		value := this.getVariable(v)
		value++
		this.setVariable(v, value)
	},

	// dec
	func(this *ZMachine, variable uint16) {
		v := byte(variable)
		value := this.getVariable(v)
		value--
		this.setVariable(v, value)
	},

	// print_addr
	func(this *ZMachine, address uint16) {
		zchars := this.zString(int(address), false)
		zscii := zchars.ZSCIIString()
		this.output <- zscii.String()
	},

	// Nothing
	nil,

	// remove_obj
	func(this *ZMachine, obj uint16) {
		this.removeObject(byte(obj))
	},

	// print_obj
	func(this *ZMachine, objw uint16) {
		obj := byte(objw)
		zstring := this.getObjectName(obj)
		zscii := zstring.ZSCIIString()
		this.output <- zscii.String()
	},

	// ret
	func(this *ZMachine, value uint16) {
		this.returnFromRoutine(value)
	},

	// jump
	func(this *ZMachine, arg uint16) {
		offset := int16(arg)
		this.pc += int(offset) - 2
	},

	// print_paddr
	func(this *ZMachine, paddr uint16) {
		address := this.unpackAddress(paddr)
		zchars := this.zString(address, false)
		zscii := zchars.ZSCIIString()
		this.output <- zscii.String()
	},

	// load
	func(this *ZMachine, varw uint16) {
		this.store(this.getVariable(byte(varw)))
	},

	// not
	func(this *ZMachine, value uint16) {
		this.store(^value)
	},
}

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

	// put_prop
	func(this *ZMachine, args ...uint16) {
		obj, prop, value := byte(args[0]), byte(args[1]), args[2]
		address := this.getObjectPropertyAddress(obj, prop)
		size := this.getObjectPropertySize(obj, prop)
		if size == 1 {
			this.memory[address] = byte(value)
		} else if size == 2 {
			this.setNumber(address, value)
		} else {
			panic("Illegal put_prop on property of size greater than two")
		}
	},

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
		this.memory[text+zscii.Size()+1] = 0 // Terminate string with null
		this.tokeniseZSCII(parse, zscii)
	},

	// print_char
	func(this *ZMachine, args ...uint16) {
		zscii := ZSCIIString{[]byte{byte(args[0])}, this}
		this.output <- zscii.String()
	},

	// print_num
	func(this *ZMachine, args ...uint16) {
		this.output <- string(args[0])
	},

	// random
	func(this *ZMachine, args ...uint16) {
		r := int16(args[0])
		if r == 0 {
			rand.Seed(time.Now().Unix())
		} else if r < 0 {
			rand.Seed(int64(r * -1))
		} else {
			this.store(uint16(rand.Int31n(int32(r + 1))))
		}
	},

	// push
	func(this *ZMachine, args ...uint16) {
		this.stack.Push(args[0])
	},

	// pull
	func(this *ZMachine, args ...uint16) {
		variable := byte(args[0])
		this.setVariable(variable, this.stack.Pop())
	},

	// split_window
	nil,

	// set_window
	nil,

	// output_stream
	nil,

	// input_stream
	nil,
}
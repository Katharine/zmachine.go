package zmachine

import (
	"log"
	"os"
)

type OperandType byte
type OpcodeFormat byte

const OPCODE_FORMAT_SHORT OpcodeFormat = 1
const OPCODE_FORMAT_LONG OpcodeFormat = 2
const OPCODE_FORMAT_VARIABLE OpcodeFormat = 3

const OPERAND_TYPE_SMALL OperandType = 1
const OPERAND_TYPE_LARGE OperandType = 0
const OPERAND_TYPE_VAR OperandType = 2
const OPERAND_TYPE_OMITTED OperandType = 3

type ZMachine struct {
	input  chan string
	output chan string

	story_file string
	memory     []byte
	version    byte

	memoryDynamicEnd  uint16
	memoryStaticStart uint16
	memoryStaticEnd   uint16
	memoryHighStart   uint16
	memoryHighEnd     uint16

	dictionaryStart     uint16
	objectTableStart    uint16
	globalVariableStart uint16
	abbreviationStart   uint16

	wordSeparators        []byte
	dictionaryEntryLength byte
	dictionaryLength      uint16

	pc        int
	stack     Stack
	callStack Stack
	running   bool

	opcodesExecuted int
}

func Make(file string, in chan string, out chan string) ZMachine {
	machine := ZMachine{
		story_file: file,
		input:      in,
		output:     out,
	}

	return machine
}

func (this *ZMachine) LoadStory() (err error) {
	file, err := os.Open(this.story_file)
	defer file.Close()

	stat, _ := file.Stat()

	this.memory = make([]byte, stat.Size())
	_, err = file.Read(this.memory)

	return
}

func (this *ZMachine) CompleteSetup() {
	this.version = this.memory[0]
	if this.version > 3 {
		panic("Unsupported version")
	}

	this.memoryHighEnd = uint16(len(this.memory) - 1)
	this.memoryDynamicEnd = this.number(0x0E)
	this.memoryStaticStart = this.memoryDynamicEnd + 1
	this.memoryStaticEnd = this.memoryHighEnd
	if this.memoryStaticEnd < 0xFFFF {
		this.memoryStaticEnd = 0xFFFF
	}
	this.memoryHighStart = this.number(0x04)

	this.dictionaryStart = this.number(0x08)
	this.objectTableStart = this.number(0x0A)
	this.globalVariableStart = this.number(0x0C)
	this.abbreviationStart = this.number(0x18)

	this.pc = int(this.number(0x06))

	this.stack = MakeStack(1024)
	this.callStack = MakeStack(1024)

	n := uint16(this.memory[this.dictionaryStart]) + this.dictionaryStart + 1
	this.wordSeparators = []byte(this.memory[this.dictionaryStart+1 : n])
	this.dictionaryEntryLength = this.memory[n]
	this.dictionaryLength = this.number(int(n + 1))

	log.Printf("Loaded version %d story file from %s", this.version, this.story_file)
	log.Printf("dynamic_end: 0x%x, static_end: 0x%x, high_start: 0x%x", this.memoryDynamicEnd, this.memoryStaticEnd, this.memoryHighStart)
	log.Printf("dictionaryStart: 0x%x, dictionaryEntryLength: %d, objectTableStart: 0x%x, globalVariableStart: 0x%x, abbreviationStart; 0x%x",
		this.dictionaryStart, this.dictionaryEntryLength, this.objectTableStart, this.globalVariableStart, this.abbreviationStart)
	log.Printf("pc: 0x%x", this.pc)
}

func (this *ZMachine) number(address int) uint16 {
	if address > int(this.memoryHighEnd)-1 {
		panic("Attempt to retrieve data from past the end of high memory")
	}

	top := uint16(this.memory[address]) << 8
	bottom := uint16(this.memory[address+1])

	return top | bottom
}

func (this *ZMachine) setNumber(address int, number uint16) {
	high := byte(number >> 8)
	low := byte(number)
	this.memory[address] = high
	this.memory[address+1] = low
}

func (this *ZMachine) unpackAddress(address uint16) int {
	return 2 * int(address)
}

func (this *ZMachine) zString(address int, wordAddress bool) ZString {
	if wordAddress {
		address *= 2
	}
	return ZStringFromMemory(this, address)
}

func (this *ZMachine) executeCycle() {
	opcode := this.memory[this.pc]
	var format OpcodeFormat
	operandCount := 0
	var operandTypes []OperandType
	reallyVariable := false

	if opcode&0xC0 == 0xC0 {
		format = OPCODE_FORMAT_VARIABLE
		if opcode&0x20 == 0 {
			operandCount = 2
		} else {
			reallyVariable = true
		}
		opcode &= 0x1F
	} else if opcode&0x80 == 0x80 {
		format = OPCODE_FORMAT_SHORT
		if opcode&0x30 == 0x30 {
			operandCount = 0
		} else {
			operandCount = 1
			switch {
			case opcode&0x30 == 0x00:
				operandTypes = []OperandType{OPERAND_TYPE_LARGE}
			case opcode&0x10 == 0x10:
				operandTypes = []OperandType{OPERAND_TYPE_SMALL}
			case opcode&0x20 == 0x20:
				operandTypes = []OperandType{OPERAND_TYPE_VAR}
			default:
				panic("Nonsense in executeCycle")
			}
		}
		opcode &= 0x0F
	} else {
		format = OPCODE_FORMAT_LONG
		operandCount = 2
		operandTypes = []OperandType{OPERAND_TYPE_SMALL, OPERAND_TYPE_SMALL}
		if opcode&0x40 == 0x40 {
			operandTypes[0] = OPERAND_TYPE_VAR
		}
		if opcode&0x20 == 0x20 {
			operandTypes[1] = OPERAND_TYPE_VAR
		}
		opcode &= 0x1F
	}

	if format == OPCODE_FORMAT_VARIABLE {
		this.pc++
		bits := this.memory[this.pc]
		operandTypes = make([]OperandType, 0, 4)
		for i := uint(0); i < 4; i++ {
			now := OperandType((bits >> (3 - i) * 2) & 0x03)
			if now != OPERAND_TYPE_OMITTED {
				operandTypes = append(operandTypes, now)
				operandCount++
			}
		}
	}

	operands := make([]uint16, operandCount)
	for i, t := range operandTypes {
		if t == OPERAND_TYPE_LARGE {
			operands[i] = this.number(this.pc + 1)
			this.pc += 2
		} else if t == OPERAND_TYPE_SMALL {
			this.pc++
			operands[i] = uint16(this.memory[this.pc])
		} else if t == OPERAND_TYPE_VAR {
			this.pc++
			operands[i] = this.getVariable(this.memory[this.pc])
		}
	}

	// TODO: Actually call things somehow.
	_ = reallyVariable

	this.opcodesExecuted++
}

func (this *ZMachine) getVariable(variable byte) uint16 {
	if variable == 0x00 {
		return this.stack.Pop()
	} else if variable >= 0x10 {
		return this.number(int(this.globalVariableStart) + ((int(variable) - 0x10) * 2))
	} else {
		return this.stack.Look(int(this.callStack.Peek()) + int(variable) - 1)
	}
	return 0
}

func (this *ZMachine) setVariable(variable byte, value uint16) {
	if variable == 0x00 {
		this.stack.Push(value)
	} else if variable >= 0x10 {
		this.setNumber(int(this.globalVariableStart)+(int(variable)-0x10)*2, value)
	} else {
		this.stack.Set(int(this.callStack.Peek())+int(variable)-1, value)
	}
}

func (this *ZMachine) store(value uint16) {
	this.pc++
	this.setVariable(this.memory[this.pc], value)
}

func (this *ZMachine) branch(result bool) {
	this.pc++
	branch := this.memory[this.pc]
	required := branch&0x80 == 0x80
	target := int(branch & 0x3F)

	// If the second bit is set, target is two bytes (actually 14 bits - one was taken for
	// required and one for this flag), so we need to get the next byte.
	if branch&0x40 == 0 {
		target = target << 8
		this.pc++
		target |= int(this.memory[this.pc])
	}

	// target is a 14-bit signed integer. We need to manually sign it.
	if target&(1<<13) != 0 {
		target = target - (1 << 14)
	}

	// We only do anything if the result matches the required one.
	// 0 and 1 are shortcuts for ret 0 and ret 1.
	if result == required {
		switch target {
		case 0:
			this.returnFromRoutine(0)
		case 1:
			this.returnFromRoutine(1)
		default:
			this.pc += target - 2
		}
	}
}

// Used to return from a Z-Code routine, placing value in the appropriate location.
func (this *ZMachine) returnFromRoutine(value uint16) {
	stackTop := this.callStack.Pop()    // The top of the stack after returning
	this.pc = int(this.callStack.Pop()) // The program counter after returning
	retVar := this.callStack.Pop()      // The variable the caller wants the return value placed in
	this.callStack.Pop()                // A useless value
	this.stack.Truncate(uint(stackTop))
	this.setVariable(byte(retVar), value)
}

package zmachine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

import "github.com/Katharine/chunk.go"

func LoadQuetzalFile(filename string, machine *ZMachine) (err error) {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	form, err := chunk.Make(f)
	if err != nil {
		return err
	}
	ifzs := make([]byte, 4)
	if n, err := form.Read(ifzs); err != nil || n != 4 {
		return err
	}
	if string(ifzs) != "IFZS" {
		return errors.New("File is not a quetzal save file")
	}

	for {
		if chunk, err := chunk.Make(f); err == nil {
			fmt.Println(chunk.Name())
			if f, ok := quetzalChunkHandlers[chunk.Name()]; ok {
				if err := f(machine, chunk); err != nil {
					return err
				}
			} else {
				chunk.Skip()
			}
		} else if err == io.EOF {
			break
		} else {
			return err
		}
	}
	return nil
}

var quetzalChunkHandlers = map[string]func(machine *ZMachine, chunk *chunk.Chunk) error{
	"IFhd": func(machine *ZMachine, chunk *chunk.Chunk) error {
		var release uint16
		var serial = make([]byte, 6)
		var checksum uint16
		var pcb [3]byte
		binary.Read(chunk, binary.BigEndian, &release)
		binary.Read(chunk, binary.BigEndian, &serial)
		binary.Read(chunk, binary.BigEndian, &checksum)
		binary.Read(chunk, binary.BigEndian, &pcb)
		pc := int(pcb[0])<<16 | int(pcb[1])<<8 | int(pcb[2])

		if machine.number(0x02) != release || !bytes.Equal(serial, machine.memory[0x12:0x18]) || checksum != machine.number(0x1C) {
			return errors.New("Wrong game")
		}

		machine.LoadStory()
		machine.pc = pc - 1
		return nil
	},
	"CMem": func(machine *ZMachine, chunk *chunk.Chunk) error {
		cmem := make([]byte, chunk.Size())
		chunk.Read(cmem)
		pointer := uint16(0)
		skipping := false
		for _, b := range cmem {
			if b != 0 && !skipping {
				machine.memory[pointer] ^= b
				pointer++
			} else if !skipping {
				skipping = true
				pointer++
			} else {
				skipping = false
				pointer += uint16(b)
			}
		}

		if skipping {
			return errors.New("CMem chunk ended while skipping")
		}
		if pointer > machine.memoryDynamicEnd {
			return errors.New("CMem data overruns dynamic memory")
		}
		return nil
	},
	"UMem": func(machine *ZMachine, chunk *chunk.Chunk) error {
		if chunk.Size() != uint32(machine.memoryDynamicEnd) {
			return errors.New("Uncompressed memory image does not match dynamic memory area")
		}
		umem := make([]byte, chunk.Size())
		chunk.Read(umem)
		copy(machine.memory, umem)
		return nil
	},
	"Stks": func(machine *ZMachine, chunk *chunk.Chunk) error {
		machine.stack.Truncate(0)
		machine.callStack.Truncate(0)

		for {
			var pcb [3]byte
			if err := binary.Read(chunk, binary.BigEndian, &pcb); err == io.EOF {
				break
			} else if err != nil {
				return errors.New(fmt.Sprintf("Error while reading stack frame: %s", err))
			}

			pc := int(pcb[0])<<16 | int(pcb[1])<<8 | int(pcb[2])

			var flags, ret, argCount byte
			var stackSize uint16
			binary.Read(chunk, binary.BigEndian, &flags)
			binary.Read(chunk, binary.BigEndian, &ret)
			binary.Read(chunk, binary.BigEndian, &argCount)
			binary.Read(chunk, binary.BigEndian, &stackSize)

			localCount := flags & 0x0F
			//discardResult := flags & 0x10 == 0x10 // Unused by v1-3.

			local := make([]uint16, localCount)
			stack := make([]uint16, stackSize)

			binary.Read(chunk, binary.BigEndian, &local)
			binary.Read(chunk, binary.BigEndian, &stack)

			if pc > 0 {
				pc--
				machine.callStack.Push(uint16(argCount)<<8 | uint16(localCount))
				machine.callStack.Push(uint16(ret))
				machine.callStack.Push(uint16(pc >> 16))
				machine.callStack.Push(uint16(pc & 0xFFFF))
				machine.callStack.Push(uint16(machine.stack.Size()))
			}
			for _, v := range local {
				machine.stack.Push(v)
			}
			for _, word := range stack {
				machine.stack.Push(word)
			}
		}
		return nil
	},
}

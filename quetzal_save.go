package zmachine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func SaveQuetzalFile(filename string, machine *ZMachine, compressed bool) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	s := new(bytes.Buffer)
	// Header.
	quetzalWriteIFhd(s, machine)

	// If requested, try to write a compressed file. If we can't do that (because we
	// can't load the original story file), write an uncompressed file instead.
	if compressed {
		if err := quetzalWriteCMem(s, machine); err != nil {
			compressed = false
		}
	}
	if !compressed {
		quetzalWriteUMem(s, machine)
	}
	quetzalWriteStks(s, machine)
	quetzalWriteANNO(s, machine)

	// Write it to our file.
	length := uint32(s.Len() + 4)
	data := []interface{}{
		[]byte("FORM"),
		length,
		[]byte("IFZS"),
	}
	err = multiWrite(f, data)
	if err == nil {
		_, err = s.WriteTo(f)
	}
	if length&1 == 1 {
		f.Write([]byte{0})
	}
	return
}

func multiWrite(stream io.Writer, data []interface{}) error {
	for _, v := range data {
		if err := binary.Write(stream, binary.BigEndian, v); err != nil {
			return err
		}
	}
	return nil
}

func quetzalWriteIFhd(stream io.Writer, machine *ZMachine) {
	pc := machine.pc + 1
	pcb := [3]byte{byte((pc >> 16) & 0xFF), byte((pc >> 8) & 0xFF), byte(pc & 0xFF)}
	data := []interface{}{
		[]byte("IFhd"),
		uint32(13),
		machine.number(0x02),
		machine.memory[0x12:0x18],
		machine.number(0x1C),
		pcb,
		byte(0),
	}
	multiWrite(stream, data)
}

func quetzalWriteCMem(stream io.Writer, machine *ZMachine) (err error) {
	// We need the original machine image to do this.
	var original []byte
	if err := machine.loadStory(&original); err != nil {
		return err
	}

	// Should we only use some fraction of the dynamic memory size?
	// It would probably be worthwhile if 64k was a lot of memory.
	cmem := make([]byte, 0, machine.memoryDynamicEnd)

	running := false
	run := byte(0)
	for i := uint16(0); i < machine.memoryDynamicEnd; i++ {
		xor := original[i] ^ machine.memory[i]
		if xor != 0 {
			if running {
				running = false
				cmem = append(cmem, run)
			}
			cmem = append(cmem, xor)
		} else {
			if running {
				if run == 255 {
					cmem = append(cmem, 255, 0)
					run = 0
				} else {
					run++
				}
			} else {
				cmem = append(cmem, 0)
				running = true
				run = 0
			}
		}
	}

	if running {
		cmem = append(cmem, run)
	}

	header := []interface{}{
		[]byte("CMem"),
		uint32(len(cmem)),
	}
	multiWrite(stream, header)
	stream.Write(cmem)
	if len(cmem)&1 == 1 {
		stream.Write([]byte{0})
	}
	return nil
}

func quetzalWriteUMem(stream io.Writer, machine *ZMachine) {
	stream.Write([]byte("UMem"))
	binary.Write(stream, binary.BigEndian, uint32(machine.memoryDynamicEnd))
	stream.Write(machine.memory[:machine.memoryDynamicEnd])
	if machine.memoryDynamicEnd&1 == 1 {
		stream.Write([]byte{0})
	}
}

func quetzalWriteStks(stream io.Writer, machine *ZMachine) {
	callStackPointer := uint(0)
	frames := new(bytes.Buffer)

	// Dummy first frame
	frames.Write([]byte{0, 0, 0, 0, 0, 0}) // pc, flags, return variable, argument mask
	dummyFrameStackSize := uint16(0)
	if machine.callStack.Size() > 4 {
		dummyFrameStackSize = machine.callStack.Look(4)
	}
	binary.Write(frames, binary.BigEndian, dummyFrameStackSize)
	binary.Write(frames, binary.BigEndian, machine.stack.store[:dummyFrameStackSize]) // This is probably naughty.
	// Real frames!
	for callStackPointer < machine.callStack.Size() {
		argumentMask := byte(machine.callStack.Look(callStackPointer) >> 8)
		localCount := byte(machine.callStack.Look(callStackPointer) & 0x0F)
		ret := byte(machine.callStack.Look(callStackPointer + 1))
		pc := int(machine.callStack.Look(callStackPointer+2))<<8 | int(machine.callStack.Look(callStackPointer+3))
		top := machine.callStack.Look(callStackPointer + 4)
		callStackPointer += 5
		var stackSize uint16
		if callStackPointer+4 >= machine.callStack.Size() {
			stackSize = uint16(machine.stack.Size()) - top
		} else {
			stackSize = machine.callStack.Look(callStackPointer+4) - top
		}
		stackSize -= uint16(localCount)

		pc++ // Deal with us wanting pc to be one too low (because of the post-loop increment)

		pcb := [3]byte{byte((pc >> 16) & 0xFF), byte((pc >> 8) & 0xFF), byte(pc & 0xFF)}

		frame := []interface{}{
			pcb,
			localCount,
			ret,
			argumentMask,
			stackSize,
			machine.stack.store[top : top+uint16(localCount)],
			machine.stack.store[top+uint16(localCount) : top+uint16(localCount)+stackSize],
		}

		multiWrite(frames, frame)
	}

	frameSize := int32(frames.Len())
	header := []interface{}{
		[]byte("Stks"),
		frameSize,
	}

	multiWrite(stream, header)
	frames.WriteTo(stream)
	if frameSize&1 == 1 {
		stream.Write([]byte{0})
	}
}

func quetzalWriteANNO(stream io.Writer, machine *ZMachine) {
	message := fmt.Sprintf("Version %d game, saved by zmachine.go", machine.version)
	data := []interface{}{
		[]byte("ANNO"),
		uint32(len(message)),
		[]byte(message),
	}
	multiWrite(stream, data)
	if len(message)&1 == 1 {
		stream.Write([]byte{0})
	}
}

package zmachine

type Stack struct {
	store   []uint16
	pointer uint
}

func (this *Stack) Push(value uint16) {
	this.store[this.pointer] = value
	this.pointer++
}

func (this *Stack) Pop() uint16 {
	this.pointer--
	return this.store[this.pointer]
}

func (this *Stack) Peek() uint16 {
	if this.pointer == 0 {
		return 0
	}
	return this.store[this.pointer-1]
}

func (this *Stack) Look(where int) uint16 {
	return this.store[where]
}

func (this *Stack) Set(where int, value uint16) {
	this.store[where] = value
}

func (this *Stack) Truncate(size uint) {
	if size > this.pointer {
		panic("Attempting to truncate a stack to greater than its original size")
	}
	this.pointer = size
}

func (this *Stack) Size() uint {
	return this.pointer
}

func MakeStack(size int) Stack {
	return Stack{make([]uint16, size), 0}
}

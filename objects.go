package zmachine

func (this *ZMachine) getObjectAddress(obj byte) int {
	return int(this.objectTableStart + 62 + (uint16(obj)-1)*9)
}

func (this *ZMachine) getObjectAttribute(obj, attribute byte) bool {
	if attribute > 31 {
		panic("Attempt to read invalid attribute")
	}
	if obj == 0 {
		panic("Attempted to read attribute from null object")
	}
	address := uint16(this.getObjectAddress(obj))
	bit := byte(0x80 >> (attribute % 8))
	part := uint16(attribute / 8)
	return (this.memory[address+part]&bit == bit)
}

func (this *ZMachine) setObjectAttribute(obj, attribute byte, value bool) {
	if attribute > 31 {
		panic("Attempt to set invalid attribute")
	}
	if obj == 0 {
		panic("Attempted to set attribute from null object")
	}
	address := uint16(this.getObjectAddress(obj))
	bit := byte(0x80 >> (attribute % 8))
	part := uint16(attribute / 8)
	if value {
		this.memory[address+part] |= bit
	} else {
		this.memory[address+part] &= ^bit
	}
}

func (this *ZMachine) getObjectPropertyTableAddress(obj byte) int {
	if obj == 0 {
		panic("Attempted to read property table for null object")
	}
	address := this.getObjectAddress(obj)
	return int(this.number(address + 7))
}

func (this *ZMachine) getObjectName(obj byte) ZString {
	return this.zString(this.getObjectPropertyTableAddress(obj)+1, false)
}

func (this *ZMachine) getObjectPropertyAddress(obj, prop byte) int {
	address := this.getObjectPropertyTableAddress(obj)
	address += int(this.memory[address])*2 + 1
	for this.memory[address] != 0 {
		now := this.memory[address] % 32
		size := this.memory[address]/32 + 1
		if now == prop {
			return address + 1
		} else if now < prop {
			return 0
		}
		address += int(size) + 1
	}
	return 0
}

func (this *ZMachine) getObjectPropertySize(obj, prop byte) byte {
	address := this.getObjectPropertyAddress(obj, prop) - 1
	return this.memory[address]/32 + 1
}

func (this *ZMachine) getObjectParent(obj byte) byte {
	if obj == 0 {
		return 0
	}
	address := this.getObjectAddress(obj)
	return this.memory[address+4]
}

func (this *ZMachine) getObjectSibling(obj byte) byte {
	if obj == 0 {
		return 0
	}
	address := this.getObjectAddress(obj)
	return this.memory[address+5]
}

func (this *ZMachine) getObjectChild(obj byte) byte {
	if obj == 0 {
		return 0
	}
	address := this.getObjectAddress(obj)
	return this.memory[address+6]
}

func (this *ZMachine) getObjectPreviousSibling(obj byte) byte {
	parent := this.getObjectParent(obj)
	if parent > 0 {
		child := this.getObjectChild(parent)
		if child == obj {
			return 0
		}
		for child != 0 {
			sibling := this.getObjectSibling(child)
			if sibling == obj {
				return child
			}
			child = sibling
		}
	}
	return 0
}

func (this *ZMachine) removeObject(obj byte) {
	address := this.getObjectAddress(obj)
	previousSibling := this.getObjectPreviousSibling(obj)
	if previousSibling == 0 {
		parent := this.memory[address+4]
		if parent > 0 {
			parentAddress := this.getObjectAddress(parent)
			this.memory[parentAddress+6] = this.memory[address+5] // object.parentNode.firstChild = object.nextSibling
		}
	} else {
		previousAddress := this.getObjectAddress(previousSibling)
		this.memory[previousAddress+5] = this.memory[address+5] // object.previousSibling.nextSibling = object.nextSibling
	}
	this.memory[address+5] = 0 // object.nextSibling = null
	this.memory[address+4] = 0 // object.parentNode = null
}

func (this *ZMachine) insertObject(obj, dest byte) {
	objAddress := this.getObjectAddress(obj)
	destAddress := this.getObjectAddress(dest)

	// Pull the object out of its old location.
	this.removeObject(obj)

	this.memory[objAddress+5] = this.memory[destAddress+6] // object.nextSibling = dest.firstChild
	this.memory[objAddress+4] = dest                       // object.parentNode = dest
	this.memory[destAddress+6] = obj                       // dest.firstChild = object
}

package ioctl

func encodeFreeBSD(mode Mode, size uint16, cmd uintptr) Command {
	var dir Command
	if mode == None {
		dir = 0x20000000 // IOC_VOID
	} else {
		if mode&Write != 0 {
			dir |= 0x80000000 // IOC_IN
		}
		if mode&Read != 0 {
			dir |= 0x40000000 // IOC_OUT
		}
	}
	return dir | (Command(size)&0x1fff)<<16 | Command(cmd)
}

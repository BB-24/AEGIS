package chunker

const (
	gearSeed uint64 = 0x9e3779b97f4a7c15
	hashBits        = 48
	hashMask uint64 = (1 << hashBits) - 1
)

var defaultGear = generateGearTable(gearSeed)

func generateGearTable(seed uint64) [256]uint64 {
	var table [256]uint64
	state := seed
	for i := range table {
		state += 0x9e3779b97f4a7c15
		value := state
		value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
		value = (value ^ (value >> 27)) * 0x94d049bb133111eb
		value ^= value >> 31
		table[i] = value & hashMask
	}
	return table
}

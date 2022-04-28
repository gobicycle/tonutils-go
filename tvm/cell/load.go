package cell

import (
	"math/big"
)

type LoadCell struct {
	bitsSz   int
	loadedSz int
	data     []byte

	// store it as slice of pointers to make indexing logic cleaner on parse,
	// from outside it should always come as object to not have problems
	refs []*LoadCell
}

func (c *LoadCell) LoadRef() (*LoadCell, error) {
	if len(c.refs) == 0 {
		return nil, ErrNoMoreRefs
	}
	ref := c.refs[0]
	c.refs = c.refs[1:]

	return ref, nil
}

func (c *LoadCell) LoadCoins() (uint64, error) {
	value, err := c.LoadBigCoins()
	if err != nil {
		return 0, err
	}

	return value.Uint64(), nil
}

func (c *LoadCell) LoadBigCoins() (*big.Int, error) {
	// varInt 16 https://github.com/ton-blockchain/ton/blob/24dc184a2ea67f9c47042b4104bbb4d82289fac1/crypto/block/block-parse.cpp#L319
	ln, err := c.LoadUInt(4)
	if err != nil {
		return nil, err
	}

	value, err := c.LoadBigInt(int(ln * 8))
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *LoadCell) LoadUInt(sz int) (uint64, error) {
	res, err := c.LoadBigInt(sz)
	if err != nil {
		return 0, err
	}

	return res.Uint64(), nil
}

func (c *LoadCell) LoadBigInt(sz int) (*big.Int, error) {
	if sz > 256 {
		return nil, ErrTooBigValue
	}

	b, err := c.LoadSlice(sz)
	if err != nil {
		return nil, err
	}

	// check is value is uses full bytes
	if offset := sz % 8; offset > 0 {
		// move bits to right side of bytes
		for i := len(b) - 1; i >= 0; i-- {
			b[i] >>= 8 - offset // get last bits
			if i > 0 {
				b[i] += b[i-1] << offset
			}
		}
	}

	return new(big.Int).SetBytes(b), nil
}

func (c *LoadCell) LoadSlice(sz int) ([]byte, error) {
	if c.bitsSz-c.loadedSz < sz {
		return nil, ErrNotEnoughData
	}

	leftSz := sz
	unusedBits := 8 - (c.loadedSz % 8)

	var loadedData []byte

	oneMoreLeft, oneMoreRight := 0, 0
	if unusedBits < 8 {
		oneMoreLeft = 1
	}

	if sz%8 != 0 && sz < 8 {
		oneMoreRight = 1
	}

	ln := sz/8 + oneMoreLeft

	for i := oneMoreLeft; i < ln+oneMoreRight; i++ {
		var b byte
		if unusedBits < 8 {
			b = c.data[i-1] << byte(8-unusedBits)
			if i < ln {
				b += c.data[i] >> unusedBits
			}
		} else {
			b = c.data[i]
		}

		if leftSz == 0 {
			break
		} else if leftSz < 8 {
			b &= 0xFF << (8 - leftSz)
			leftSz = 0
			loadedData = append(loadedData, b)
			break
		}

		if i < ln {
			loadedData = append(loadedData, b)
		}
		leftSz -= 8
	}

	usedBytes := sz / 8
	if unusedBits > 0 && unusedBits <= sz%8 {
		usedBytes++
	}
	c.data = c.data[usedBytes:]

	c.loadedSz += sz

	return loadedData, nil
}

func (c *LoadCell) RestBits() (int, []byte, error) {
	left := c.bitsSz - c.loadedSz
	data, err := c.LoadSlice(left)
	return left, data, err
}

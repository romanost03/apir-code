package dpf

// This file contains all the client code for the FSS scheme.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	//"fmt"

	"github.com/si-co/vpir-code/lib/field"
	"github.com/si-co/vpir-code/lib/utils"
)

// ClientInitialize initializes client with this function numBits represents
// the input domain for the function, i.e. the number of bits to check
func ClientInitialize(numBits uint) *Fss {
	f := new(Fss)
	f.NumBits = numBits
	f.PrfKeys = make([][]byte, initPRFLen)
	// Create fixed AES blocks
	f.FixedBlocks = make([]cipher.Block, initPRFLen)
	for i := uint(0); i < initPRFLen; i++ {
		f.PrfKeys[i] = make([]byte, aes.BlockSize)
		rand.Read(f.PrfKeys[i])
		//fmt.Println("client")
		//fmt.Println(f.PrfKeys[i])
		block, err := aes.NewCipher(f.PrfKeys[i])
		if err != nil {
			panic(err.Error())
		}
		f.FixedBlocks[i] = block
	}
	// Check if int is 32 or 64 bit
	var x uint64 = 1 << 32
	if uint(x) == 0 {
		f.N = 32
	} else {
		f.N = 64
	}
	f.Temp = make([]byte, aes.BlockSize)
	f.Out = make([]byte, aes.BlockSize*initPRFLen)
	return f
}

// This is based on the following paper:
// Boyle, Elette, Niv Gilboa, and Yuval Ishai. "Function Secret Sharing:
// Improvements and Extensions." Proceedings of the 2016 ACM SIGSAC Conference
// on Computer and Communications Security. ACM, 2016.

// GenerateTreePF generates keys for 2-party point functions
// It creates keys for a function that evaluates to b when input x = a.
func (f Fss) GenerateTreePF(a uint, b *field.Element) []FssKeyEq2P {
	fssKeys := make([]FssKeyEq2P, 2)
	// Set up initial values
	tempRand1 := make([]byte, aes.BlockSize+1)
	rand.Read(tempRand1)
	fssKeys[0].SInit = tempRand1[:aes.BlockSize]
	fssKeys[0].TInit = tempRand1[aes.BlockSize] % 2
	fssKeys[1].SInit = make([]byte, aes.BlockSize)
	rand.Read(fssKeys[1].SInit)
	fssKeys[1].TInit = fssKeys[0].TInit ^ 1

	// Set current seed being used
	sCurr0 := make([]byte, aes.BlockSize)
	sCurr1 := make([]byte, aes.BlockSize)
	copy(sCurr0, fssKeys[0].SInit)
	copy(sCurr1, fssKeys[1].SInit)
	tCurr0 := fssKeys[0].TInit
	tCurr1 := fssKeys[1].TInit

	// Initialize correction words in FSS keys
	fssKeys[0].CW = make([][]byte, f.NumBits)
	fssKeys[1].CW = make([][]byte, f.NumBits)
	for i := uint(0); i < f.NumBits; i++ {
		// make AES block size + 2 bytes
		fssKeys[0].CW[i] = make([]byte, aes.BlockSize+2)
		fssKeys[1].CW[i] = make([]byte, aes.BlockSize+2)
	}

	leftStart := 0
	rightStart := aes.BlockSize + 1
	for i := uint(0); i < f.NumBits; i++ {
		// "expand" seed into two seeds + 2 bits
		prf(sCurr0, f.FixedBlocks, 3, f.Temp, f.Out)
		//prfKey(sCurr0, f.PrfKeys, 3, f.Temp, f.Out)
		prfOut0 := make([]byte, aes.BlockSize*3)
		copy(prfOut0, f.Out[:aes.BlockSize*3])
		prf(sCurr1, f.FixedBlocks, 3, f.Temp, f.Out)
		//prfKey(sCurr1, f.PrfKeys, 3, f.Temp, f.Out)
		prfOut1 := make([]byte, aes.BlockSize*3)
		copy(prfOut1, f.Out[:aes.BlockSize*3])

		// Parse out "t" bits
		t0Left := prfOut0[aes.BlockSize] % 2
		t0Right := prfOut0[(aes.BlockSize*2)+1] % 2
		t1Left := prfOut1[aes.BlockSize] % 2
		t1Right := prfOut1[(aes.BlockSize*2)+1] % 2
		// Find bit in a
		aBit := getBit(a, (f.N - f.NumBits + i + 1), f.N)

		// Figure out which half of expanded seeds to keep and lose
		keep := rightStart
		lose := leftStart
		if aBit == 0 {
			keep = leftStart
			lose = rightStart
		}

		// Set correction words for both keys. Note: they are the same
		for j := 0; j < aes.BlockSize; j++ {
			fssKeys[0].CW[i][j] = prfOut0[lose+j] ^ prfOut1[lose+j]
			fssKeys[1].CW[i][j] = fssKeys[0].CW[i][j]
		}
		fssKeys[0].CW[i][aes.BlockSize] = t0Left ^ t1Left ^ aBit ^ 1
		fssKeys[1].CW[i][aes.BlockSize] = fssKeys[0].CW[i][aes.BlockSize]
		fssKeys[0].CW[i][aes.BlockSize+1] = t0Right ^ t1Right ^ aBit
		fssKeys[1].CW[i][aes.BlockSize+1] = fssKeys[0].CW[i][aes.BlockSize+1]

		for j := 0; j < aes.BlockSize; j++ {
			sCurr0[j] = prfOut0[keep+j] ^ (tCurr0 * fssKeys[0].CW[i][j])
			sCurr1[j] = prfOut1[keep+j] ^ (tCurr1 * fssKeys[0].CW[i][j])
		}
		//fmt.Println("sKeep0:", prfOut0[keep:keep+aes.BlockSize])
		//fmt.Println("sKeep1:", prfOut1[keep:keep+aes.BlockSize])
		tCWKeep := fssKeys[0].CW[i][aes.BlockSize]
		if keep == rightStart {
			tCWKeep = fssKeys[0].CW[i][aes.BlockSize+1]
		}
		tCurr0 = (prfOut0[keep+aes.BlockSize] % 2) ^ tCWKeep*tCurr0
		tCurr1 = (prfOut1[keep+aes.BlockSize] % 2) ^ tCWKeep*tCurr1
	}

	// Convert final CW to integer
	sFinal0 := new(field.Element).SetBytes(sCurr0)
	sFinal1 := new(field.Element).SetBytes(sCurr1)
	sFinal0.Neg(sFinal0)
	fssKeys[0].FinalCW.Add(&fssKeys[0].FinalCW, sFinal0)
	fssKeys[0].FinalCW.Add(&fssKeys[0].FinalCW, sFinal1)
	fssKeys[0].FinalCW.Add(&fssKeys[0].FinalCW, b)
	fssKeys[1].FinalCW = fssKeys[0].FinalCW
	if tCurr1 == 1 {
		fssKeys[0].FinalCW.Neg(&fssKeys[0].FinalCW)
		fssKeys[1].FinalCW = fssKeys[0].FinalCW
	}
	return fssKeys
}

// GenerateTreePFVector generates keys for 2-party point functions It creates a
// vector of keys for a function that evaluates to b when
// input x = a.
func (f Fss) GenerateTreePFVector(a uint, b []field.Element) []FssKeyVectorEq2P {
	fssKeys := make([]FssKeyVectorEq2P, 2)
	// Set up initial values
	tempRand1 := make([]byte, aes.BlockSize+1)
	rand.Read(tempRand1)
	fssKeys[0].SInit = tempRand1[:aes.BlockSize]
	fssKeys[0].TInit = tempRand1[aes.BlockSize] % 2
	fssKeys[1].SInit = make([]byte, aes.BlockSize)
	rand.Read(fssKeys[1].SInit)
	fssKeys[1].TInit = fssKeys[0].TInit ^ 1

	// Set current seed being used
	sCurr0 := make([]byte, aes.BlockSize)
	sCurr1 := make([]byte, aes.BlockSize)
	copy(sCurr0, fssKeys[0].SInit)
	copy(sCurr1, fssKeys[1].SInit)
	tCurr0 := fssKeys[0].TInit
	tCurr1 := fssKeys[1].TInit

	// Initialize correction words in FSS keys
	fssKeys[0].CW = make([][]byte, f.NumBits)
	fssKeys[1].CW = make([][]byte, f.NumBits)
	for i := uint(0); i < f.NumBits; i++ {
		// make AES block size + 2 bytes
		fssKeys[0].CW[i] = make([]byte, aes.BlockSize+2)
		fssKeys[1].CW[i] = make([]byte, aes.BlockSize+2)
	}

	leftStart := 0
	rightStart := aes.BlockSize + 1
	for i := uint(0); i < f.NumBits; i++ {
		// "expand" seed into two seeds + 2 bits
		prf(sCurr0, f.FixedBlocks, 3, f.Temp, f.Out)
		//prfKey(sCurr0, f.PrfKeys, 3, f.Temp, f.Out)
		prfOut0 := make([]byte, aes.BlockSize*3)
		copy(prfOut0, f.Out[:aes.BlockSize*3])
		prf(sCurr1, f.FixedBlocks, 3, f.Temp, f.Out)
		//prfKey(sCurr1, f.PrfKeys, 3, f.Temp, f.Out)
		prfOut1 := make([]byte, aes.BlockSize*3)
		copy(prfOut1, f.Out[:aes.BlockSize*3])

		// Parse out "t" bits
		t0Left := prfOut0[aes.BlockSize] % 2
		t0Right := prfOut0[(aes.BlockSize*2)+1] % 2
		t1Left := prfOut1[aes.BlockSize] % 2
		t1Right := prfOut1[(aes.BlockSize*2)+1] % 2
		// Find bit in a
		aBit := getBit(a, (f.N - f.NumBits + i + 1), f.N)

		// Figure out which half of expanded seeds to keep and lose
		keep := rightStart
		lose := leftStart
		if aBit == 0 {
			keep = leftStart
			lose = rightStart
		}

		// Set correction words for both keys. Note: they are the same
		for j := 0; j < aes.BlockSize; j++ {
			fssKeys[0].CW[i][j] = prfOut0[lose+j] ^ prfOut1[lose+j]
			fssKeys[1].CW[i][j] = fssKeys[0].CW[i][j]
		}
		fssKeys[0].CW[i][aes.BlockSize] = t0Left ^ t1Left ^ aBit ^ 1
		fssKeys[1].CW[i][aes.BlockSize] = fssKeys[0].CW[i][aes.BlockSize]
		fssKeys[0].CW[i][aes.BlockSize+1] = t0Right ^ t1Right ^ aBit
		fssKeys[1].CW[i][aes.BlockSize+1] = fssKeys[0].CW[i][aes.BlockSize+1]

		for j := 0; j < aes.BlockSize; j++ {
			sCurr0[j] = prfOut0[keep+j] ^ (tCurr0 * fssKeys[0].CW[i][j])
			sCurr1[j] = prfOut1[keep+j] ^ (tCurr1 * fssKeys[0].CW[i][j])
		}
		//fmt.Println("sKeep0:", prfOut0[keep:keep+aes.BlockSize])
		//fmt.Println("sKeep1:", prfOut1[keep:keep+aes.BlockSize])
		tCWKeep := fssKeys[0].CW[i][aes.BlockSize]
		if keep == rightStart {
			tCWKeep = fssKeys[0].CW[i][aes.BlockSize+1]
		}
		tCurr0 = (prfOut0[keep+aes.BlockSize] % 2) ^ tCWKeep*tCurr0
		tCurr1 = (prfOut1[keep+aes.BlockSize] % 2) ^ tCWKeep*tCurr1
	}

	length := len(b)
	// Convert final CW to integer
	// Use bytes in sCurr0 and sCurr1 as a seed to generate a stream of
	// pseudorandom field elements
	// TODO: this is useless if only one bit element: add an if
	var key0 utils.PRGKey
	copy(key0[:], sCurr0)
	prg0 := utils.NewPRG(&key0)
	sFinal0, err := field.RandomVectorPointers(prg0, length)
	if err != nil {
		panic(err)
	}

	var key1 utils.PRGKey
	copy(key1[:], sCurr1)
	prg1 := utils.NewPRG(&key1)
	sFinal1, err := field.RandomVectorPointers(prg1, length)
	if err != nil {
		panic(err)
	}

	// preallocate finalCW for both keys
	fssKeys[0].FinalCW = make([]field.Element, length)
	fssKeys[1].FinalCW = make([]field.Element, length)

	// operate on vector elements
	for i := 0; i < length; i++ {
		sFinal0[i].Neg(sFinal0[i])
		fssKeys[0].FinalCW[i].Add(&fssKeys[0].FinalCW[i], sFinal0[i])
		fssKeys[0].FinalCW[i].Add(&fssKeys[0].FinalCW[i], sFinal1[i])
		fssKeys[0].FinalCW[i].Add(&fssKeys[0].FinalCW[i], &b[i])
		fssKeys[1].FinalCW[i] = fssKeys[0].FinalCW[i]
		if tCurr1 == 1 {
			fssKeys[0].FinalCW[i].Neg(&fssKeys[0].FinalCW[i])
			fssKeys[1].FinalCW[i] = fssKeys[0].FinalCW[i]
		}
	}
	return fssKeys
}

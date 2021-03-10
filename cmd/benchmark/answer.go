package main

import (
	"flag"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/si-co/vpir-code/lib/constants"
	"github.com/si-co/vpir-code/lib/database"
	"github.com/si-co/vpir-code/lib/field"
	"github.com/si-co/vpir-code/lib/utils"
)

func main() {
	workers := flag.Int("workers", 1, "number of workers")
	flag.Parse()

	dbLen := 1048576 * 8 * 1000 // 1000 MB
	blockLen := 16
	elemBitSize := field.Bytes * 8
	nRows := 1
	nCols := dbLen / (elemBitSize * blockLen * nRows)

	dbPRG := utils.RandomPRG()
	db := database.CreateRandomMultiBitDB(dbPRG, dbLen, nRows, blockLen)

	q, err := field.RandomVector(utils.RandomPRG(), nCols*(blockLen+1))
	if err != nil {
		panic(err)
	}

	start := time.Now()
	answer(q, db, *workers)
	t := time.Now()
	elapsed := t.Sub(start) / time.Millisecond
	fmt.Printf("%d,%dms\n", *workers, elapsed)
}

// Answer computes the answer for the given query
func answer(q []field.Element, db *database.DB, workers int) []field.Element {
	// Doing simplified scheme if block consists of a single bit
	if db.BlockSize == constants.SingleBitBlockLength {
		a := make([]field.Element, db.NumRows)
		for i := 0; i < db.NumRows; i++ {
			for j := 0; j < db.NumColumns; j++ {
				if db.Entries[i*db.NumColumns+j].Equal(&constants.One) {
					a[i].Add(&a[i], &q[j])
				}
			}
		}
		return a
	}

	// multithreading
	numCores := workers
	//numCores := 2
	// compute the matrix-vector inner products
	// addition and multiplication of elements
	// in DB(2^128)^b are executed component-wise
	// we have to traverse column by column
	var begin, end int
	if db.NumRows == 1 {
		numWorkers := 0
		// channel to pass the ch from the routines back
		ch := make(chan []field.Element, numCores*(db.BlockSize+1))
		colPerChunk := divideAndRoundUp(db.NumColumns, numCores)
		for j := 0; j < db.NumColumns; j += colPerChunk {
			colPerChunk, begin, end = computeChunkIndices(j, colPerChunk, db.NumColumns, db.BlockSize)
			go processRowChunk(db.Entries[begin:end], db.BlockSize, q[j*(db.BlockSize+1):(j+colPerChunk)*(db.BlockSize+1)], ch)
			numWorkers++
		}
		result := combineChunkResults(numWorkers, db.BlockSize+1, ch)
		close(ch)
		return result
	} else {
		m := make([]field.Element, db.NumRows*(db.BlockSize+1))
		var wg sync.WaitGroup
		rowsPerCore := divideAndRoundUp(db.NumRows, numCores)
		for j := 0; j < db.NumRows; j += rowsPerCore {
			rowsPerCore, begin, end = computeChunkIndices(j, rowsPerCore, db.NumRows, db.BlockSize)
			wg.Add(1)
			go processRows(db.Entries[begin*db.NumColumns:end*db.NumColumns], db.BlockSize, db.NumColumns, q, &wg,
				m[j*(db.BlockSize+1):(j+rowsPerCore)*(db.BlockSize+1)])
		}
		wg.Wait()

		return m
	}
}

// processing multiple rows by iterating over them
func processRows(rows []field.Element, blockLen, numColumns int, q []field.Element, wg *sync.WaitGroup, output []field.Element) {
	numElementsInRow := blockLen * numColumns
	for i := 0; i < len(rows)/numElementsInRow; i++ {
		res := multiplyAndTag(rows[i*numElementsInRow:(i+1)*numElementsInRow], blockLen, q)
		copy(output[i*(blockLen+1):(i+1)*(blockLen+1)], res)
	}
	wg.Done()
}

// processing a chunk of a database row
func processRowChunk(chunk []field.Element, blockLen int, q []field.Element, reply chan<- []field.Element) {
	reply <- multiplyAndTag(chunk, blockLen, q)
}

// combine the results of processing a row by different routines
func combineChunkResults(nw int, resLen int, workerReplies <-chan []field.Element) []field.Element {
	product := make([]field.Element, resLen)
	for i := 0; i < nw; i++ {
		reply := <-workerReplies
		for i, elem := range reply {
			product[i].Add(&product[i], &elem)
		}
	}
	return product
}

// multiplyAndTag multiplies db entries with the elements
// from the client query and computes a tag over each block
func multiplyAndTag(elements []field.Element, blockLen int, q []field.Element) []field.Element {
	var prodTag, prod field.Element
	sumTag := field.Zero()
	sum := field.ZeroVector(blockLen)
	for j := 0; j < len(elements)/blockLen; j++ {
		for b := 0; b < blockLen; b++ {
			if elements[j*blockLen+b].IsZero() {
				// no need to multiply if the element value is zero
				continue
			}
			// compute message
			prod.Mul(&elements[j*blockLen+b], &q[j*(blockLen+1)])
			sum[b].Add(&sum[b], &prod)
			// compute block tag
			prodTag.Mul(&elements[j*blockLen+b], &q[j*(blockLen+1)+1+b])
			sumTag.Add(&sumTag, &prodTag)
		}
	}
	return append(sum, sumTag)
}

func computeChunkIndices(ind, step, max, multiplier int) (int, int, int) {
	// avoiding overflow when colPerChunk does not divide db.Columns evenly
	if ind+step > max {
		step = max - ind
	}
	return step, ind * multiplier, (ind + step) * multiplier
}

func divideAndRoundUp(dividend, divisor int) int {
	return int(math.Ceil(float64(dividend) / float64(divisor)))
}

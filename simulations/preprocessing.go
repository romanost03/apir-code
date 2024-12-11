package main

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"time"

	"github.com/si-co/vpir-code/lib/database"
	"github.com/si-co/vpir-code/lib/merkle"
	"github.com/si-co/vpir-code/lib/monitor"
)

func RandomMerkleDB(rnd io.Reader, dbLen, numRows, blockLen, nRepeat int) []*Chunk {
	// run the experiment nRepeat times
	results := make([]*Chunk, nRepeat)

	// generate random data only once
	numBlocks := dbLen / (8 * blockLen)
	// generate random numBlocks blocks
	data := make([]byte, numBlocks*blockLen)
	if _, err := rnd.Read(data); err != nil {
		log.Fatal(err)
	}

	blocks := make([][]byte, numBlocks)
	for i := range blocks {
		// generate random block
		blocks[i] = make([]byte, blockLen)
		copy(blocks[i], data[i*blockLen:(i+1)*blockLen])
	}

	// clean memory since data is not needed anymore
	runtime.GC()

	m := monitor.NewMonitor()

	for j := 0; j < nRepeat; j++ {
		log.Printf("start repetition %d out of %d", j+1, nRepeat)
		logPerformanceMetrics("merkle", fmt.Sprintf("Start of repitition %d", j+1))
		results[j] = initChunk(1)
		results[j].CPU[0] = initBlock(1)

		m.Reset()

		// generate tree
		tree, err := merkle.New(blocks)
		if err != nil {
			log.Fatalf("impossible to create Merkle tree: %v", err)
		}

		// generate db
		numColumns := numBlocks / numRows
		proofLen := tree.EncodedProofLength()
		// +1 is for storing the padding signal byte
		blockLen = blockLen + proofLen + 1
		blockLens := make([]int, numRows*numColumns)
		for b := 0; b < numRows*numColumns; b++ {
			blockLens[b] = blockLen
		}

		measureExecutionTime("generateMerkleProofsParallel", func() {
			_ = generateMerkleProofsParallel(blocks, tree, blockLen)
		})

		/*measureExecutionTime("generateMerkleProofs", func() {
			_ = generateMerkleProofs(blocks, tree, blockLen)
		})*/

		logPerformanceMetrics("merkle", fmt.Sprintf("End of repitition %d", j+1))

		results[j].CPU[0].Answers[0] = m.RecordAndReset()

		// GC after each repetition
		runtime.GC()

		// sleep after every iteration
		time.Sleep(2 * time.Second)
	}

	return results

}

func generateMerkleProofs(data [][]byte, t *merkle.MerkleTree, blockLen int) []byte {
	result := make([]byte, 0, blockLen*len(data))
	for b := 0; b < len(data); b++ {
		p, err := t.GenerateProof(data[b])
		if err != nil {
			log.Fatalf("error while generating proof for block %v: %v", b, err)
		}
		encodedProof := merkle.EncodeProof(p)
		// appending 0x80
		encodedProof = database.PadWithSignalByte(encodedProof)
		// copying the data block and encoded proof into output
		result = append(result, append(data[b], encodedProof...)...)
	}

	return result
}

func generateMerkleProofsParallel(data [][]byte, t *merkle.MerkleTree, blockLen int) []byte {
	result := make([]byte, 0, blockLen*len(data))
	numWorkers := runtime.NumCPU()
	workChan := make(chan int, len(data))
	resultChan := make(chan []byte, len(data))

	worker := func() {
		for b := range workChan {
			p, err := t.GenerateProof(data[b])
			if err != nil {
				log.Fatalf("error while generating proof for block %v: %v", b, err)
			}
			encodedProof := merkle.EncodeProof(p)
			encodedProof = database.PadWithSignalByte(encodedProof)
			blockWithProof := append(data[b], encodedProof...)
			resultChan <- blockWithProof
		}
	}

	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	for i := range data {
		workChan <- i
	}
	close(workChan)

	for i := 0; i < len(data); i++ {
		result = append(result, <-resultChan...)
	}
	close(resultChan)

	return result
}

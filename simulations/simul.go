package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/cloudflare/circl/group"
	"github.com/si-co/vpir-code/lib/client"
	"github.com/si-co/vpir-code/lib/database"
	"github.com/si-co/vpir-code/lib/field"
	"github.com/si-co/vpir-code/lib/server"
	"github.com/si-co/vpir-code/lib/utils"
)

const generalConfigFile = "simul.toml"

type generalParam struct {
	DBBitLengths   []int
	BitsToRetrieve int
	Repetitions    int
}

type individualParam struct {
	Name           string
	Primitive      string
	NumServers     []int
	NumRows        int
	BlockLength    int
	ElementBitSize int
	InputSizes     []int // FSS input sizes in bytes
}

type Simulation struct {
	generalParam
	individualParam
}

func main() {
	// seed non-cryptographic randomness
	rand.Seed(time.Now().UnixNano())

	// create results directory if not presenc
	folderPath := "results"
	if _, err := os.Stat(folderPath); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(folderPath, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}

	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile := flag.String("memprofile", "", "write mem profile to file")
	indivConfigFile := flag.String("config", "", "config file for simulation")
	flag.Parse()

	// CPU profiling
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	// make sure cfg file is specified
	if *indivConfigFile == "" {
		panic("simulation's config file not provided")
	}
	log.Printf("config file %s", *indivConfigFile)

	// load simulation's config files
	s, err := loadSimulationConfigs(generalConfigFile, *indivConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	// check simulation
	if !s.validSimulation() {
		log.Fatal("invalid simulation")
	}

	log.Printf("running simulation %#v\n", s)
	// initialize experiment
	experiment := &Experiment{Results: make(map[int][]*Chunk, 0)}

	// amplification parameters (found via script in /scripts/integrity_amplification.py)
	// KiB, MiB, GiB
	tECC := map[int]int{
		1 << 13: 3,
		1 << 23: 4,
		1 << 33: 7,
		1 << 10: 2,
		1 << 20: 5,
		1 << 30: 10,
	}

	// range over all the DB lengths specified in the general simulation config
	for _, dl := range s.DBBitLengths {
		// compute database data
		dbLen := dl
		blockLen := s.BlockLength
		nRows := s.NumRows
		numBlocks := dl

		if s.Primitive == "cmp-vpir-dh" && dbLen == 1<<33 {
			log.Printf("skipping %d db for DH", dbLen)
			continue
		}

		// matrix db
		if nRows != 1 {
			utils.IncreaseToNextSquare(&numBlocks)
			nRows = int(math.Sqrt(float64(numBlocks)))
		}

		// setup db
		dbPRG := utils.RandomPRG()
		dbElliptic := new(database.Elliptic)
		dbLWE := new(database.LWE)
		dbLWE128 := new(database.LWE128)
		switch s.Primitive[:3] {
		case "cmp":
			if s.Primitive == "cmp-vpir-dh" {
				log.Printf("Generating elliptic db of size %d\n", dbLen)
				dbElliptic = database.CreateRandomEllipticWithDigest(dbPRG, dbLen, group.P256, true)
			} else if s.Primitive == "cmp-vpir-lwe" {
				log.Printf("Generating LWE db of size %d\n", dbLen)
				dbLWE = database.CreateRandomBinaryLWEWithLength(dbPRG, dbLen)
			} else if s.Primitive == "cmp-vpir-lwe-128" {
				log.Printf("Generating LWE128 db of size %d\n", dbLen)
				dbLWE128 = database.CreateRandomBinaryLWEWithLength128(dbPRG, dbLen)
			} else {
				log.Fatal("unknow primitive type:", s.Primitive)
			}
		}

		// GC after DB creation
		runtime.GC()
		time.Sleep(3)

		// run experiment
		var results []*Chunk
		switch s.Primitive {
		case "cmp-vpir-dh":
			log.Printf("db info: %#v", dbElliptic.Info)
			// results = pirElliptic(dbElliptic, s.Repetitions)
			results = pirEllipticParallel(dbElliptic, s.Repetitions)
		case "cmp-vpir-lwe": // LWE uses Amplify
			log.Printf("db info: %#v", dbLWE.Info)
			rep, ok := tECC[dbLen]
			if !ok {
				panic("tECC not defined for this db length")
			}
			//results = pirLWE(dbLWE, s.Repetitions, rep)
			results = pirLWEParallel(dbLWE, s.Repetitions, rep)
		case "cmp-vpir-lwe-128":
			log.Printf("db info: %#v", dbLWE128.Info)
			results = pirLWE128Parallel(dbLWE128, s.Repetitions)
		case "preprocessing":
			log.Printf("Merkle preprocessing evaluation for dbLen %d bits\n", dbLen)
			results = RandomMerkleDB(dbPRG, dbLen, nRows, blockLen, s.Repetitions)
		default:
			log.Fatal("unknown primitive type:", s.Primitive)
		}
		experiment.Results[dbLen] = results

		// GC at the end of the iteration
		runtime.GC()
	}

	// print results
	res, err := json.Marshal(experiment)
	if err != nil {
		panic(err)
	}
	fileName := s.Name + ".json"
	if err = ioutil.WriteFile(path.Join("results", fileName), res, 0644); err != nil {
		panic(err)
	}

	// mem profiling
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
	log.Println("simulation terminated successfully")
}

func pirLWE128(db *database.LWE128, nRepeat int) []*Chunk {
	numRetrievedBlocks := 1
	results := make([]*Chunk, nRepeat)

	p := utils.ParamsWithDatabaseSize128(db.Info.NumRows, db.Info.NumColumns)
	c := client.NewLWE128(utils.RandomPRG(), &db.Info, p)
	s := server.NewLWE128(db)

	for j := 0; j < nRepeat; j++ {
		log.Printf("start repetition %d out of %d", j+1, nRepeat)
		results[j] = initChunk(numRetrievedBlocks)
		logPerformanceMetrics("lwe128", fmt.Sprintf("Start of repitition %d", j+1))

		measureExecutionTime("lwe128", func() {
			// store digest size
			results[j].Digest = db.Auth.DigestLWE128.BytesSize()

			// pick a random block index to start the retrieval
			ii := rand.Intn(db.NumRows)
			jj := rand.Intn(db.NumColumns)
			results[j].CPU[0] = initBlock(1)
			results[j].Bandwidth[0] = initBlock(1)

			t := time.Now()

			query := c.Query(ii, jj)
			answer := s.Answer(query)
			if _, err := c.Reconstruct(answer); err != nil {
				log.Fatal(err)
			}

			// store eval results
			results[j].CPU[0].Reconstruct = time.Since(t).Seconds()
			results[j].Bandwidth[0].Query = query.BytesSize()
			results[j].Bandwidth[0].Answers[0] = answer.BytesSize()
		})

		logPerformanceMetrics("lwe128", fmt.Sprintf("End of repitition %d", j+1))
		// GC after each repetition
		runtime.GC()
		time.Sleep(2)
	}

	return results
}

// LWE uses Amplify
func pirLWE(db *database.LWE, nRepeat, tECC int) []*Chunk {
	numRetrievedBlocks := 1
	results := make([]*Chunk, nRepeat)

	p := utils.ParamsWithDatabaseSize(db.Info.NumRows, db.Info.NumColumns)
	c := client.NewAmplify(utils.RandomPRG(), &db.Info, p, tECC)
	s := server.NewAmplify(db)

	for j := 0; j < nRepeat; j++ {
		log.Printf("start repetition %d out of %d", j+1, nRepeat)
		results[j] = initChunk(numRetrievedBlocks)
		logPerformanceMetrics("lwe", fmt.Sprintf("Start of repition %d", j+1))

		measureExecutionTime("lwe", func() {
			// store digest size
			results[j].Digest = db.Auth.DigestLWE.BytesSize()
			// pick a random block index to start the retrieval
			ii := rand.Intn(db.NumRows)
			jj := rand.Intn(db.NumColumns)
			results[j].CPU[0] = initBlock(1)
			results[j].Bandwidth[0] = initBlock(1)

			t := time.Now()

			query := c.Query(ii, jj)
			answer := s.Answer(query)
			if _, err := c.Reconstruct(answer); err != nil {
				log.Fatal(err)
			}
			results[j].CPU[0].Reconstruct = time.Since(t).Seconds()
			results[j].Bandwidth[0].Query = query[0].BytesSize() * float64(len(query))        // all matrices equal
			results[j].Bandwidth[0].Answers[0] = float64(len(answer)) * answer[0].BytesSize() // all matrices equal

		})

		logPerformanceMetrics("lwe", fmt.Sprintf("End of repition %d", j+1))

		// GC after each repetition
		runtime.GC()
		time.Sleep(2)
	}

	return results
}

func pirElliptic(db *database.Elliptic, nRepeat int) []*Chunk {
	numRetrievedBlocks := 1
	results := make([]*Chunk, nRepeat)

	prg := utils.RandomPRG()
	c := client.NewDH(prg, &db.Info)
	s := server.NewDH(db)

	for j := 0; j < nRepeat; j++ {
		log.Printf("start repetition %d out of %d", j+1, nRepeat)

		logPerformanceMetrics("elliptic", fmt.Sprintf("Start of repition %d", j+1))

		measureExecutionTime("elliptic", func() {
			results[j] = initChunk(numRetrievedBlocks)

			// store digest size
			results[j].Digest = float64(len(db.SubDigests)) + float64(len(db.Digest))

			// pick a random block index to start the retrieval
			index := rand.Intn(db.NumRows * db.NumColumns)
			results[j].CPU[0] = initBlock(1)
			results[j].Bandwidth[0] = initBlock(1)

			//m.Reset()
			t := time.Now()
			query, err := c.QueryBytes(index)
			if err != nil {
				log.Fatal(err)
			}
			//results[j].CPU[0].Query = m.RecordAndReset()
			results[j].CPU[0].Query = 0
			results[j].Bandwidth[0].Query += float64(len(query))

			// get server's answer
			answer, err := s.AnswerBytes(query)
			if err != nil {
				log.Fatal(err)
			}
			//results[j].CPU[0].Answers[0] = m.RecordAndReset()
			results[j].CPU[0].Answers[0] = 0
			results[j].Bandwidth[0].Answers[0] = float64(len(answer))

			_, err = c.ReconstructBytes(answer)
			if err != nil {
				log.Fatal(err)
			}
			results[j].CPU[0].Reconstruct = time.Since(t).Seconds()
			results[j].Bandwidth[0].Reconstruct = 0

		})
		logPerformanceMetrics("elliptic", fmt.Sprintf("End of repition %d", j+1))
		// GC after each repetition
		runtime.GC()
		time.Sleep(2)
	}

	return results
}

// Converts number of bits to retrieve into the number of db blocks
func bitsToBlocks(blockSize, elemSize, numBits int) int {
	return int(math.Ceil(float64(numBits) / float64(blockSize*elemSize)))
}

func fieldVectorByteLength(vec []uint32) float64 {
	return float64(len(vec) * field.Bytes)
}

func initChunk(numRetrieveBlocks int) *Chunk {
	return &Chunk{
		CPU:       make([]*Block, numRetrieveBlocks),
		Bandwidth: make([]*Block, numRetrieveBlocks),
		Digest:    0,
	}
}

func initBlock(numAnswers int) *Block {
	return &Block{
		Query:       0,
		Answers:     make([]float64, numAnswers),
		Reconstruct: 0,
	}
}

func loadSimulationConfigs(genFile, indFile string) (*Simulation, error) {
	var err error
	genConfig := new(generalParam)
	_, err = toml.DecodeFile(genFile, genConfig)
	if err != nil {
		return nil, err
	}
	indConfig := new(individualParam)
	_, err = toml.DecodeFile(indFile, indConfig)
	if err != nil {
		return nil, err
	}
	return &Simulation{generalParam: *genConfig, individualParam: *indConfig}, nil
}

func (s *Simulation) validSimulation() bool {
	return s.Primitive == "cmp-vpir-dh" ||
		s.Primitive == "cmp-vpir-lwe" ||
		s.Primitive == "cmp-vpir-lwe-128" ||
		s.Primitive == "preprocessing"
}

func logPerformanceMetrics(algoName string, step string) {
	cpuFile, err := os.OpenFile(fmt.Sprintf("%s_cpu_usage.txt", algoName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Could not open CPU usage file: %v", err)
	}
	defer cpuFile.Close()

	ramFile, err := os.OpenFile(fmt.Sprintf("%s_ram_usage.txt", algoName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Could not open RAM usage file: %v", err)
	}
	defer ramFile.Close()

	var numGoroutines = getCPUUsage()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	ramUsage := memStats.Alloc / 1024 / 1024 // In MB

	fmt.Fprintf(cpuFile, "%s: Goroutines: %d\n", step, numGoroutines)
	fmt.Fprintf(ramFile, "%s: RAM Usage: %d MB\n", step, ramUsage)
}

func measureExecutionTime(algoName string, fn func()) {
	timeFile, err := os.OpenFile(fmt.Sprintf("%s_execution_time.txt", algoName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Could not open execution time file: %v", err)
	}
	defer timeFile.Close()

	start := time.Now()
	fn()
	elapsed := time.Since(start)

	fmt.Fprintf(timeFile, "Execution Time: %s\n", elapsed)
}

func getCPUUsage() string {
	out, err := exec.Command("bash", "-c", "top -b -d 1 -n 5 | grep 'Cpu(s)'").Output()
	if err != nil {
		return "Error measuring CPU usage"
	}
	return strings.TrimSpace(string(out))
}

func parallelTests(nRepeat int, numWorkers int, testFunc func(int)) {
	jobs := make(chan int, nRepeat)
	results := make(chan struct{}, nRepeat)

	for w := 0; w < numWorkers; w++ {
		go func() {
			for j := range jobs {
				testFunc(j)
				results <- struct{}{}
			}
		}()
	}

	for j := 0; j < nRepeat; j++ {
		jobs <- j
	}
	close(jobs)

	for a := 0; a < nRepeat; a++ {
		<-results
	}
}

func pirLWE128Parallel(db *database.LWE128, nRepeat int) []*Chunk {
	numRetrievedBlocks := 1
	results := make([]*Chunk, nRepeat)

	p := utils.ParamsWithDatabaseSize128(db.Info.NumRows, db.Info.NumColumns)

	numWorkers := runtime.NumCPU()
	parallelTests(nRepeat, numWorkers, func(j int) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic occurred in repetition %d: %v", j+1, r)
			}
		}()

		log.Printf("start repetition %d out of %d", j+1, nRepeat)
		results[j] = initChunk(numRetrievedBlocks)

		c := client.NewLWE128(utils.RandomPRG(), &db.Info, p)
		s := server.NewLWE128(db)

		logPerformanceMetrics("lwe128", fmt.Sprintf("Start of repetition %d", j+1))

		measureExecutionTime("lwe128", func() {
			results[j].Digest = db.Auth.DigestLWE128.BytesSize()

			ii := rand.Intn(db.NumRows)
			jj := rand.Intn(db.NumColumns)
			results[j].CPU[0] = initBlock(1)
			results[j].Bandwidth[0] = initBlock(1)

			t := time.Now()

			query := c.Query(ii, jj)
			if query == nil {
				log.Fatalf("Failed to generate query: repetition %d, indices (%d, %d)", j+1, ii, jj)
			}
			answer := s.Answer(query)
			if answer == nil {
				log.Fatalf("Failed to generate answer: repetition %d, query size", j+1)
			}
			if _, err := c.Reconstruct(answer); err != nil {
				log.Fatalf("Reconstruction failed: repetition %d, error: %v", j+1, err)
			}

			results[j].CPU[0].Reconstruct = time.Since(t).Seconds()
			results[j].Bandwidth[0].Query = query.BytesSize()
			results[j].Bandwidth[0].Answers[0] = answer.BytesSize()
		})

		logPerformanceMetrics("lwe128", fmt.Sprintf("End of repetition %d", j+1))
		runtime.GC()
		time.Sleep(2)
	})

	return results
}

func pirLWEParallel(db *database.LWE, nRepeat, tECC int) []*Chunk {
	numRetrievedBlocks := 1
	results := make([]*Chunk, nRepeat)

	numWorkers := runtime.NumCPU()

	parallelTests(nRepeat, numWorkers, func(j int) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic occurred in repetition %d: %v", j+1, r)
			}
		}()

		log.Printf("start repetition %d out of %d", j+1, nRepeat)
		results[j] = initChunk(numRetrievedBlocks)

		// Jede Goroutine verwendet ihre eigene Client- und Server-Instanz
		p := utils.ParamsWithDatabaseSize(db.Info.NumRows, db.Info.NumColumns)
		c := client.NewAmplify(utils.RandomPRG(), &db.Info, p, tECC)
		s := server.NewAmplify(db)

		logPerformanceMetrics("lwe", fmt.Sprintf("Start of repetition %d", j+1))

		measureExecutionTime("lwe", func() {
			// Store digest size
			results[j].Digest = db.Auth.DigestLWE.BytesSize()

			// Pick a random block index to start the retrieval
			ii := rand.Intn(db.NumRows)
			jj := rand.Intn(db.NumColumns)
			results[j].CPU[0] = initBlock(1)
			results[j].Bandwidth[0] = initBlock(1)

			// Query generation
			query := c.Query(ii, jj)
			if query == nil {
				log.Fatalf("Failed to generate query: repetition %d, indices (%d, %d)", j+1, ii, jj)
			}

			// Answer generation
			answer := s.Answer(query)
			if answer == nil {
				log.Fatalf("Failed to generate answer: repetition %d, query size %d", j+1, len(query))
			}

			// Reconstruction
			if _, err := c.Reconstruct(answer); err != nil {
				log.Fatalf("Reconstruction failed: repetition %d, error: %v", j+1, err)
			}

			// Store evaluation results
			results[j].CPU[0].Reconstruct = time.Since(time.Now()).Seconds()
			results[j].Bandwidth[0].Query = query[0].BytesSize() * float64(len(query))        // all matrices equal
			results[j].Bandwidth[0].Answers[0] = float64(len(answer)) * answer[0].BytesSize() // all matrices equal
		})

		logPerformanceMetrics("lwe", fmt.Sprintf("End of repetition %d", j+1))

		// Garbage Collection and Sleep
		runtime.GC()
		time.Sleep(2)
	})

	return results
}

func pirEllipticParallel(db *database.Elliptic, nRepeat int) []*Chunk {
	numRetrievedBlocks := 1
	results := make([]*Chunk, nRepeat)

	numWorkers := runtime.NumCPU()
	prg := utils.RandomPRG()

	parallelTests(nRepeat, numWorkers, func(j int) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic occurred in repetition %d: %v", j+1, r)
			}
		}()

		log.Printf("start repetition %d out of %d", j+1, nRepeat)
		results[j] = initChunk(numRetrievedBlocks)

		// Jede Goroutine verwendet ihre eigene Client- und Server-Instanz
		c := client.NewDH(prg, &db.Info)
		s := server.NewDH(db)

		logPerformanceMetrics("elliptic", fmt.Sprintf("Start of repetition %d", j+1))

		measureExecutionTime("elliptic", func() {
			// Store digest size
			results[j].Digest = float64(len(db.SubDigests)) + float64(len(db.Digest))

			// Pick a random block index to start the retrieval
			index := rand.Intn(db.NumRows * db.NumColumns)
			results[j].CPU[0] = initBlock(1)
			results[j].Bandwidth[0] = initBlock(1)

			// Query generation
			query, err := c.QueryBytes(index)
			if err != nil {
				log.Fatalf("Error generating query for repetition %d: %v", j+1, err)
			}
			results[j].Bandwidth[0].Query += float64(len(query))

			// Server answer generation
			answer, err := s.AnswerBytes(query)
			if err != nil {
				log.Fatalf("Error generating answer for repetition %d: %v", j+1, err)
			}
			results[j].Bandwidth[0].Answers[0] = float64(len(answer))

			// Reconstruct the result
			if _, err = c.ReconstructBytes(answer); err != nil {
				log.Fatalf("Reconstruction failed for repetition %d: %v", j+1, err)
			}
			results[j].CPU[0].Reconstruct = time.Since(time.Now()).Seconds()
		})

		logPerformanceMetrics("elliptic", fmt.Sprintf("End of repetition %d", j+1))
		// Garbage Collection and Sleep
		runtime.GC()
		time.Sleep(2)
	})

	return results
}

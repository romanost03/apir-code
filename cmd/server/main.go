package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/si-co/vpir-code/lib/constants"
	"github.com/si-co/vpir-code/lib/database"
	"github.com/si-co/vpir-code/lib/utils"

	"github.com/si-co/vpir-code/lib/proto"
	"github.com/si-co/vpir-code/lib/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	// flags
	sid := flag.Int("id", -1, "Server ID")
	schemePtr := flag.String("scheme", "", "dpf for DPF-based and IT for information-theoretic")
	logFile := flag.String("log", "", "write log to file instead of stdout/stderr")
	prof := flag.Bool("prof", false, "Write CPU prof file")
	flag.Parse()

	// start profiling
	if *prof {
		fmt.Println("here")
		utils.StartProfiling(fmt.Sprintf("server-%v.prof", *sid))
		defer utils.StopProfiling()
	}

	// set logs
	log.SetOutput(os.Stdout)
	log.SetPrefix(fmt.Sprintf("[Server %v] ", *sid))
	if len(*logFile) > 0 {
		f, err := os.Create(*logFile)
		if err != nil {
			log.Fatal("Could not open file: ", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	// configs
	config, err := utils.LoadConfig("config.toml")
	if err != nil {
		log.Fatalf("could not load the server config file: %v", err)
	}
	addr := config.Addresses[*sid]

	// generate db
	// TODO: generate different db if PIR
	// TODO: How do we choose dbLen (hence, nCols) ?
	dbLen := 40 * 1024 * 8
	chunkLength := constants.ChunkBytesLength // maximum numer of bytes embedded in a field elements
	nRows := 1
	nCols := dbLen / (nRows * chunkLength)
	db, err := database.GenerateRandomKeyDB("data/random_id_key.csv", chunkLength, nRows, nCols)
	if err != nil {
		log.Fatalf("could not generate keys db: %v", err)
	}

	// run server with TLS
	cfg := &tls.Config{
		Certificates: []tls.Certificate{utils.ServerCertificates[*sid]},
		ClientAuth:   tls.NoClientCert,
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	rpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(cfg)))

	// select correct server
	var s server.Server
	switch *schemePtr {
	case "dpf":
		s = server.NewDPF(db, byte(*sid))
	case "it":
		s = server.NewIT(db)
	default:
		log.Fatal("undefined scheme type")
	}
	log.Printf("scheme: %s", *schemePtr)

	// start server
	proto.RegisterVPIRServer(rpcServer, &vpirServer{Server: s})
	log.Printf("is listening at %s", addr)

	if err := rpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// vpirServer is used to implement VPIR Server protocol.
type vpirServer struct {
	proto.UnimplementedVPIRServer
	Server server.Server // both IT and DPF-based server
}

func (s *vpirServer) DatabaseInfo(ctx context.Context, r *proto.DatabaseInfoRequest) (
	*proto.DatabaseInfoResponse, error) {
	log.Print("got databaseInfo request")

	dbInfo := s.Server.DBInfo()
	resp := &proto.DatabaseInfoResponse{
		NumRows:     uint32(dbInfo.NumRows),
		NumColumns:  uint32(dbInfo.NumColumns),
		BlockLength: uint32(dbInfo.BlockSize),
		IdLength:    uint32(dbInfo.IDLength),
		KeyLength:   uint32(dbInfo.KeyLength),
	}

	return resp, nil
}

func (s *vpirServer) Query(ctx context.Context, qr *proto.QueryRequest) (
	*proto.QueryResponse, error) {
	log.Print("got query request")

	a, err := s.Server.AnswerBytes(qr.GetQuery())
	if err != nil {
		return nil, err
	}

	return &proto.QueryResponse{Answer: a}, nil
}

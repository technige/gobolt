package main

import (
	"flag"
	"neo4j-go-connector/pkg"
	"errors"
	"fmt"
	"time"
	"log"
	"os"
)

var (
	uri string
	username string
	password string
	secure bool
	query string
	debug bool
	stats bool
)

func executeQuery()  {
	start := time.Now()
	connector, err := neo4j.NewConnector(uri, map[string]interface{}{
		"principal": username,
		"credentials": password,
	}, &neo4j.Config{Encryption: secure, Debug: debug })
	if err != nil {
		panic(err)
	}
	defer connector.Close()
	elapsed := time.Since(start)
	if stats {
		log.Printf("NewConnector took %s", elapsed)
	}

	start = time.Now()
	pool, err := connector.GetPool()
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	elapsed = time.Since(start)
	if stats {
		log.Printf("GetPool took %s", elapsed)
	}

	start = time.Now()
	conn, err := pool.Acquire()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	elapsed = time.Since(start)
	if stats {
		log.Printf("Acquire took %s", elapsed)
	}

	start = time.Now()
	runMsg, err := conn.Run( query, &map[string]interface{}{})
	if err != nil {
		panic(err)
	}
	elapsed = time.Since(start)
	if stats {
		log.Printf("Run took %s", elapsed)
	}

	start = time.Now()
	pullAllMsg, err := conn.PullAll()
	if err != nil {
		panic(err)
	}
	elapsed = time.Since(start)
	if stats {
		log.Printf("PullAll took %s", elapsed)
	}

	start = time.Now()
	err = conn.Flush()
	if err != nil {
		panic(err)
	}
	elapsed = time.Since(start)
	if stats {
		log.Printf("Flush took %s", elapsed)
	}


	start = time.Now()
	records, err := conn.FetchSummary(runMsg)
	if records != 0 {
		panic(errors.New("unexpected summary fetch return"))
	}
	elapsed = time.Since(start)
	if stats {
		log.Printf("FetchSummary took %s", elapsed)
	}

	start = time.Now()
	metadata, err := conn.Metadata()
	if err != nil {
		panic(err)
	}

	fields := metadata["fields"].([]interface{})
	for i := 0; i < len(fields); i++ {
		if i > 0 {
			fmt.Print("\t")
		}

		fmt.Print(fields[i])
	}
	fmt.Println()
	elapsed = time.Since(start)
	if stats {
		log.Printf("Summary processing took %s", elapsed)
	}

	start = time.Now()
	for {
		fetch, err := conn.Fetch(pullAllMsg)
		if err != nil {
			panic(err)
		}
		if fetch <= 0 {
			break
		}

		data, err := conn.Data()
		if err != nil {
			panic(err)
		}

		fields = data
		for i := 0; i < len(fields); i++ {
			if i > 0 {
				fmt.Print("\t")
			}

			fmt.Print(fields[i])
		}

		fmt.Println()
	}
	elapsed = time.Since(start)
	if stats {
		log.Printf("Result processing took %s", elapsed)
	}
}

func main() {
	flag.Parse()
	executeQuery()

	if stats {
		current, peak, events := neo4j.GetAllocationStats()

		fmt.Fprintf( os.Stderr, "=====================================\n")
		fmt.Fprintf( os.Stderr, "current allocation	: %d bytes\n", current)
		fmt.Fprintf( os.Stderr, "peak allocation		: %d bytes\n", peak)
		fmt.Fprintf( os.Stderr, "allocation events	: %d\n", events)
		fmt.Fprintf( os.Stderr, "=====================================\n")
	}
}

func init() {
	flag.BoolVar(&secure, "secure", true, "whether to use TLS encryption")
	flag.StringVar(&uri, "uri", "bolt://localhost:7687", "bolt uri to establish connection against")
	flag.StringVar(&username, "username", "neo4j", "bolt user name")

	flag.StringVar(&password, "password", "neo4j", "bolt password")
	flag.StringVar(&query, "query", "UNWIND RANGE(1,1000) AS N RETURN N", "cypher query to run")
	flag.BoolVar(&debug, "debug", true, "whether to use debug logging")
	flag.BoolVar(&stats, "stats", true, "whether to dump allocation stats on exit")
}
package main

import (
	"bufio"
	"encoding/gob"
	"encoding/hex"
	"strconv"

	//"container/heap"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"

	//"sort"
	//"strconv"
	"strings"
	"sync"
	"time"
)

var hostnames []string
var serviceAddress string
var servicePort string
var localPort string
var numConnections int
var numListens int

//var serviceConn net.Conn

var introducedConnections sync.Map

var connDecoders sync.Map
var connEncoders sync.Map
var connectedAddrs sync.Map
var transactions sync.Map
var working sync.Map

// var balances sync.Map
var balances map[int]int
var workingBalances map[int]int

var c SafeCounter

var chan_solved chan string
var quit_block chan string

var currentBlock string

var blockMutex *sync.Mutex

var balancesMutex *sync.Mutex

var blockLength int
var lengthMutex *sync.Mutex

// var globalBlockNum int

var blocks map[string]Block

const numBlockTransactions = 5

var numCommittedTransactions int
var committedMutex *sync.Mutex

var prevBlockHash string
var prevHashMutex *sync.Mutex

// var (
// 	outfile, _ = os.Create("transactions.txt") // update path for your needs
// 	logging    = log.New(outfile, "", 0)
// )

// type Message struct {
// 	msg string
// }

type SafeCounter struct {
	numTransactions int
	mux             sync.Mutex
}

type Transaction struct {
	Id          string
	Timestamp   float64
	Source      int
	Destination int
	Amount      int
}

type GobTransport struct {
	Type string
	Data interface{}
}

type Introduction struct {
	Addrs string
}

// Block which is broadcasted and received
type Block struct {
	PrevHash     string
	Transactions []Transaction
	HashSolution string
}

// // RECEIVING 12.34.56.78:9012
// // TRANSACTION 1551208414.204385 f78480653bf33e3fd700ee8fae89d53064c8dfa6 183 99 10
// type Transaction struct {
// 	timestamp      string
// 	transactionID string
// 	fromMember    string
// 	toMember      string
// 	amt            int
// }

//var c SafeCounter
//var balanceMutex sync.Mutex

func (c *SafeCounter) Inc() {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.

	c.numTransactions++

	c.mux.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *SafeCounter) Value() int {
	c.mux.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mux.Unlock()
	return c.numTransactions
}

// func main() {
// 	c := SafeCounter{numTransactions: make(int)}
// 	go c.Inc()

// 	time.Sleep(time.Second)
// 	fmt.Println(c.Value("somekey"))
// }

// Function to create a block from transactions
func createBlock(conn net.Conn) {
	fmt.Println("Entering createBlock...")

	hasEnough := false

	for {
		committedMutex.Lock()
		committedTx := numCommittedTransactions
		committedMutex.Unlock()

		if hasEnough {

			getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")

			fmt.Println(getTime, "Have enough transactions, creating block...\n")
			// read 2000 tx from tx pool

			count := 0

			block := make([]Transaction, 0)

			workingBalances = make(map[int]int)

			balancesMutex.Lock()
			for k, v := range balances {
				workingBalances[k] = v
			}
			balancesMutex.Unlock()

			committedTransactions := make(map[string]bool)

			prevHashMutex.Lock()
			prev := prevBlockHash
			prevHashMutex.Unlock()

			blockMutex.Lock()
			for prev != "0" {
				currBlock := blocks[prev]
				for _, txn := range currBlock.Transactions {
					committedTransactions[txn.Id] = true
				}
				prev = blocks[prev].PrevHash
			}
			blockMutex.Unlock()

			// blocktx := make(map[float64]Transaction)

			transactions.Range(func(key interface{}, value interface{}) bool {

				v := value.(Transaction)

				if _, ok := committedTransactions[v.Id]; !ok {

					count++

					if count > numBlockTransactions {
						return false
					}

					deducted := false

					if v.Source == 0 {
						deducted = true
					} else if val, ok := workingBalances[v.Source]; ok {
						if val >= v.Amount {
							workingBalances[v.Source] -= v.Amount
							deducted = true
						}
					}

					_, ok := workingBalances[v.Destination]
					if ok && deducted {
						workingBalances[v.Destination] += v.Amount
					} else {
						workingBalances[v.Destination] = v.Amount
					}

					if deducted {
						block = append(block, v)
					}
				}

				// t, _ := time.Parse("2006/01/02 15:04:05.999999", timestamp)

				// blocktx[transaction.timestamp] = transaction
				return true

			})

			prevHashMutex.Lock()
			prevHash := prevBlockHash
			prevHashMutex.Unlock()

			select {
			case <-quit_block:
				fmt.Println("Quit out of createBlock!")
				return
			case <-time.After(time.Second):
				committedMutex.Lock()
				numCommittedTransactions += count
				committedMutex.Unlock()
				sendBlock(Block{prevHash, block, "fakesoln"}, conn)
			}

			// var starts []float64
			// for k := range blocktx {
			// 	startfloat := k / 1000000000
			// 	starts = append(starts, startfloat)
			// }
			// sort.Float64s(starts)
			// block := ""
			// for iter, k := range starts {
			// 	v := blocktx[k]

			// check validity
			hasEnough = false

		} else if c.Value()-committedTx >= numBlockTransactions {
			// fmt.Println("Dumb thing thinks we have enough transactions already")
			// check if 2000 tx
			hasEnough = true

		} else {

			// Sleep for one sec
			time.Sleep(time.Second)
		}
	}
}

func sendBlock(blk Block, conn net.Conn) {
	// fmt.Println("Entering sendBlock...")

	// Send solve request to service
	// print("Entered sendBlock()...\n")

	block_hash := generateHash(blk.PrevHash, blk.Transactions)

	getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
	msgToSend := "SOLVE " + block_hash
	//fmt.Println("SERVICEMSG: ", msgToSend)
	fmt.Println(getTime, "Message to send: ", msgToSend)

	conn.Write([]byte(msgToSend + "\n"))

	/* Now wait for signal from channel */
	msgReceived := <-chan_solved

	prevHashMutex.Lock()
	prevBlockHash = block_hash
	prevHashMutex.Unlock()

	lengthMutex.Lock()
	blockLength++
	lengthMutex.Unlock()

	var sol_hash string
	sol_hash = strings.Split(msgReceived, " ")[2]
	// fmt.Println("I got past the conn write msgToSend...	")

	// for {
	// 	_, err := conn.Write([]byte(msgToSend + "\n"))
	// 	if err == nil {
	// 		break
	// 	}
	// }

	// scannerConn := bufio.NewScanner(conn)

	// var sol_hash string
	// for scannerConn.Scan() {
	// 	msgReceived := scannerConn.Text()
	// 	cmd := strings.Split(msgReceived, " ")[0]
	// 	if cmd == "SOLVED" {
	// 		sol_hash = strings.Split(msgReceived, " ")[2]

	// 		break
	getTime = time.Now().UTC().Format("2006/01/02 15:04:05.000000")
	fmt.Print(getTime, "The solved msgReceived is", msgReceived)

	// 	}
	// }

	// Gossip block to all connections

	// fmt.Println("Before balances mutex")

	balancesMutex.Lock()

	// introducedConnections.Range(func(key interface{}, value interface{}) bool {

	// 	encoder := gob.NewEncoder(value.(net.Conn))

	// 	// Construct block
	// 	block_data := Block{prevHash: blk.prevHash, transactions: blk.transactions, hashSolution: sol_hash}
	// 	gob_data := GobTransport{Type: "Block", Data: block_data}
	// 	// try until serialization successful
	// 	for {
	// 		err := encoder.Encode(gob_data)
	// 		// err := encoder.Encode(block_data)
	// 		// if TCP connection has error (failed)
	// 		if err == nil {
	// 			// // parse out IP address of user connected to
	// 			// parts := strings.Split(c.RemoteAddr().String(), ":")
	// 			// // retrieve chat alias of user and display user has left
	// 			// value, ok := vm_names.Load(parts[0])
	// 			// if ok {
	// 			//   fmt.Println(value, "has left")
	// 			// }
	// 			break

	// 		}
	// 	}

	// 	return true
	// })

	// ################################
	block_data := Block{PrevHash: blk.PrevHash, Transactions: blk.Transactions, HashSolution: sol_hash}
	gob_data := GobTransport{Type: "Block", Data: block_data}

	// Send transaction to everyone [except self!]
	introducedConnections.Range(func(key interface{}, value interface{}) bool {

		hostname, _ := os.Hostname()
		if hostname+":"+localPort != key {
			// print("Gossiping block \n")
			encoder_interface, _ := connEncoders.Load(key)
			encoder := encoder_interface.(*gob.Encoder)
			// Try until serialization successful
			for {
				err := encoder.Encode(gob_data)
				if err == nil {
					break
				}
				// fmt.Println(err)
			}

			// 							transaction := Transaction{id: id, timestamp: timestamp, source: src, destination: dest, amount: amt}
			// (value.(net.Conn)).Write([]byte("TRANSACTION " + id + " " + timestamp + " " + src + " " + dest + " " + amt + "\n"))
		}
		return true
	})
	// ####################################

	// add block to verified account ledger

	// for _, transaction := range blk.transactions {
	// 	if transaction.Source != 0 {
	// 		balances[transaction.Source] -= transaction.Amount
	// 	}
	// 	if _, ok := balances[transaction.Destination]; ok {
	// 		balances[transaction.Destination] += transaction.Amount
	// 	} else {
	// 		balances[transaction.Destination] = transaction.Amount
	// 	}

	// 	working.Delete(transaction.Id)
	// }
	balances = workingBalances
	workingBalances = make(map[int]int)

	balancesMutex.Unlock()

	blockMutex.Lock()

	blocks[block_hash] = block_data

	blockMutex.Unlock()
	// remove transactions from working copy

	// work on new block

	// fmt.Println("go createBlock")
	go createBlock(conn)

}

func main() {
	// fmt.Println("Entering main...")
	gob.Register(Transaction{})
	gob.Register(Introduction{})
	gob.Register(Block{})
	serviceAddress = "sp19-cs425-g31-09.cs.illinois.edu" // Totally not hardcoded
	servicePort = "4444"                                 // Hardcoded

	localPort = os.Args[1]
	// TODO: Setup Gossip Listener
	// go gossipListener()

	balances = make(map[int]int)
	blocks = make(map[string]Block)

	chan_solved = make(chan string)
	quit_block = make(chan string)

	blockMutex = &sync.Mutex{}
	balancesMutex = &sync.Mutex{}

	blockLength = 0
	lengthMutex = &sync.Mutex{}

	prevBlockHash = "0"
	prevHashMutex = &sync.Mutex{}

	numCommittedTransactions = 0
	committedMutex = &sync.Mutex{}

	// Setup listener for connections
	portStr := ":" + localPort
	ln, err := net.Listen("tcp", portStr)
	if err != nil {
		panic(err)
	}

	c = SafeCounter{numTransactions: 0}
	// balanceMutex = SafeCounter{numTransactions: 0}

	// No blocks have been mined yet
	//globalBlockNum := 0

	// Connect to Service VM on launch
	serviceConn := handleServiceConnection(serviceAddress, servicePort, localPort)

	// Accept all incoming connections
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			numListens++

			// fmt.Println("NumListens: ", numListens)
			nodeName := conn.RemoteAddr().String()
			// fmt.Println("Nodename: ", nodeName, "\n")

			dec := gob.NewDecoder(conn)
			connDecoders.Store(nodeName, dec)

			go listenForUpdates(nodeName, serviceConn)
		}
	}()

	go createBlock(serviceConn)

	runtime.Goexit()
	// fmt.Println("Exit")
}

// This function is called only once for initial connection to Service
func handleServiceConnection(serviceAddress string, servicePort string, localPort string) net.Conn {
	// fmt.Println("Entering handleServiceConnection...")

	serviceRemote := serviceAddress + ":" + servicePort

	serviceConn, err := net.Dial("tcp", serviceRemote)

	if err != nil {
		panic(err)
	}

	//localAddress := strings.Split(conn.LocalAddr().String(), ":")[0]

	// Setup and send message to Service
	hostname, _ := os.Hostname()
	msgToSend := "CONNECT " + hostname + ":" + localPort
	//fmt.Println("SERVICEMSG: ", msgToSend)
	serviceConn.Write([]byte(msgToSend + "\n")) //fmt.Fprintf(conn, msgToSend)

	// dec := gob.NewDecoder(conn)

	go serviceListener(serviceConn)

	return serviceConn

}

// A server listening to incoming messages from Service
func serviceListener(conn net.Conn) {
	// fmt.Println("Entering serviceListener...")

	// listen for reply
	scannerConn := bufio.NewScanner(conn)
	// fmt.Println("Listening for service response")

	for scannerConn.Scan() {
		msgReceived := scannerConn.Text()
		// println("Message received: " + msgReceived)

		// Parse message
		cmd := strings.Split(msgReceived, " ")[0]

		// print("cmd: ", cmd)

		// launch TCP connection to node you're introduced to
		if cmd == "INTRODUCE" {

			// this is flawless
			getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
			fmt.Println(getTime, msgReceived, "DEBUG1")
			introducedName := strings.Split(msgReceived, " ")[1]
			// introducedAddress := strings.Split(msgReceived, " ")[2]
			// introducedPort := strings.Split(msgReceived, " ")[3]

			go connectToIntroduced(introducedName, conn)

			// Received transaction  from service
		} else if cmd == "TRANSACTION" {
			timestamp := strings.Split(msgReceived, " ")[1]
			id := strings.Split(msgReceived, " ")[2]
			src := strings.Split(msgReceived, " ")[3]
			dest := strings.Split(msgReceived, " ")[4]
			amt := strings.Split(msgReceived, " ")[5]

			src_int, _ := strconv.Atoi(src)
			amt_int, _ := strconv.Atoi(amt)
			dest_int, _ := strconv.Atoi(dest)
			timestamp_float, _ := strconv.ParseFloat(timestamp, 64)

			if src != "0" {

				// strconv.ParseFloat(f, 32)
				// print("Val: ", val)
				//print("Amt Int: ", amt_int, "\n")
				//print("Amt: ", amt, "\n")

				val, ok := balances[src_int]

				// print("Source: ", src, "\n")
				// print("Val: ", val, "\n")

				if ok && val >= amt_int {

					balances[src_int] -= amt_int
					if _, ok := balances[dest_int]; ok {
						balances[dest_int] += amt_int
					} else {
						balances[dest_int] = amt_int
					}

					transaction := Transaction{Id: id, Timestamp: timestamp_float, Source: src_int, Destination: dest_int, Amount: amt_int}

					transactions.Store(id, transaction)
					getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
					println(getTime, "RECEIVED VALID TRANSACTION", conn.RemoteAddr().String(), msgReceived)
					fmt.Println(getTime, "RECEIVING", conn.RemoteAddr().String(), msgReceived)

					// Increment global counter for number of transactions
					go c.Inc()

					// gob_data := GobTransport{Type: "Transaction", Data: interface{}(transaction)}

					// Send transaction to everyone [except self!]
					// introducedConnections.Range(func(key interface{}, value interface{}) bool {

					// 	hostname, _ := os.Hostname()
					// 	if hostname+":"+localPort != key {
					// 		print("Gossiping transaction \n")
					// 		encoder_interface, _ := connEncoders.Load(key)
					// 		fmt.Println("Load successful")
					// 		encoder := encoder_interface.(*gob.Encoder)
					// 		// Try until serialization successful
					// 		for {
					// 			fmt.Println("Before encode")
					// 			err := encoder.Encode(gob_data)
					// 			fmt.Println("After encode")
					// 			if err == nil {
					// 				break
					// 			}
					// 			fmt.Println(err)
					// 		}
					// 		fmt.Println("Encoding successful")

					// 		getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
					// 		fmt.Println(getTime, "SENDING", (value.(net.Conn)).RemoteAddr().String(), msgReceived)
					// 		fmt.Println("Transaction Alert: new transaction")
					// 		// 							transaction := Transaction{id: id, timestamp: timestamp, source: src, destination: dest, amount: amt}
					// 		// (value.(net.Conn)).Write([]byte("TRANSACTION " + id + " " + timestamp + " " + src + " " + dest + " " + amt + "\n"))
					// 	}
					// 	return true
					// })
				}

				// If transaction from source "0"
			} else {
				// Deposit money in destination
				if _, ok := balances[dest_int]; ok {
					balances[dest_int] += amt_int
				} else {
					balances[dest_int] = amt_int
				}

			}

		} else if cmd == "SOLVED" {
			chan_solved <- msgReceived

		} else if cmd == "QUIT" {
			getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
			fmt.Println(getTime, "QUIT")
			os.Exit(0)
		} else if cmd == "DIE" {
			getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
			fmt.Println(getTime, "DIE")
			os.Exit(1)
		}

	}
}

// Connects and stores the connection instances for every introduced node
func connectToIntroduced(nodeName string, serviceConn net.Conn) {
	// fmt.Println("Entering connectToIntroduced...")

	// Don't connect to yourself!
	hostname, _ := os.Hostname()
	if hostname+":"+localPort == nodeName {
		return
	}

	// Check if we are already connected
	if _, ok := introducedConnections.Load(nodeName); ok {
		return
	}

	conn, err := net.Dial("tcp", nodeName)
	//print("Connecting to Introduced \n")
	// Don't panic! Return gracefully, important for thanos
	if err != nil {
		// fmt.Println(err)
		return
	}

	///dec := gob.NewDecoder(conn)

	getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
	fmt.Println(getTime, "Connecting to: ", nodeName)
	introducedConnections.Store(nodeName, conn)

	// dec := gob.NewDecoder(conn)
	// connDecoders.Store(nodeName, dec)

	enc := gob.NewEncoder(conn)
	connEncoders.Store(nodeName, enc)

	//connectedAddrs.Store(nodeAddress+":"+nodePort, nodeName)
	//hostname, _ := os.Hostname()
	//connectedAddrs.Store(strings.Split(conn.LocalAddr().String(), ":")[0]+":"+localPort, hostname+localPort)
	//fmt.Println("Stored connection instance")

	// Add yourself to the connections list
	sentinelConnection := conn
	introducedConnections.Store(hostname+":"+localPort, sentinelConnection)

	dec_self := gob.NewDecoder(conn)
	connDecoders.Store(nodeName, dec_self)

	enc_self := gob.NewEncoder(conn)
	connEncoders.Store(nodeName, enc_self)

	encoder_interface, _ := connEncoders.Load(nodeName)
	encoder := encoder_interface.(*gob.Encoder)

	transactions.Range(func(key interface{}, value interface{}) bool {
		// fmt.Println("Transaction Alert: old transactions")
		getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
		fmt.Printf(getTime, "%s SENDING %s TRANSACTION %s %v\n", getTime, conn.RemoteAddr().String(), key.(string), value.(Transaction))
		// fmt.Println(getTime, "SENDING", conn.RemoteAddr().String(), "TRANSACTION", key.(string)+" "+value.(string))
		transaction := value.(Transaction)

		gob_data := GobTransport{Type: "Transaction", Data: interface{}(transaction)}
		// encoder := gob.NewEncoder(conn)

		// Try until serialization successful
		for {
			err := encoder.Encode(gob_data)
			if err == nil {
				// print("Gossiped transaction successfully \n")
				break
			}
			// fmt.Println("Error in sending transaction: ", err, "\n")
		}

		// introducedConnections.Range(func(key interface{}, value interface{}) bool {
		// 	// fmt.Println("Transaction Alert: connectedAddrs")
		// 	//fmt.Println("BROADCAST ", key.(string))
		// 	// conn.Write([]byte("INTRODUCE " + key.(string) + "\n"))

		// 	transaction := value.(Transaction)

		// 	encoder := gob.NewEncoder(conn)

		// 	// Try until serialization successful
		// 	for {
		// 		err := encoder.Encode(transaction)
		// 		if err == nil {
		// 			break
		// 		}
		// 	}

		// 	return true
		// })

		// conn.Write([]byte("TRANSACTION " + key.(string) + " " + value.(string) + "\n"))
		return true
	})
	// fmt.Println("Sent " + nodeAddress + ":" + nodePort + " list of previous transactions")
	// encoder := gob.NewEncoder(conn)
	introducedConnections.Range(func(key interface{}, value interface{}) bool {
		// fmt.Println("Transaction Alert: connectedAddrs")
		//fmt.Println("BROADCAST ", key.(string))
		// conn.Write([]byte("INTRODUCE " + key.(string) + "\n"))

		introduction := Introduction{Addrs: key.(string)}
		getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
		fmt.Println(getTime, "Sending introduction: ", key.(string))
		gob_data := GobTransport{Type: "Introduction", Data: introduction}

		// Try until serialization successful
		for {
			err := encoder.Encode(gob_data)
			if err == nil {
				break
			}
			// fmt.Println("Error in sending introduction: ", err, "\n")
		}

		// // Flush
		// gob_data = GobTransport{Type: "Flush", Data: "NULL"}

		// // encoder = gob.NewEncoder(conn)

		// // Try until serialization successful
		// for {
		// 	err := encoder.Encode(gob_data)
		// 	if err == nil {
		// 		break
		// 	}
		// }

		return true
	})

	go listenForUpdates(nodeName, serviceConn)
}

// func acceptBlock(block Block) {
// 	// do you drop a block or accept block on receiving?
// 	// do this syncronously for all incoming blocks - no thread problems...
// 	// need to consider current block and blockchain

// 	blockMutex.Lock()
// 	currBlock := currentBlock
// 	blockMutex.Unlock()

// 	block_sol_hash = generateHash(block.prevHash, block.transactions)
// 	return verifyBlockWithService(block.prevHash, block_sol_hash)
// }

// To receive updates from other VM's
func listenForUpdates(nodeName string, serviceConn net.Conn) {
	// fmt.Println("Entering listenForUpdates...")

	dec_interface, _ := connDecoders.Load(nodeName)
	dec := dec_interface.(*gob.Decoder)
	for {
		//dec := gob.NewDecoder(conn)

		var gob_data GobTransport
		err := dec.Decode(&gob_data)

		if err != nil {
			// panic(err)
			return
		}
		getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
		fmt.Println(getTime, "Received Gob: ", gob_data.Type)
		// Received introduction
		if gob_data.Type == "Introduction" {
			getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
			fmt.Println(getTime, "Received introduction: ", gob_data.Data.(Introduction).Addrs)
			//x := randomConnGenerator(0, 1, 0, 100)
			//if x == 1 {
			go connectToIntroduced(gob_data.Data.(Introduction).Addrs, serviceConn)
			//}

		} else if gob_data.Type == "Block" {
			// print("Received Block \n")

			block := gob_data.Data.(Block)

			blockchain := []string{block.HashSolution}

			block_hash := generateHash(block.PrevHash, block.Transactions)

			if verifyBlockWithService(block_hash, block.HashSolution, serviceConn) {
				blockMutex.Lock()
				blocks[block.HashSolution] = block // add the block to the global block map
				// blockchain := []string{block.HashSolution}
				prev_hash := block_hash
				for prev_hash != "0" {
					prev_hash = blocks[prev_hash].PrevHash
					blockchain = append([]string{prev_hash}, blockchain...)
				}
				blockMutex.Unlock()
			}

			ledger := make(map[int]int)

			for _, hash := range blockchain {
				if hash != "0" {
					blockMutex.Lock()
					for _, transaction := range blocks[hash].Transactions {
						if transaction.Source != 0 {
							//src := strconv.Itoa(transaction.Source)
							if val, ok := ledger[transaction.Source]; ok && val > transaction.Amount {
								ledger[transaction.Source] = val - transaction.Amount
							} else {
								panic("Uh oh, there is not enough money!")
							}
						}

						if val, ok := ledger[transaction.Destination]; ok {
							ledger[transaction.Destination] = val + transaction.Amount
						} else {
							ledger[transaction.Destination] = transaction.Amount
						}

					}
					blockMutex.Unlock()
				}
			}

			// if longer, switch to this block and work off this block

			lengthMutex.Lock()
			length := blockLength
			lengthMutex.Unlock()

			if len(blockchain) > length {
				balancesMutex.Lock()
				balances = ledger
				balancesMutex.Unlock()
			}

			committedMutex.Lock()
			numCommittedTransactions += len(block.Transactions)
			committedMutex.Unlock()

			prevHashMutex.Lock()
			prevBlockHash = block_hash
			prevHashMutex.Unlock()

			gob_data := GobTransport{Type: "Block", Data: block}

			// Gossip block to connections
			connEncoders.Range(func(key interface{}, value interface{}) bool {
				encoder := value.(*gob.Encoder)

				// try until serialization successful
				for {
					err := encoder.Encode(gob_data)

					// if TCP connection has error (failed)
					if err != nil {
						// // parse out IP address of user connected to
						// parts := strings.Split(c.RemoteAddr().String(), ":")
						// // retrieve chat alias of user and display user has left
						// value, ok := vm_names.Load(parts[0])
						// if ok {
						//   fmt.Println(value, "has left")
						// }

						// TODO: Parse block contents
						// block_data.prevHash...
						// block_data_transactions...
						// block_data.hashSolution...

						// return
					}
				}
				return true
			})

			select {
			case quit_block <- "QUIT!":
				go createBlock(serviceConn)
				// fmt.Println("Restarted createBlock")
			case <-time.After(time.Second):
				// fmt.Println("Haha failed")
				continue
			}

		} else if gob_data.Type == "Transaction" {
			// print("Received gossiped transaction \n")

			transaction := gob_data.Data.(Transaction)
			transactions.Store(transaction.Id, transaction)
		} else {
			// print("Don't know what I just received \n")
			continue
		}

		// ##################################################################
		// transaction_data := Transaction{}
		// err_transaction := dec.Decode(&transaction_data)
		// // Received transaction
		// if err_transaction == nil {
		// 	print("Received gossiped transaction \n")
		// 	if _, ok := transactions.LoadOrStore(transaction_data.Id, transaction_data); !ok {
		// 		introducedConnections.Range(func(key interface{}, value interface{}) bool {

		// 			hostname, _ := os.Hostname()
		// 			if hostname+":"+localPort != key {

		// 				// // fmt.Println("Transaction Alert: new transaction")
		// 				// getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
		// 				// fmt.Println(getTime, "SENDING", (value.(net.Conn)).RemoteAddr().String(), msgReceived)
		// 				// transaction := Transaction{id: id, timestamp: timestamp, source: src, destination: dest, amount: amt}
		// 				// (value.(net.Conn)).Write([]byte("TRANSACTION " + id + " " + timestamp + " " + src + " " + dest + " " + amt + "\n"))

		// 				encoder := gob.NewEncoder(value.(net.Conn))

		// 				// Try until serialization successful
		// 				for {
		// 					err := encoder.Encode(transaction_data)
		// 					if err != nil {
		// 						// return
		// 					}
		// 				}

		// 				// getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
		// 				//fmt.Println(getTime, "SENDING", (value.(net.Conn)).RemoteAddr().String(), msgReceived)

		// 			}
		// 			return true
		// 		})
		// 	}

		// }

	}
}

// go func() {
// 	// Spawn thread to listen for introductions, transactions
// 	for {
// 		scannerConn := bufio.NewScanner(conn)
// 		for scannerConn.Scan() {
// 			msgReceived := scannerConn.Text()
// 			cmd := strings.Split(msgReceived, " ")[0]
// 			if cmd == "INTRODUCE" {
// 				getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
// 				fmt.Println(getTime, msgReceived, "DEBUG2")
// 				introducedName := strings.Split(msgReceived, " ")[1]
// 				// introducedAddr := strings.Split(msgReceived, " ")[2]
// 				// introducedAddress := strings.Split(introducedAddr, ":")[0]
// 				// introducedPort := strings.Split(introducedAddr, ":")[1]

// 				// Randomly connect to several
// 				x := randomConnGenerator(0, 1, 0, 100)
// 				if x == 1 {
// 					go connectToIntroduced(introducedName)
// 				}

// 				// Everytime you receive a transaction you 'gossip' it to your connections
// 			}
// 		}
// 	}
// }

// 			else if cmd == "TRANSACTION" {
// 				// log.Println("Transaction received: " + msgReceived)
// 				// 					timestamp := strings.Split(msgReceived, " ")[2]
// 				// 					id := strings.Split(msgReceived, " ")[1]
// 				// 					src := strings.Split(msgReceived, " ")[3]
// 				// 					dest := strings.Split(msgReceived, " ")[4]
// 				// 					amt := strings.Split(msgReceived, " ")[5]

// 				// 					transaction := Transaction{id: id, timestamp: timestamp, source: src, destination: dest, amount: amt}

// 				// 					getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
// 				// 					fmt.Println(getTime, "RECEIVING", conn.RemoteAddr().String(), msgReceived)

// 				// 					if _, ok := transactions.LoadOrStore(id, transaction); !ok {
// 				// 						introducedConnections.Range(func(key interface{}, value interface{}) bool {

// 				// 							hostname, _ := os.Hostname()
// 				// 							if hostname+":"+localPort != key {

// 				// 								// fmt.Println("Transaction Alert: new transaction")
// 				// 								getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
// 				// 								fmt.Println(getTime, "SENDING", (value.(net.Conn)).RemoteAddr().String(), msgReceived)
// 				// 								transaction := Transaction{id: id, timestamp: timestamp, source: src, destination: dest, amount: amt}
// 				// 								(value.(net.Conn)).Write([]byte("TRANSACTION " + id + " " + timestamp + " " + src + " " + dest + " " + amt + "\n"))
// 				// 							}
// 				// 							return true
// 				// 						})
// 				// 					}

// 				dec := gob.NewDecoder(conn)
// 				for {
// 					// dec := gob.NewDecoder(conn)
// 					transaction_data := Transaction{}

// 					err := dec.Decode(&transaction_data)

// 					if err == nil {
// 						// Store transaction here

// 						if _, ok := transactions.LoadOrStore(transaction_data.Id, transaction_data); !ok {
// 							introducedConnections.Range(func(key interface{}, value interface{}) bool {

// 								hostname, _ := os.Hostname()
// 								if hostname+":"+localPort != key {

// 									// // fmt.Println("Transaction Alert: new transaction")
// 									// getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
// 									// fmt.Println(getTime, "SENDING", (value.(net.Conn)).RemoteAddr().String(), msgReceived)
// 									// transaction := Transaction{id: id, timestamp: timestamp, source: src, destination: dest, amount: amt}
// 									// (value.(net.Conn)).Write([]byte("TRANSACTION " + id + " " + timestamp + " " + src + " " + dest + " " + amt + "\n"))

// 									encoder := gob.NewEncoder(value.(net.Conn))

// 									// Try until serialization successful
// 									for {
// 										err := encoder.Encode(transaction_data)
// 										if err != nil {
// 											// return
// 										}
// 									}

// 									getTime := time.Now().UTC().Format("2006/01/02 15:04:05.000000")
// 									fmt.Println(getTime, "SENDING", (value.(net.Conn)).RemoteAddr().String(), msgReceived)

// 								}
// 								return true
// 							})
// 						}
// 					}

// 				}
// 			}
// 		}
// 	}
// }()
//}

func generateHash(block_prev_hash string, block_transactions []Transaction) string {
	// fmt.Println("Entering generateHash...")

	var transactions_string strings.Builder

	transactions_string.WriteString(block_prev_hash)
	transactions_string.WriteString(" ")

	for _, block_transaction := range block_transactions {

		transactions_string.WriteString(block_transaction.Id)
		transactions_string.WriteString(" ")

	}

	transactions_string.WriteString("\n")

	fin_transaction_string := transactions_string.String()

	// hash_gen := sha256.New()
	// hash_gen.Write([]byte(fin_transaction_string))

	// fin_hash := string(hash_gen.Sum(nil))
	// return fin_hash

	// Create a new HMAC by defining the hash type and the key (as byte array)
	h := hmac.New(sha256.New, []byte(fin_transaction_string))

	// Write Data to it
	//h.Write([]byte(data))

	// Get result and encode as hexadecimal string
	sha := hex.EncodeToString(h.Sum(nil))

	return sha

}

func verifyBlockWithService(block_hash string, block_sol_hash string, serviceConn net.Conn) bool {
	// fmt.Println("Entering verifyBlockWithService...")

	// msgToSend := "VERIFY " + block_hash + ":" + block_sol_hash
	// fmt.Println("SERVICEMSG: ", msgToSend)

	// for {
	// 	_, err := serviceConn.Write([]byte(msgToSend + "\n"))
	// 	if err == nil {
	// 		break
	// 	}
	// }

	// scannerConn := bufio.NewScanner(serviceConn)

	// for scannerConn.Scan() {
	// 	msgReceived := scannerConn.Text()
	// 	cmd := strings.Split(msgReceived, " ")[0]
	// 	if cmd == "VERIFY" {

	// 		success := strings.Split(msgReceived, " ")[1]

	// 		if success == "OK" {
	// 			return true
	// 		} else {
	// 			return false
	// 		}
	// 	}
	// }
	return false
}

func randomConnGenerator(x int, y int, px int, py int) int {
	// fmt.Println("Entering randomConnGenerator...")

	r := rand.Intn(101)

	if r <= px {
		return x
	} else {
		return y
	}
}

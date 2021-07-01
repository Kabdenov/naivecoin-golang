package main

import (
	"encoding/json"
	"fmt"
	"log"
	"naivecoin/blockchain"
	p2p "naivecoin/p2p"
	"naivecoin/wallet"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
)

// port for wallet api requests
var httpPort int = 8080

// addPeer adds a new peer to peer list
func addPeer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	peerAddress := vars["peerAddress"]
	err := p2p.AddPeer(peerAddress)
	w.Header().Set("Content-Type", "application/json")
	if err == nil {
		json.NewEncoder(w).Encode("success")
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// mineBlock mines a new block built with transactions in a transaction pool
// also includes coinbase transaction
func mineBlock(w http.ResponseWriter, r *http.Request) {
	block, err := blockchain.ProduceNextBlock()
	w.Header().Set("Content-Type", "application/json")
	if err == nil {
		json.NewEncoder(w).Encode(block)
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// getBlocks returns all blocks in a blockchain
func getBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blockchain.GetBlockChain())
}

// lastBlock returns the latest block in a blockchain
func lastBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blockchain.GetLatestBlock())
}

// getBalance returns a sum of all unspent transactions for current wallet
func getBalance(w http.ResponseWriter, r *http.Request) {
	balance := blockchain.GetAccountBalance()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balance)
}

// sendTx creates a new transaction, adds it into transaction pool and broadcasts it to peers
func sendTx(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	amount := vars["amount"]
	amountFloat, parseFloatError := strconv.ParseFloat(amount, 64)

	w.Header().Set("Content-Type", "application/json")
	if parseFloatError != nil {
		http.Error(w, parseFloatError.Error(), http.StatusBadRequest)
		return
	}

	tx, sendCoinsError := blockchain.SendTransaction(address, amountFloat)
	if sendCoinsError == nil {
		json.NewEncoder(w).Encode(tx)
	} else {
		http.Error(w, sendCoinsError.Error(), http.StatusBadRequest)
	}
}

// sendCoins creates a new transaction, adds it into a block, then mines this block and broadcasts it to peers
func sendCoins(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	amount := vars["amount"]
	amountFloat, parseFloatError := strconv.ParseFloat(amount, 64)

	w.Header().Set("Content-Type", "application/json")
	if parseFloatError != nil {
		http.Error(w, parseFloatError.Error(), http.StatusBadRequest)
		return
	}

	block, sendCoinsError := blockchain.SendCoinsToAddress(address, amountFloat)
	if sendCoinsError == nil {
		json.NewEncoder(w).Encode(block)
	} else {
		http.Error(w, sendCoinsError.Error(), http.StatusBadRequest)
	}
}

// unspentTxOuts returns unspent transactions for a blockchain
func unspentTxOuts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blockchain.GetUnspentTxOuts())
}

// https://www.golangprograms.com/how-to-use-wildcard-or-a-variable-in-our-url-for-complex-routing.html
func initHttpServer() {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/api/unspentTxOuts", unspentTxOuts)
	rtr.HandleFunc("/api/blocks", getBlocks)
	rtr.HandleFunc("/api/lastBlock", lastBlock)
	rtr.HandleFunc("/api/balance", getBalance)
	rtr.HandleFunc("/api/sendCoins/{address}/{amount}", sendCoins)
	rtr.HandleFunc("/api/sendTx/{address}/{amount}", sendTx)
	rtr.HandleFunc("/api/mineBlock", mineBlock)
	rtr.HandleFunc("/api/addPeer/{peerAddress}", addPeer)

	http.Handle("/", rtr)

	http.HandleFunc("/ws", p2p.WsEndpoint)
	http.HandleFunc("/p2p", p2p.P2pEndpoint)

	fmt.Printf("listening on port %d\n", httpPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil))
}

func main() {
	args := os.Args
	if len(args) == 2 {
		if portNumber, err := strconv.Atoi(args[1]); err == nil {
			httpPort = portNumber
		}
	}
	blockchain.SetNetwork(p2p.Network{})
	wallet.InitWallet()
	fmt.Printf("Your address: %s\n", wallet.GetBase58Address())
	initHttpServer()
}

package p2p

import (
	"encoding/json"
	"fmt"
	"log"
	"naivecoin/blockchain"
	tx "naivecoin/transactions"
	"naivecoin/txpool"
	"naivecoin/wallet"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// A single mutex to be used by both goroutines
var peerSocketListLock sync.Mutex
var peerSendLock sync.Mutex
var webClientSocketLock sync.Mutex
var webClientSendLock sync.Mutex

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// message codes used to distinguish between different message types received from peers
const (
	txPoolMsg         = "TX_POOL"
	blockchainMsg     = "BLOCKCHAIN"
	getLatestBlockMsg = "GET_LATEST_BLOCK"
	getAllBlocksMsg   = "GET_ALL_BLOCKS"
	getTxPoolMsg      = "GET_TX_POOL"
	walletInfoMsg     = "WALLET_INFO"
)

// Message struct to hold data and message code
type Message struct {
	Code string
	Data interface{}
}

// peers is the list of connections to peers
var peers []*websocket.Conn = []*websocket.Conn{}

// webClientSocket is a connection to web client
var webClientSocket *websocket.Conn

// getPeers returns the list of connected peers
func getPeers() []*websocket.Conn {
	return peers
}

// Network struct used by blockhain package to access BroadcastTransactionPool and BroadcastLatest functions
type Network struct{}

// BroadcastTransactionPool broadcasts transaction pool to all connected peers
// also sends an update to web client
func (Network) BroadcastTransactionPool() {
	broadcast(txpool.GetTransactionPool(), txPoolMsg)
	sendUpdateToWebClient()
}

// BroadcastLatest broadcasts the latest block in a blockchain to all connected peers
// also sends an update to web client
func (Network) BroadcastLatest() {
	latestBlock := []blockchain.Block{blockchain.GetLatestBlock()}
	broadcast(latestBlock, blockchainMsg)
	sendUpdateToWebClient()
}

// buildMessage builds a message to be sent later to websockets
func buildMessage(data interface{}, code string) ([]byte, error) {
	var msg Message = Message{
		Code: code,
		Data: data,
	}

	dataBytes, err := json.Marshal(msg)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return dataBytes, nil
}

// broadcast broadcasts data to all peers
func broadcast(data interface{}, code string) {

	dataBytes, err := buildMessage(data, code)
	if err != nil {
		return
	}

	peerSocketListLock.Lock()
	for _, socket := range peers {
		send(socket, dataBytes)
	}
	peerSocketListLock.Unlock()
}

// send sends byte data to a websocket
func send(ws *websocket.Conn, dataBytes []byte) {
	peerSendLock.Lock()
	err := ws.WriteMessage(websocket.TextMessage, dataBytes)
	peerSendLock.Unlock()
	if err != nil {
		log.Println(err)
	}
}

// unmarshalDtoToBlocks unmarshales dto to a collection of blocks
func unmarshalDtoToBlocks(byteData []byte) ([]blockchain.Block, error) {
	blocks := &[]blockchain.Block{}
	dto := Message{Data: blocks}
	err := json.Unmarshal(byteData, &dto)
	return *blocks, err
}

// unmarshalDtoToTxPool unmarshales dto to a collection of transactions
func unmarshalDtoToTxPool(byteData []byte) ([]tx.Transaction, error) {
	txs := &[]tx.Transaction{}
	dto := Message{Data: txs}
	err := json.Unmarshal(byteData, &dto)
	return *txs, err
}

// handleReceivedBlocks handles received blocks: replaces chain if received blocks are valid
func handleReceivedBlocks(blocks []blockchain.Block) {
	if len(blocks) == 0 {
		return
	}
	var latestBlockReceived blockchain.Block = blocks[len(blocks)-1]
	blockchain.Lock.Lock()
	var latestBlockHeld blockchain.Block = blockchain.GetLatestBlock()
	blockchain.Lock.Unlock()

	if latestBlockReceived.Fields.Index > latestBlockHeld.Fields.Index {
		if latestBlockHeld.Hash == latestBlockReceived.Fields.PrevHash {
			blockchain.Lock.Lock()
			if blockchain.AddBlockToChain(latestBlockReceived) {
				latestBlock := []blockchain.Block{blockchain.GetLatestBlock()}
				broadcast(latestBlock, blockchainMsg)
			}
			blockchain.Lock.Unlock()
		} else if len(blocks) == 1 {
			fmt.Println("some blocks are missing, requesting all blocks")
			broadcast(nil, getAllBlocksMsg)
		} else {
			blockchain.Lock.Lock()
			blockchain.ReplaceChain(blocks)
			blockchain.Lock.Unlock()
		}
	}
}

// handleMessage handles messages received through webscoket connection
func handleMessage(ws *websocket.Conn, code string, messageBytes []byte) {
	switch code {

	// handle a case when peer requests latest block in a blockchain
	case getLatestBlockMsg:
		responseBytes, err := buildMessage([]blockchain.Block{blockchain.GetLatestBlock()}, blockchainMsg)
		if err != nil {
			log.Println(err)
			return
		}
		send(ws, responseBytes)

	// handle a case when peer requests all blocks in a blockchain
	case getAllBlocksMsg:
		responseBytes, err := buildMessage(blockchain.GetBlockChain(), blockchainMsg)
		if err != nil {
			log.Println(err)
			return
		}
		send(ws, responseBytes)

	// handle a case when peer sends a list of blocks
	case blockchainMsg:
		fmt.Println("blockchain received")
		blocks, err := unmarshalDtoToBlocks(messageBytes)
		if err != nil {
			log.Println(err)
			return
		}
		handleReceivedBlocks(blocks)

	// handle a case when peer requests a list of transactions in transaction pool
	case getTxPoolMsg:
		responseBytes, err := buildMessage(txpool.GetTransactionPool(), txPoolMsg)
		if err != nil {
			log.Println(err)
			return
		}
		send(ws, responseBytes)

	// handle a case when peer send a list of transactions in his transaction pool
	case txPoolMsg:
		fmt.Println("tx pool received")
		txs, err := unmarshalDtoToTxPool(messageBytes)
		if err != nil {
			log.Println(err)
			return
		}
		for _, tx := range txs {
			err := blockchain.HandleReceivedTransaction(tx)
			if err == nil {
				broadcast(txpool.GetTransactionPool(), txPoolMsg)
			}
		}

	default:
		log.Printf("unsupported message code: %s", code)
	}

	sendUpdateToWebClient()
}

// sendUpdateToWebClient sends current wallet balance and wallet address to connected web client
func sendUpdateToWebClient() {
	if webClientSocket == nil {
		return
	}

	walletInfo := struct {
		Balance float64
		Address string
	}{
		Balance: blockchain.GetAccountBalance(),
		Address: wallet.GetBase58Address(),
	}

	dataBytes, _ := buildMessage(walletInfo, walletInfoMsg)
	webClientSendLock.Lock()
	err := webClientSocket.WriteMessage(websocket.TextMessage, dataBytes)
	if err != nil {
		log.Println(err)
	}
	webClientSendLock.Unlock()
}

// removePeerAtIndex removes a peer from the list at a given index
func removePeerAtIndex(sockets_ []*websocket.Conn, index int) {
	peers = append(sockets_[:index], sockets_[index+1:]...)
}

// reader listens for messages on websocket connection and sends them to be handled further
func reader(ws *websocket.Conn) {
	for {
		// read in a message
		messageType, messageBytes, err := ws.ReadMessage()

		if err != nil {
			log.Println(err)
			peerSocketListLock.Lock()
			for index, socket := range peers {
				if socket == ws {
					socket.Close()
					removePeerAtIndex(peers, index)
					break
				}
			}
			peerSocketListLock.Unlock()
			break
		}

		if messageType != websocket.TextMessage {
			log.Println("text message types expected")
			continue
		}

		messageStruct := Message{}

		err = json.Unmarshal(messageBytes, &messageStruct)

		if err != nil {
			log.Println(err)
			continue
		}

		handleMessage(ws, messageStruct.Code, messageBytes)
	}
}

// WsEndpoint starts a websocket connection to web client
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	webClientSocketLock.Lock()
	webClientSocket = ws
	webClientSocketLock.Unlock()
	log.Println("Web client Connected")

	sendUpdateToWebClient()
}

// P2pEndpoint accepts a bidirectional connection from a peer
func P2pEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	peerSocketListLock.Lock()
	peers = append(peers, ws)
	peerSocketListLock.Unlock()

	log.Println("Peer connected")

	go reader(ws)

	getLatestBlockMsgBytes, err := buildMessage(nil, getLatestBlockMsg)
	if err != nil {
		log.Println(err)
	}

	go send(ws, getLatestBlockMsgBytes)

	time.Sleep(500 * time.Millisecond)

	go broadcast(nil, getTxPoolMsg)
}

// AddPeer starts a bidirectional connection from a peer
func AddPeer(peerAddress string) error {
	ws, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/p2p", peerAddress), nil)
	if err != nil {
		return err
	}

	peerSocketListLock.Lock()
	peers = append(peers, ws)
	peerSocketListLock.Unlock()

	log.Println("Peer Connected")

	go reader(ws)

	getLatestBlockMsgBytes, err := buildMessage(nil, getLatestBlockMsg)
	if err != nil {
		log.Println(err)
	}

	go send(ws, getLatestBlockMsgBytes)

	time.Sleep(500 * time.Millisecond)

	go broadcast(nil, getTxPoolMsg)

	return nil
}

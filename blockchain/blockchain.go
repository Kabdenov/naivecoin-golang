package blockchain

import (
	"errors"
	"fmt"
	"math"
	tx "naivecoin/transactions"
	"naivecoin/txpool"
	"naivecoin/utils"
	"naivecoin/wallet"
	"strings"
	"sync"
	"time"
)

// A single mutex to be used by both goroutines
var Lock sync.Mutex

type Network interface {
	BroadcastTransactionPool()
	BroadcastLatest()
}

var p2pNetwork Network

func SetNetwork(net Network) {
	p2pNetwork = net
}

const (
	blockGenerationInterval      uint = 10 // number of seconds
	difficultyAdjustmentInterval uint = 10 // number of blocks
)

// BlockFields defines required fields for a block
type BlockFields struct {
	Index        int
	PrevHash     string
	Ts           uint64
	Transactions []tx.Transaction
	Difficulty   float64
	Nonce        int
}

// Block defines a structure of a block
type Block struct {
	Fields BlockFields
	Hash   string
}

// genesisTransaction is the very first transaction in a blockchain, hardcoded
var GenesisTransaction tx.Transaction = tx.Transaction{
	TxIns: tx.TxInCollection{
		tx.TxIn{},
	},
	TxOuts: tx.TxOutCollection{
		tx.TxOut{
			Address: "S7H2fmjGPxznuu9NPcnYCyEqdg1ebSbMN6AJRqQQo4Z1D1yQdKwEGwiJezSDka6yqHDSb2jqaf3Tewg1tryEbDzG",
			Amount:  50,
		},
	},
	Id: "62530d1bbbf4f75200448207cbc3c84b4b67fe7a85eddf6f5c3e4bbac4461b82",
}

// GenesisBlock is the very first block in a blockchain, hardcoded
var GenesisBlock Block = Block{
	Fields: BlockFields{
		Transactions: []tx.Transaction{GenesisTransaction},
	},
	Hash: "fbf56e4cc6a37936341c07f2d452ee01c93a1bb30d0bfe219d3d2af1cf38f78b",
}

// blockchain holds a chain of blocks, each block is dependant on previous block and must follow a predefined set of rules
var blockchain []Block = []Block{GenesisBlock}

// GetBlockChain returns current blockchain
func GetBlockChain() []Block {
	return blockchain
}

// cumulativeBlocksDifficulty stores accumulated blockchain difficulty for current blockchain
var cumulativeBlocksDifficulty uint64

// unspentTxOuts is the list of unspent txOuts that can be used later by their owners
// TODO: different data structure should be used, because search in an array takes O(n) time
var unspentTxOuts, _ = tx.ProcessTransactions(blockchain[0].Fields.Transactions, []tx.UnspentTxOut{}, 0)

// getUnspentTxOuts returns a deep copy of unspent txOuts
//https://stackoverflow.com/questions/27055626/concisely-deep-copy-a-slice
func getUnspentTxOuts() []tx.UnspentTxOut {
	cpy := make([]tx.UnspentTxOut, len(unspentTxOuts))
	copy(cpy, unspentTxOuts)
	return cpy
}

func GetUnspentTxOuts() []tx.UnspentTxOut {
	return getUnspentTxOuts()
}

// getLatestBlock returns the latest block in a blockchain
func GetLatestBlock() Block {
	return blockchain[len(blockchain)-1]
}

// setUnspentTxOuts updates the list of unspent txOuts with a new one
func setUnspentTxOuts(newUnspentTxOut []tx.UnspentTxOut) {
	unspentTxOuts = newUnspentTxOut
}

// getDifficulty gets required difficulty for a block
// the value of difficulty is used to adjsut how many leading zeros must be in the hash of a block
// this value is used to control proof-of-work based on a number of produced blocks per time period
func getDifficulty(blockchain_ []Block, latestBlock Block) float64 {
	var (
		adjustmentIntervalIsReached bool = latestBlock.Fields.Index%int(difficultyAdjustmentInterval) == 0
		isGenesisBlock              bool = latestBlock.Fields.Index == 0
	)
	if adjustmentIntervalIsReached && !isGenesisBlock {
		return getAdjustedDifficulty(blockchain_, latestBlock)
	} else {
		return latestBlock.Fields.Difficulty
	}
}

// getAdjustedDifficulty returns an adjusted difficulty based on expected time to produce difficultyAdjustmentInterval blocks
func getAdjustedDifficulty(blockchain_ []Block, latestBlock Block) float64 {

	if latestBlock.Fields.Index+1 < int(difficultyAdjustmentInterval) {
		fmt.Println("blockchain length is less than difficulty adjustment interval")
		return 0
	}

	var (
		prevAdjustmentBlock Block  = blockchain_[latestBlock.Fields.Index+1-int(difficultyAdjustmentInterval)]
		timeExpected        uint64 = uint64(blockGenerationInterval * difficultyAdjustmentInterval)
		timeTaken           uint64 = latestBlock.Fields.Ts - prevAdjustmentBlock.Fields.Ts
	)

	// if blocks are produced too frequently, increase difficulty
	if timeTaken < (uint64(timeExpected) / 2) {
		return prevAdjustmentBlock.Fields.Difficulty + 1
		// if block are produced too infrequently, decrease difficulty
	} else if timeTaken > uint64(timeExpected)*2 {
		temp := prevAdjustmentBlock.Fields.Difficulty - 1
		// negative difficulty bug fix
		if temp < 0 {
			return 0
		} else {
			return temp
		}
	}

	return prevAdjustmentBlock.Fields.Difficulty
}

// getMyUnspentTransactionOutputs returns the unspent txOuts owned by the wallet
func getMyUnspentTransactionOutputs() []tx.UnspentTxOut {
	return wallet.FindUnspentTxOuts(wallet.GetBase58Address(), getUnspentTxOuts())
}

// produceBlock produces a new block from a given transaction list
func produceBlock(transactions []tx.Transaction) (Block, error) {
	var lastBlock Block = blockchain[len(blockchain)-1]
	var blockFields BlockFields = BlockFields{
		Index:        lastBlock.Fields.Index + 1,
		PrevHash:     lastBlock.Hash,
		Ts:           uint64(time.Now().Unix()),
		Transactions: transactions,
		Difficulty:   getDifficulty(blockchain, lastBlock),
		Nonce:        0,
	}
	// proof of work
	for {
		var hash string = utils.Hash(blockFields)
		matchesDifficulty, _ := hashMatchesDifficulty(hash, blockFields.Difficulty)
		if matchesDifficulty {
			var newBlock = Block{
				Fields: blockFields,
				Hash:   hash,
			}
			if AddBlockToChain(newBlock) {
				p2pNetwork.BroadcastLatest()
				return newBlock, nil
			} else {
				return Block{}, errors.New("failed to produce valid block")
			}
		}
		//fmt.Printf("%s doesnt match difficulty: %s, retrying with nonce %d\n", hash, requiredPrefix, blockFields.Nonce+1)
		blockFields.Nonce++
	}
}

// ProduceNextBlock produces a new block from transactions in a transaction pool
func ProduceNextBlock() (Block, error) {
	var coinbaseTx tx.Transaction = tx.GetCoinbaseTransaction(wallet.GetBase58Address(), GetLatestBlock().Fields.Index+1)
	var blockData []tx.Transaction = []tx.Transaction{coinbaseTx}
	blockData = append(blockData, txpool.GetTransactionPool()...)
	return produceBlock(blockData)
}

// SendCoinsToAddress creates a new transaction, includes it into a block, finds valid hash and broadcasts new block to peers
func SendCoinsToAddress(base58Address string, amount float64) (Block, error) {
	if amount <= 0 {
		return Block{}, errors.New("invalid amount")
	}

	if !tx.IsValidBase58Address(base58Address) {
		return Block{}, errors.New("invalid address")
	}
	var coinbaseTx tx.Transaction = tx.GetCoinbaseTransaction(wallet.GetBase58Address(), GetLatestBlock().Fields.Index+1)
	var normalTx tx.Transaction = wallet.CreateTransaction(base58Address, amount, getUnspentTxOuts(), txpool.GetTransactionPool())
	var blockData []tx.Transaction = []tx.Transaction{coinbaseTx, normalTx}

	newBlock, err := produceBlock(blockData)
	return newBlock, err
}

// GetAccountBalance returns an account balance for current wallet
func GetAccountBalance() float64 {
	return wallet.GetBalance(wallet.GetBase58Address(), getUnspentTxOuts())
}

// SendTransaction creates a new transaction and broadcasts it to peers (without creating a new block)
func SendTransaction(base58Address string, amount float64) (tx.Transaction, error) {
	if amount <= 0 {
		return tx.Transaction{}, errors.New("invalid amount")
	}
	var tx tx.Transaction = wallet.CreateTransaction(base58Address, amount, getUnspentTxOuts(), txpool.GetTransactionPool())
	err := txpool.AddToTransactionPool(tx, getUnspentTxOuts())
	if err == nil {
		p2pNetwork.BroadcastTransactionPool()
	}
	return tx, err
}

// hashMatchesDifficulty checks if hash has a required number of leading zeroes
func hashMatchesDifficulty(hash string, difficulty float64) (bool, string) {
	hashInBinary, err := utils.HexToBin(hash)
	if err != nil {
		fmt.Println(err.Error())
		return false, ""
	}
	var requiredPrefix string = strings.Repeat("0", int(difficulty))
	return strings.HasPrefix(hashInBinary, requiredPrefix), requiredPrefix
}

// IsValidBlock checks if a given block is valid
func IsValidBlock(blockchain_ []Block, prevBlock Block, block Block) bool {

	var isSuccessor = prevBlock.Fields.Index+1 == block.Fields.Index
	if !isSuccessor {
		fmt.Println("block is not a successor of prev block")
		return false
	}

	var includesPrevBlockHash = prevBlock.Hash == block.Fields.PrevHash
	if !includesPrevBlockHash {
		fmt.Println("block does not include prev block hash")
		return false
	}

	var hashIsValid = utils.Hash(block.Fields) == block.Hash
	if !hashIsValid {
		fmt.Println("block has is not valid")
		return false
	}

	var prevBlockIsGenesisBlock = prevBlock.Fields.Index == 0
	var olderThanPrevBlock = prevBlock.Fields.Ts-60 >= block.Fields.Ts
	var farInTheFuture = block.Fields.Ts-60 >= uint64(time.Now().Unix())

	if !prevBlockIsGenesisBlock && (olderThanPrevBlock || farInTheFuture) {
		fmt.Println("block timestamp is invalid")
		return false
	}

	if getDifficulty(blockchain_, prevBlock) != block.Fields.Difficulty {
		fmt.Println("block difficulty is invalid")
		return false
	}

	matchesDifficulty, _ := hashMatchesDifficulty(block.Hash, block.Fields.Difficulty)
	if !matchesDifficulty {
		fmt.Println("block difficulty does not match")
		return false
	}

	return true
}

// GetCumulativeDifficulty returns a accumulated difficulty for a given blockchain
func GetCumulativeDifficulty(blockchain_ []Block) uint64 {
	var result float64
	for n := 0; n < len(blockchain_); n++ {
		result += math.Pow(2, blockchain_[n].Fields.Difficulty)
	}
	return uint64(result)
}

// IsValidBlockChain checks if a given blockchain is valid
func IsValidBlockChain(blockchain_ []Block) ([]tx.UnspentTxOut, error) {
	// first of all check genesis block
	if fmt.Sprintf("%v", blockchain_[0]) != fmt.Sprintf("%v", GenesisBlock) {
		return []tx.UnspentTxOut{}, errors.New("blockchain is invalid")
	}

	var unspentTxOuts_ []tx.UnspentTxOut = []tx.UnspentTxOut{}
	// then check all other blocks
	for n := 0; n < len(blockchain_); n++ {
		if n != 0 && !IsValidBlock(blockchain_, blockchain_[n-1], blockchain_[n]) {
			return []tx.UnspentTxOut{}, errors.New("blockchain is invalid")
		}

		retValue, err := tx.ProcessTransactions(blockchain_[n].Fields.Transactions, unspentTxOuts_, blockchain_[n].Fields.Index)
		unspentTxOuts_ = retValue

		//fmt.Printf("IsValidBlockChain unspentTxOuts_ after ieration %d: %v\n", n, unspentTxOuts_)

		if err != nil {
			return unspentTxOuts_, err
		}
	}
	return unspentTxOuts_, nil
}

// addBlockToChain adds block to a chain
func AddBlockToChain(newBlock Block) bool {
	if IsValidBlock(blockchain, GetLatestBlock(), newBlock) {
		retVal, err := tx.ProcessTransactions(newBlock.Fields.Transactions, getUnspentTxOuts(), newBlock.Fields.Index)
		if err != nil {
			fmt.Println("block is not valid in terms of transactions")
			return false
		} else {
			blockchain = append(blockchain, newBlock)
			// update cumulative block difficulty
			cumulativeBlocksDifficulty += uint64(math.Pow(2, newBlock.Fields.Difficulty))
			setUnspentTxOuts(retVal)
			txpool.UpdateTransactionPool(unspentTxOuts)
			return true
		}
	}
	return false
}

// ReplaceChain computes accumulated difficulty of new blocks,
// and if greater that existing blockchain's acc difficulty, replaces it
func ReplaceChain(newBlocks []Block) error {
	unspentTxOuts_, err := IsValidBlockChain(newBlocks)
	if err != nil {
		fmt.Println(err.Error())
		return errors.New("received blockchain invalid")
	}

	if cumulativeBlocksDifficulty == 0 {
		cumulativeBlocksDifficulty = GetCumulativeDifficulty(blockchain)
	}
	var newCumulativeBlocksDifficulty = GetCumulativeDifficulty(newBlocks)

	if cumulativeBlocksDifficulty >= newCumulativeBlocksDifficulty {
		return errors.New("received blockchain invalid")
	}

	//fmt.Printf("ReplaceChain unspentTxOuts_: %v\n", unspentTxOuts_)

	fmt.Println("Received blockchain is valid. Replacing current blockchain with received blockchain")
	blockchain = newBlocks
	cumulativeBlocksDifficulty = newCumulativeBlocksDifficulty
	setUnspentTxOuts(unspentTxOuts_)
	txpool.UpdateTransactionPool(unspentTxOuts_)
	p2pNetwork.BroadcastLatest()

	return nil
}

// HandleReceivedTransaction adds received transaction to a transaction pool
func HandleReceivedTransaction(transaction tx.Transaction) error {
	return txpool.AddToTransactionPool(transaction, getUnspentTxOuts())
}

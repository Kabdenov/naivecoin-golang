package transactions

import (
	"errors"
	"fmt"
	"naivecoin/utils"
	"strings"
)

const coinBaseAmount float64 = 50

// TxIn defines structure of an incoming transaction
type TxIn struct {
	TxOutId    string
	TxOutIndex int
	Signature  string
}

// TxInCollection defines a collection of incoming transactions
type TxInCollection []TxIn

// TxIn Content function to return contents of an incoming transaction in a string form
func (t TxIn) Content() string {
	// signature field is left out on purpose, as it will be computed later
	return fmt.Sprintf("%s;%d", t.TxOutId, t.TxOutIndex)
}

// TxInCollection Content function to return contents of a collection of incoming transactions in a string form
func (t TxInCollection) Content() string {
	var result string = ""
	for n := 0; n < len(t); n++ {
		result += t[n].Content()
	}
	return result
}

// TxOut defines structure of an outgoing transaction
type TxOut struct {
	Address string
	Amount  float64
}

// TxOutCollection defines structure of a collection of outgoing transactions
type TxOutCollection []TxOut

// TxOut Content function to return contents of an outgoing transaction in a string form
func (t TxOut) Content() string {
	return fmt.Sprintf("%s;%f", t.Address, t.Amount)
}

// TxOutCollection Content function to return contents of a collection of outgoing transactions in a string form
func (t TxOutCollection) Content() string {
	var result string = ""
	for n := 0; n < len(t); n++ {
		result += t[n].Content()
	}
	return result
}

// UnspentTxOut defines an outgoing transaction that was not spent yet
type UnspentTxOut struct {
	TxOutId    string
	TxOutIndex int
	Address    string
	Amount     float64
}

// Transaction defines a structure of incoming and outgoing transactions
type Transaction struct {
	Id     string
	TxIns  TxInCollection
	TxOuts TxOutCollection
}

// GetTransactionId returns an Id for a transaction based on SHA-256 hash of its contents
func GetTransactionId(transaction Transaction) string {
	return utils.Hash(transaction.TxIns.Content() + ";" + transaction.TxOuts.Content())
}

// validateTxIn validates an incoming transaction, returns true if valid, false otherwise
func validateTxIn(txIn TxIn, transaction Transaction, unspentTxOuts_ []UnspentTxOut) bool {
	// new transaction must reference a previously unspent outgoing transaction
	var referencedUTxOut UnspentTxOut
	var found bool = false
	for _, v := range unspentTxOuts_ {
		if v.TxOutId == txIn.TxOutId && v.TxOutIndex == txIn.TxOutIndex {
			referencedUTxOut = v
			found = true
			break
		}
	}
	if !found {
		fmt.Println("referenced txOut not found: " + txIn.Content())
		return false
	}

	var base58Address string = referencedUTxOut.Address
	var publicKey = utils.Base58Decode(base58Address)
	var isValidSignature bool = utils.VerifySignature(transaction.Id, txIn.Signature, publicKey)
	if !isValidSignature {
		fmt.Printf("invalid txIn signature: %s txId: %s address: %s", txIn.Signature, transaction.Id, referencedUTxOut.Address)
		return false
	}
	return true
}

// getTxInAmount returns an amount of txOut that is referenced by a txIn
func getTxInAmount(txIn TxIn, unspentTxOuts_ []UnspentTxOut) float64 {
	unspentTxOut, err := findUnspentTxOut(txIn.TxOutId, txIn.TxOutIndex, unspentTxOuts_)
	if err != nil {
		return 0
	}
	return unspentTxOut.Amount
}

// findUnspentTxOut find unspent txOuts for a given transaction
func findUnspentTxOut(txOutId string, txOutIndex int, unspentTxOuts_ []UnspentTxOut) (UnspentTxOut, error) {
	for _, v := range unspentTxOuts_ {
		if v.TxOutId == txOutId && v.TxOutIndex == txOutIndex {
			return v, nil
		}
	}
	return UnspentTxOut{}, fmt.Errorf("unspent txOut not found")
}

// GetCoinbaseTransaction returns a coinbase transaction
func GetCoinbaseTransaction(base58Address string, blockIndex int) Transaction {
	var txIn TxIn = TxIn{
		TxOutIndex: blockIndex,
	}

	var txOut TxOut = TxOut{
		Address: base58Address,
		Amount:  coinBaseAmount,
	}

	var t Transaction = Transaction{
		TxIns:  TxInCollection{txIn},
		TxOuts: TxOutCollection{txOut},
	}

	t.Id = GetTransactionId(t)
	return t
}

// ValidateTransaction validates transactions: must have valid id, valid txIn, total txIn amount must be equal to txOut amount
func ValidateTransaction(transaction Transaction, unspentTxOuts_ []UnspentTxOut) bool {
	if GetTransactionId(transaction) != transaction.Id {
		fmt.Println("Invalid tx id: " + transaction.Id)
		return false
	}

	var totalTxInValues float64
	for n := 0; n < len(transaction.TxIns); n++ {
		if !validateTxIn(transaction.TxIns[n], transaction, unspentTxOuts_) {
			fmt.Println("some of the txIns are invalid in tx: " + transaction.Id)
			return false
		}
		totalTxInValues += getTxInAmount(transaction.TxIns[n], unspentTxOuts_)
	}

	var totalTxOutValues float64
	for n := 0; n < len(transaction.TxOuts); n++ {
		totalTxOutValues += transaction.TxOuts[n].Amount
	}

	if totalTxInValues != totalTxOutValues {
		fmt.Println("totalTxOutValues != totalTxInValues in tx: " + transaction.Id)
		return false
	}

	return true
}

// validateCoinbaseTx validates a coinbase transaction: msut have valid id, exactly one txIn and txOut, valid index and amount
func validateCoinbaseTx(transaction Transaction, blockIndex int) bool {
	if GetTransactionId(transaction) != transaction.Id {
		fmt.Println("invalid coinbase tx id: " + transaction.Id)
		return false
	}
	if len(transaction.TxIns) != 1 {
		fmt.Println("one txIn must be specified in the coinbase transaction")
		return false
	}
	if transaction.TxIns[0].TxOutIndex != blockIndex {
		fmt.Println("the txIn signature in coinbase tx must be the block height")
		return false
	}
	if len(transaction.TxOuts) != 1 {
		fmt.Println("invalid number of txOuts in coinbase transaction")
		return false
	}
	if transaction.TxOuts[0].Amount != coinBaseAmount {
		fmt.Println("invalid coinbase amount in coinbase transaction")
		return false
	}
	return true
}

// hasDuplicateTxIns checks if there any duplicates in txIn list
func hasDuplicateTxIns(txIns []TxIn) bool {
	hashmap := make(map[string]bool)
	for n := 0; n < len(txIns); n++ {
		var key string = txIns[n].TxOutId + fmt.Sprint(txIns[n].TxOutIndex)
		if hashmap[key] {
			fmt.Println("has duplicate txIn: " + key)
			return true
		}
		hashmap[key] = true
	}
	return false
}

// validateBlockTransactions validates provided transactions: must have a valid coinbase tx, no duplicates txIns, valid txIns
func validateBlockTransactions(transactions []Transaction, unspentTxOuts_ []UnspentTxOut, blockIndex int) bool {
	if len(transactions) == 0 {
		fmt.Println("the first transaction in the block must be coinbase transaction")
		return false
	}

	var coinbaseTx = transactions[0]
	if !validateCoinbaseTx(coinbaseTx, blockIndex) {
		fmt.Println("invalid coinbase transaction: " + fmt.Sprintf("%v", coinbaseTx))
		return false
	}

	// flatten txIns list
	var txInsFlattened []TxIn = []TxIn{}
	for n := 0; n < len(transactions); n++ {
		txInsFlattened = append(txInsFlattened, transactions[n].TxIns...)
	}
	// check if there are any duplicates
	if hasDuplicateTxIns(txInsFlattened) {
		return false
	}

	// validate all but coinbase transactions
	for n := 1; n < len(transactions); n++ {
		if !ValidateTransaction(transactions[n], unspentTxOuts_) {
			return false
		}
	}

	return true
}

// SignTxIn returns a signature for transaction id, signed by provided private key
func SignTxIn(transaction Transaction, txInIndex int, privateKey string, unspentTxOuts []UnspentTxOut) (string, error) {
	var txIn TxIn = transaction.TxIns[txInIndex]

	referencedUnspentTxOut, err := findUnspentTxOut(txIn.TxOutId, txIn.TxOutIndex, unspentTxOuts)
	if err != nil {
		return "", err
	}

	var referencedBase58Address string = referencedUnspentTxOut.Address
	var referencedPublicKey = utils.Base58Decode(referencedBase58Address)

	if utils.GetPublicKey(privateKey) != referencedPublicKey {
		return "", errors.New("trying to sign an input with private key that does not match the address that is referenced in txIn")
	}

	return utils.GetSignature(transaction.Id, privateKey), nil
}

// updateUnspentTxOuts updates unspent txOut
func updateUnspentTxOuts(transactions []Transaction, unspentTxOuts_ []UnspentTxOut) []UnspentTxOut {
	// each transaction introduces unspent txOuts, which can be used later by their owner
	var newUnspentTxOuts []UnspentTxOut = []UnspentTxOut{}

	// each transaction consumes txIns, that must be removed from the unspent transactions list
	var consumedTxOuts []UnspentTxOut = []UnspentTxOut{}

	for i := 0; i < len(transactions); i++ {
		for j := 0; j < len(transactions[i].TxOuts); j++ {
			unspentTxOut := UnspentTxOut{
				TxOutId:    transactions[i].Id,
				TxOutIndex: j,
				Address:    transactions[i].TxOuts[j].Address,
				Amount:     transactions[i].TxOuts[j].Amount,
			}
			newUnspentTxOuts = append(newUnspentTxOuts, unspentTxOut)
		}

		for j := 0; j < len(transactions[i].TxIns); j++ {
			consumedTxOut := UnspentTxOut{
				TxOutId:    transactions[i].TxIns[j].TxOutId,
				TxOutIndex: transactions[i].TxIns[j].TxOutIndex,
			}
			consumedTxOuts = append(consumedTxOuts, consumedTxOut)
		}
	}

	// filter out consumed txOuts from unspent transactions list
	var resultingUnspentTxOuts []UnspentTxOut = []UnspentTxOut{}
	for i := 0; i < len(unspentTxOuts_); i++ {
		_, err := findUnspentTxOut(unspentTxOuts_[i].TxOutId, unspentTxOuts_[i].TxOutIndex, consumedTxOuts)
		// error is set if given unspent txOut is not found in consumed txOuts list
		if err != nil {
			resultingUnspentTxOuts = append(resultingUnspentTxOuts, unspentTxOuts_[i])
		}
	}

	// add new transactions to resulting unspent txOuts
	resultingUnspentTxOuts = append(resultingUnspentTxOuts, newUnspentTxOuts...)
	return resultingUnspentTxOuts
}

// ProcessTransactions validates all given transactins, and returs an updated list of all unspent txOuts
func ProcessTransactions(transactions []Transaction, unspentTxOuts_ []UnspentTxOut, blockIndex int) ([]UnspentTxOut, error) {
	if !validateBlockTransactions(transactions, unspentTxOuts_, blockIndex) {
		return []UnspentTxOut{}, errors.New("invalid block transactions")
	}
	return updateUnspentTxOuts(transactions, unspentTxOuts_), nil
}

// IsValidAddress validates wallet address: must be of length 130, start with 04, contain only hex characters
// TODO: wallet address better be base58 encoded: shorter, distinct characters
func IsValidBase58Address(base58Address string) bool {
	address := utils.Base58Decode(base58Address)
	if len(address) != 130 {
		fmt.Println(address)
		fmt.Println("invalid public key length")
		return false
	} else if !utils.IsHex(address) {
		fmt.Printf("public key must contain only hex characters: %s\n", address)
		return false
	} else if !strings.HasPrefix(address, "04") {
		fmt.Println("public key must start with 04")
		return false
	}
	return true
}

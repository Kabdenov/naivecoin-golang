package txpool

import (
	"errors"
	"fmt"
	t "naivecoin/transactions"
)

// txPool stores a list of transactions received from another peers
var txPool []t.Transaction = []t.Transaction{}

// GetTransactionPool returns a deep copy of the transaction pool
func GetTransactionPool() []t.Transaction {
	cpy := make([]t.Transaction, len(txPool))
	copy(cpy, txPool)
	return cpy
}

// AddToTransactionPool validates and if valid adds a given transaction to a transaction pool
func AddToTransactionPool(tx t.Transaction, unspentTxOuts []t.UnspentTxOut) error {
	if !t.ValidateTransaction(tx, unspentTxOuts) {
		return errors.New("trying to add invalid tx to pool")
	}

	if !isValidTxForPool(tx, txPool) {
		return errors.New("trying to add invalid tx to pool")
	}

	//fmt.Printf("adding to txPool: %v", tx)
	txPool = append(txPool, tx)
	return nil
}

// hasTxIn checks if unspent transactions list contains a given txIn - transaction to be spent
func hasTxIn(txIn t.TxIn, unspentTxOuts []t.UnspentTxOut) bool {
	for n := 0; n < len(unspentTxOuts); n++ {
		if unspentTxOuts[n].TxOutId == txIn.TxOutId && unspentTxOuts[n].TxOutIndex == txIn.TxOutIndex {
			return true
		}
	}
	return false
}

// UpdateTransactionPool updates transaction pool with valid transactions
// transaction is valid if unspent transactions list contains it
func UpdateTransactionPool(unspentTxOuts_ []t.UnspentTxOut) {
	var newTxPool []t.Transaction = []t.Transaction{}
	for i := 0; i < len(txPool); i++ {
		isValid := true
		for j := 0; j < len(txPool[i].TxIns); j++ {
			if !hasTxIn(txPool[i].TxIns[j], unspentTxOuts_) {
				isValid = false
				break
			}
		}
		if isValid {
			newTxPool = append(newTxPool, txPool[i])
		}
	}
	txPool = newTxPool
}

// containsTxIn checks if a given lists of txIns has a specified txIn
func containsTxIn(txIns []t.TxIn, txIn t.TxIn) bool {
	for n := 0; n < len(txIns); n++ {
		if txIns[n].TxOutId == txIn.TxOutId && txIns[n].TxOutIndex == txIn.TxOutIndex {
			return true
		}
	}
	return false
}

// getTxPoolIns flattens the list of txIns included in transaction pool
func getTxPoolIns(txPool_ []t.Transaction) []t.TxIn {
	var txInsFlattened []t.TxIn = []t.TxIn{}
	for n := 0; n < len(txPool_); n++ {
		txInsFlattened = append(txInsFlattened, txPool_[n].TxIns...)
	}
	return txInsFlattened
}

// isValidTxForPool checks if transaction is already present in the pool
func isValidTxForPool(tx t.Transaction, txPool_ []t.Transaction) bool {
	var txPoolIns []t.TxIn = getTxPoolIns(txPool_)

	for n := 0; n < len(tx.TxIns); n++ {
		if containsTxIn(txPoolIns, tx.TxIns[n]) {
			fmt.Println("txIn already found in the txPool")
			return false
		}
	}
	return true
}

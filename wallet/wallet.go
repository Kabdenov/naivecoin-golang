package wallet

import (
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	t "naivecoin/transactions"
	"naivecoin/utils"
	"os"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// privateKeyPath stores a path for private key
// TODO: storing private key this way is unsecure
const privateKeyPath string = "./private.key"

// GetPublicFromWallet returns a public key for wallet, encoded as hex string
func GetBase58Address() string {
	publicKey := utils.GetPublicKey(GetPrivateFromWallet())
	return utils.Base58Encode(publicKey)
}

// GetPrivateFromWallet returns a private key for wallter, encoded as hex string
func GetPrivateFromWallet() string {
	content, err := ioutil.ReadFile(privateKeyPath)

	if err != nil {
		log.Fatal(err)
	}

	privateKey := string(content)

	return privateKey
}

// generatePrivateKey generates a new private key and returns it as hex encoded string
func generatePrivateKey() string {
	keyBytes, _, _, _ := elliptic.GenerateKey(secp256k1.S256(), rand.Reader)
	return hex.EncodeToString(keyBytes)
}

// InitWallet initializes wallet: generates a private key if does not exist
func InitWallet() {
	if _, err := os.Stat(privateKeyPath); err == nil {
		return
	} else if os.IsNotExist(err) {
		var newPrivateKey = generatePrivateKey()

		//https://zetcode.com/golang/writefile/
		f, err := os.Create(privateKeyPath)

		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		_, err2 := f.WriteString(newPrivateKey)

		if err2 != nil {
			log.Fatal(err2)
		}

		fmt.Printf("new wallet with private key created to : %s", privateKeyPath)
	}
}

// deleteWallet deletes a private key if exists
func deleteWallet() {
	if _, err := os.Stat(privateKeyPath); err == nil {
		e := os.Remove(privateKeyPath)
		if e != nil {
			log.Fatal(e)
		}
	}
}

// toUnsignedTxIn gets an unspent transaction and returns an unsigned txIn
func toUnsignedTxIn(unspentTxOut t.UnspentTxOut) t.TxIn {
	var txIn t.TxIn = t.TxIn{
		TxOutId:    unspentTxOut.TxOutId,
		TxOutIndex: unspentTxOut.TxOutIndex,
	}
	return txIn
}

// CreateTransaction creates a transaction for sending given amount for a given address
func CreateTransaction(base58Address string, amount float64, unspentTxOuts []t.UnspentTxOut, txPool []t.Transaction) t.Transaction {
	var myPrivateKey string = GetPrivateFromWallet()
	var myBase58Address string = GetBase58Address()
	var myUnspentTxOutsA = FindUnspentTxOuts(myBase58Address, unspentTxOuts)
	var myUnspentTxOuts = filterTxPoolTxs(myUnspentTxOutsA, txPool)

	// filter from unspentOutputs such inputs that are referenced in pool
	includedUnspentTxOuts, leftOverAmount, _ := FindTxOutsForAmount(amount, myUnspentTxOuts)

	var unsignedTxIns []t.TxIn = []t.TxIn{}
	for n := 0; n < len(includedUnspentTxOuts); n++ {
		unsignedTxIns = append(unsignedTxIns, toUnsignedTxIn(includedUnspentTxOuts[n]))
	}

	var tx t.Transaction = t.Transaction{
		TxIns:  unsignedTxIns,
		TxOuts: CreateTxOuts(base58Address, myBase58Address, amount, leftOverAmount),
	}

	tx.Id = t.GetTransactionId(tx)

	for index := 0; index < len(tx.TxIns); index++ {
		tx.TxIns[index].Signature, _ = t.SignTxIn(tx, index, myPrivateKey, unspentTxOuts)
	}

	return tx
}

// FindUnspentTxOuts returns a list of unused txOuts for a wallet
func FindUnspentTxOuts(base58Address string, unspentTxOuts []t.UnspentTxOut) []t.UnspentTxOut {
	var myUnspentTxOuts []t.UnspentTxOut = []t.UnspentTxOut{}
	for n := 0; n < len(unspentTxOuts); n++ {
		if unspentTxOuts[n].Address == base58Address {
			myUnspentTxOuts = append(myUnspentTxOuts, unspentTxOuts[n])
		}
	}
	return myUnspentTxOuts
}

// GetBalance returns balance of a wallet
func GetBalance(base58Address string, unspentTxOuts []t.UnspentTxOut) float64 {
	var balance float64
	for _, txOut := range FindUnspentTxOuts(base58Address, unspentTxOuts) {
		balance += txOut.Amount
	}
	return balance
}

// FindTxOutsForAmount builds the list of unspent txOuts that belong to a wallet owner and sum up to a given amount
func FindTxOutsForAmount(amount float64, unspentTxOuts []t.UnspentTxOut) ([]t.UnspentTxOut, float64, error) {
	var currentAmount float64
	var includedUnspentTxOuts []t.UnspentTxOut = []t.UnspentTxOut{}
	for n := 0; n < len(unspentTxOuts); n++ {
		includedUnspentTxOuts = append(includedUnspentTxOuts, unspentTxOuts[n])
		currentAmount += unspentTxOuts[n].Amount
		if currentAmount >= amount {
			var leftOverAmount = currentAmount - amount
			return includedUnspentTxOuts, leftOverAmount, nil
		}
	}
	return []t.UnspentTxOut{}, amount, errors.New("cannot create transaction from the available unspent transaction outputs")
}

// CreateTxOuts creates txOuts for a wallet
// also includes loopback transaction for a change if leftover amount is > 0
func CreateTxOuts(targetBase58Address string, sourceBase58Address string, amount float64, leftOverAmount float64) []t.TxOut {
	var txOut t.TxOut = t.TxOut{
		Address: targetBase58Address,
		Amount:  amount,
	}
	if leftOverAmount == 0 {
		return []t.TxOut{txOut}
	} else {
		var leftOverTx t.TxOut = t.TxOut{
			Address: sourceBase58Address,
			Amount:  leftOverAmount,
		}
		return []t.TxOut{txOut, leftOverTx}
	}
}

// findTxIn finds a txIn in a given txIns list
func findTxIn(txIns []t.TxIn, txOutId string, txOutIndex int) (t.TxIn, bool) {
	for n := 0; n < len(txIns); n++ {
		if txIns[n].TxOutId == txOutId && txIns[n].TxOutIndex == txOutIndex {
			return txIns[n], true
		}
	}
	return t.TxIn{}, false
}

// filterTxPoolTxs returns the list of unsepnt txOuts that are not present in transaction pool
func filterTxPoolTxs(unspentTxOuts []t.UnspentTxOut, txPool []t.Transaction) []t.UnspentTxOut {
	var txInsFlattened []t.TxIn = []t.TxIn{}
	for n := 0; n < len(txPool); n++ {
		txInsFlattened = append(txInsFlattened, txPool[n].TxIns...)
	}

	var filtered []t.UnspentTxOut = []t.UnspentTxOut{}
	for n := 0; n < len(unspentTxOuts); n++ {
		_, found := findTxIn(txInsFlattened, unspentTxOuts[n].TxOutId, unspentTxOuts[n].TxOutIndex)
		if !found {
			filtered = append(filtered, unspentTxOuts[n])
		}
	}

	return filtered
}

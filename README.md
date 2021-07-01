# Basic cryptocurrency implementation developed in Go.
I developed this project while self-studying the Go programming language.  

I was inspired by [this](https://lhartikk.github.io/jekyll/update/2017/07/15/chapter0.html) article that explains basic concepts of cryptocurrency:  
- blockchain, mining, and block validation  
- transactions, transaction pool, and transaction validation  
- p2p networks, basic cryptography  

The motivation for choosing this particular project was that it covers many programming aspects that are better learned through practice when studying a new language.  

There are a few possible future improvements:  
- Automatic peer discovery;  
- Another data structure for unspent transactions (now they are stored in an array, which doesn't scale well and requires O(n) search time);  
- Better scheme for blockchain synchronization (now the whole blockchain is sent for out of sync peers, instead of sending just missing parts);  
- More secure approach to store and manage a private key for a wallet (now the private key is stored in plaintext in the startup folder)  

The user interface to send coins, add peers, mine new blocks and see their structure can be found [here](https://github.com/Kabdenov/naivecoin-app)
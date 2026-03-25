``` bitcoin.conf
regtest=1
server=1
rpcuser=vuong
rpcpassword=hope
[regtest]
rpcport=18443
fallbackfee=0.0001
```
``` go
go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network regtest -wallet test -fund-sat 10000 -fee-sat 1000 -auto-mine-regtest
[1/9] Checking node connectivity...
[1.5/9] Ensuring wallet is loaded...
[2/9] Generating secrets and linked ACS script...
  linked ACS address: bcrt1qflnxz8y6vdmgnddhrsmxh7upv5yfhc5mjgar8hdffn27c9m4h4cqe4uk30
  H(r^j_a): 812869a540ecb0728f796039ce9c3a73ee2cff40d957f3217a7b2602156a5e76
  H(pre_b): 901695285e10acd215091e6843c315abe5f6cb925311662a5b4b55295e1daa97
[3/9] Funding linked ACS output...
  fund txid: b1bf01a8e60dd9864652460af0b17ec10d8a5c6246b052d77d108c12d9e9c40c
[4/9] Mining 1 regtest block to confirm funding tx...
  located funding outpoint: b1bf01a8e60dd9864652460af0b17ec10d8a5c6246b052d77d108c12d9e9c40c:0 (10000 sat)
[5/9] Building spend transaction with corrected witness order...
[6/9] Verifying mempool acceptance...
[7/9] Broadcasting spend transaction...
  spend txid: 9c793e227b99a167c30b845f3b4a09de761899c03ad79df0b2e96e4258f0983d
[8/9] Mining 1 regtest block to confirm spend tx...
[9/9] Writing artifact...
=== Linked ACS deployment completed ===
Fund TxID : b1bf01a8e60dd9864652460af0b17ec10d8a5c6246b052d77d108c12d9e9c40c
Spend TxID: 9c793e227b99a167c30b845f3b4a09de761899c03ad79df0b2e96e4258f0983d
Artifact  : artifacts\linked_acs_regtest.json
```


```bitcoin.conf
# Global settings
server=1

# Signet specific settings
[signet]
rpcuser=vuong
rpcpassword=hope
rpcport=38332
rpcallowip=127.0.0.1
prune=1000
```
``` go
go run ./scripts/deploy_linked_acs.go -bitcoin-cli "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe" -network signet -wallet test -fund-sat 10000 -fee-sat 2000 -max-wait-seconds 600
[1/9] Checking node connectivity...
[1.5/9] Ensuring wallet is loaded...
[1.6/9] Checking wallet balance...
[2/9] Generating secrets and linked ACS script...
  linked ACS address: tb1qkf9c6ksssndxte45qed39w7el674z0m036fs4s7g4l2yztz7qgtsgn7dnj
  H(r^j_a): a128a3420440f1e98511a3c0c72b9bb02e8c3cf43645cc680cbd554212522b55
  H(pre_b): f7bd767bb6d43c2effe4eaff1a4a313019771be0fce88fd07480a54313db0a7f
[3/9] Funding linked ACS output...
  fund txid: 37bcd54fe81bfcb39a6212a6e9372cf03b024288d207f4e4fbd8df62102625d4
[4/9] Skipping mining step (signet/manual mode)...
  located funding outpoint: 37bcd54fe81bfcb39a6212a6e9372cf03b024288d207f4e4fbd8df62102625d4:0 (10000 sat)
[5/9] Building spend transaction with corrected witness order...
[6/9] Verifying mempool acceptance...
[7/9] Broadcasting spend transaction...
  spend txid: bb425bdfac48e420e1e0711ed5def96a7fc98850609561a84518542a239b4b27
[8/9] No auto-mining for this network/mode.
[9/9] Writing artifact...

=== Linked ACS deployment completed ===
Fund TxID : 37bcd54fe81bfcb39a6212a6e9372cf03b024288d207f4e4fbd8df62102625d4
Spend TxID: bb425bdfac48e420e1e0711ed5def96a7fc98850609561a84518542a239b4b27
Artifact  : artifacts\linked_acs_signet.json
```

Download [Bitcoin](https://bitcoincore.org/en/download/)

[Get Signet Coin ](https://signet257.bublina.eu.org/)
or [Signet](https://signetfaucet.com/)


### 1. Wallet Management
These commands handle wallet lifecycle and solve the "Wallet not loaded" (-18) error.

* **List currently loaded wallets:**
    `bitcoin-cli -regtest listwallets`
* **List all available wallets in the directory:**
    `bitcoin-cli -regtest listwalletdir`
* **Create a new wallet:**
    `bitcoin-cli -regtest createwallet "wallet_name"`
* **Load an existing wallet:**
    `bitcoin-cli -regtest loadwallet "wallet_name"`
* **Unload a wallet:**
    `bitcoin-cli -regtest unloadwallet "wallet_name"`

---

### 2. Address & Balance
**Note:** You must use the `-rpcwallet` flag if multiple wallets are loaded.

* **Generate a new receiving address:**
    `bitcoin-cli -regtest -rpcwallet=test getnewaddress`
* **Check confirmed balance:**
    `bitcoin-cli -regtest -rpcwallet=test getbalance`
* **Check pending (unconfirmed) balance:**
    `bitcoin-cli -regtest -rpcwallet=test getunconfirmedbalance`
* **List Unspent Transaction Outputs (UTXOs):**
    `bitcoin-cli -regtest -rpcwallet=test listunspent`

---

### 3. Transactions & Mining
These are your primary tools for the CRAB-He research environment.

* **Mine blocks to get coins or confirm transactions (Regtest only):**
    `bitcoin-cli -regtest generatetoaddress 101 "your_address"`
* **Send BTC to an address:**
    `bitcoin-cli -regtest -rpcwallet=test -named sendtoaddress address="address" amount=0.01`
* **Get detailed info about a wallet transaction:**
    `bitcoin-cli -regtest -rpcwallet=test gettransaction "TXID"`
* **Decode a raw transaction (View scripts/witness):**
    `bitcoin-cli -regtest getrawtransaction "TXID" true`
* **Broadcast a signed hex string (from your Go code):**
    `bitcoin-cli -regtest sendrawtransaction "hex_string"`

---

### 4. Blockchain & Network Status


* **Check node status (Height, Network, Sync progress):**
    `bitcoin-cli -regtest getblockchaininfo`
* **Check network/connection info:**
    `bitcoin-cli -regtest getnetworkinfo`
* **Check specific wallet features (SegWit, HD):**
    `bitcoin-cli -regtest -rpcwallet=test getwalletinfo`

---

### 5. Advanced Troubleshooting (For HE-HTLC Research)
* **Test if a transaction would be accepted by the mempool:**
    `bitcoin-cli -regtest testmempoolaccept '["hex_string"]'`
* **View a transaction's script in the mempool:**
    `bitcoin-cli -regtest getmempoolentry "TXID"`



**Quick Tip:** If you want to use these for **Signet**, just replace `-regtest` with `-signet` in any command above.


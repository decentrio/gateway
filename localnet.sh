#!/bin/bash

# dymension
set -e

cd /root
binary=$1
CHAIN_ID="local_1223-1"

update_genesis () {
    cat localnet/v1/config/genesis.json | jq "$1" > localnet/v1/config/tmp_genesis.json && mv localnet/v1/config/tmp_genesis.json localnet/v1/config/genesis.json
}

systemctl stop v1 v2 v3 v4

rm -rf localnet
mkdir -p localnet/v1
mkdir -p localnet/v2
mkdir -p localnet/v3
mkdir -p localnet/v4

# init all three vs
$binary init --chain-id $CHAIN_ID v1 --home localnet/v1 2> output1
$binary init --chain-id $CHAIN_ID v2 --home localnet/v2 2> output2
$binary init --chain-id $CHAIN_ID v3 --home localnet/v3 2> output3
$binary init --chain-id $CHAIN_ID v4 --home localnet/v4 2> output4

# create keys for all three vs
mnemonic1="ozone unfold device pave lemon potato omit insect column wise cover hint narrow large provide kidney episode clay notable milk mention dizzy muffin crazy"
mnemonic2="soap step crash ceiling path virtual this armor accident pond share track spice woman vault discover share holiday inquiry oak shine scrub bulb arrive"
mnemonic3="travel jelly basic visa apart kidney piano lumber elevator fat unknown guard matter used high drastic umbrella humble crush stock banner enlist mule unique"
mnemonic4="mixture replace miss absent base climb coach eternal intact brother scissors trip post plastic hotel ginger shuffle leopard nephew issue amount acoustic jealous magic"
mnemonic5="gorilla puzzle attack deliver valid usual truck either meadow corn air interest army ridge immune illegal manage raven equal truly group screen fire dune"

node1=$(cat output1 | jq -r '.node_id')
node2=$(cat output2 | jq -r '.node_id')
node3=$(cat output3 | jq -r '.node_id')
node4=$(cat output4 | jq -r '.node_id')
rm output*

echo $mnemonic1 | $binary keys add v1 --recover --keyring-backend test --home localnet/v1
echo $mnemonic2 | $binary keys add v2 --recover --keyring-backend test --home localnet/v2
echo $mnemonic3 | $binary keys add v3 --recover --keyring-backend test --home localnet/v3
echo $mnemonic4 | $binary keys add v4 --recover --keyring-backend test --home localnet/v4
echo $mnemonic5 | $binary keys add god --recover --keyring-backend test --home localnet/v1

# create v node with tokens to transfer to the three other nodes
$binary add-genesis-account $($binary keys show v1 -a --keyring-backend test --home localnet/v1) 10000000000000000000stake --home localnet/v1 
$binary add-genesis-account $($binary keys show v2 -a --keyring-backend test --home localnet/v2) 10000000000000000000stake --home localnet/v1 
$binary add-genesis-account $($binary keys show v3 -a --keyring-backend test --home localnet/v3) 10000000000000000000stake --home localnet/v1 
$binary add-genesis-account $($binary keys show v4 -a --keyring-backend test --home localnet/v4) 10000000000000000000stake --home localnet/v1
$binary add-genesis-account $($binary keys show god -a --keyring-backend test --home localnet/v1) 1000000000000000000000stake --home localnet/v1
cp localnet/v1/config/genesis.json localnet/v2/config/genesis.json
cp localnet/v1/config/genesis.json localnet/v3/config/genesis.json
cp localnet/v1/config/genesis.json localnet/v4/config/genesis.json
$binary gentx v1 4000000000000000000stake --keyring-backend test --home localnet/v1 --chain-id $CHAIN_ID
$binary gentx v2 3000000000000000000stake --keyring-backend test --home localnet/v2 --chain-id $CHAIN_ID
$binary gentx v3 2000000000000000000stake --keyring-backend test --home localnet/v3 --chain-id $CHAIN_ID
$binary gentx v4 1000000000000000000stake --keyring-backend test --home localnet/v4 --chain-id $CHAIN_ID

cp localnet/v2/config/gentx/*.json localnet/v1/config/gentx/
cp localnet/v3/config/gentx/*.json localnet/v1/config/gentx/
cp localnet/v4/config/gentx/*.json localnet/v1/config/gentx/
$binary collect-gentxs --home localnet/v1 

# update genesis params
# update_genesis '.app_state["gov"]["params"]["voting_period"] = "15s"'
# update_genesis '.app_state["staking"]["params"]["max_validators"] = 3'
# update_genesis '.app_state["staking"]["params"]["unbonding_time"] = "3s"'
# update_genesis '.app_state["slashing"]["params"]["signed_blocks_window"] = "10"'
# update_genesis '.app_state["gov"]["params"]["min_deposit"][0]["amount"] = "10"'

# $binary genesis validate --home localnet/v1 

cp localnet/v1/config/genesis.json localnet/v2/config/genesis.json
cp localnet/v1/config/genesis.json localnet/v3/config/genesis.json
cp localnet/v1/config/genesis.json localnet/v4/config/genesis.json

# change app.toml values
v1_APP_TOML=localnet/v1/config/app.toml
v2_APP_TOML=localnet/v2/config/app.toml
v3_APP_TOML=localnet/v3/config/app.toml
v4_APP_TOML=localnet/v4/config/app.toml

# enable api
sed -i.bak -e '/^\[api\]$/,/^\[/ s/enable = false/enable = true/' $v1_APP_TOML
sed -i.bak -e '/^\[api\]$/,/^\[/ s/enable = false/enable = true/' $v2_APP_TOML
sed -i.bak -e '/^\[api\]$/,/^\[/ s/enable = false/enable = true/' $v3_APP_TOML
sed -i.bak -e '/^\[api\]$/,/^\[/ s/enable = false/enable = true/' $v4_APP_TOML

# enable swagger
sed -i.bak -e '/^\[api\]$/,/^\[/ s/swagger = false/swagger = true/' $v1_APP_TOML
sed -i.bak -e '/^\[api\]$/,/^\[/ s/swagger = false/swagger = true/' $v2_APP_TOML
sed -i.bak -e '/^\[api\]$/,/^\[/ s/swagger = false/swagger = true/' $v3_APP_TOML
sed -i.bak -e '/^\[api\]$/,/^\[/ s/swagger = false/swagger = true/' $v4_APP_TOML

# disable grpc
# sed -i.bak -e '/^\[grpc\]$/,/^\[/ s/enable = true/enable = false/' $v1_APP_TOML
# sed -i.bak -e '/^\[grpc\]$/,/^\[/ s/enable = true/enable = false/' $v2_APP_TOML
# sed -i.bak -e '/^\[grpc\]$/,/^\[/ s/enable = true/enable = false/' $v3_APP_TOML
# sed -i.bak -e '/^\[grpc\]$/,/^\[/ s/enable = true/enable = false/' $v4_APP_TOML

# set minium gas prices
sed -i -E 's|minimum-gas-prices = ""|minimum-gas-prices = "0.0001stake"|g' $v1_APP_TOML
sed -i -E 's|minimum-gas-prices = ""|minimum-gas-prices = "0.0001stake"|g' $v2_APP_TOML
sed -i -E 's|minimum-gas-prices = ""|minimum-gas-prices = "0.0001stake"|g' $v3_APP_TOML
sed -i -E 's|minimum-gas-prices = ""|minimum-gas-prices = "0.0001stake"|g' $v4_APP_TOML

# open api ports
sed -i -E 's|tcp://0.0.0.0:1317|tcp://0.0.0.0:2014|g' $v1_APP_TOML
sed -i -E 's|tcp://0.0.0.0:1317|tcp://0.0.0.0:2024|g' $v2_APP_TOML
sed -i -E 's|tcp://0.0.0.0:1317|tcp://0.0.0.0:2034|g' $v3_APP_TOML
sed -i -E 's|tcp://0.0.0.0:1317|tcp://0.0.0.0:2044|g' $v4_APP_TOML

# open grpc ports 
sed -i -E 's|0.0.0.0:9090|0.0.0.0:2012|g' $v1_APP_TOML
sed -i -E 's|0.0.0.0:9090|0.0.0.0:2022|g' $v2_APP_TOML
sed -i -E 's|0.0.0.0:9090|0.0.0.0:2032|g' $v3_APP_TOML
sed -i -E 's|0.0.0.0:9090|0.0.0.0:2042|g' $v4_APP_TOML

# open grpc-web ports 
sed -i -E 's|0.0.0.0:9091|0.0.0.0:2013|g' $v1_APP_TOML
sed -i -E 's|0.0.0.0:9091|0.0.0.0:2023|g' $v2_APP_TOML
sed -i -E 's|0.0.0.0:9091|0.0.0.0:2033|g' $v3_APP_TOML
sed -i -E 's|0.0.0.0:9091|0.0.0.0:2043|g' $v4_APP_TOML

# open json rpc ports 
sed -i -E 's|127.0.0.1:8545|0.0.0.0:2015|g' $v1_APP_TOML
sed -i -E 's|127.0.0.1:8545|0.0.0.0:2025|g' $v2_APP_TOML
sed -i -E 's|127.0.0.1:8545|0.0.0.0:2035|g' $v3_APP_TOML
sed -i -E 's|127.0.0.1:8545|0.0.0.0:2045|g' $v4_APP_TOML

# open json rpc ws ports 
sed -i -E 's|127.0.0.1:8546|0.0.0.0:2016|g' $v1_APP_TOML
sed -i -E 's|127.0.0.1:8546|0.0.0.0:2026|g' $v2_APP_TOML
sed -i -E 's|127.0.0.1:8546|0.0.0.0:2036|g' $v3_APP_TOML
sed -i -E 's|127.0.0.1:8546|0.0.0.0:2046|g' $v4_APP_TOML

# change config.toml values
v1_CONFIG=localnet/v1/config/config.toml
v2_CONFIG=localnet/v2/config/config.toml
v3_CONFIG=localnet/v3/config/config.toml
v4_CONFIG=localnet/v4/config/config.toml

# open and change rpc ports
sed -i -E 's|tcp://127.0.0.1:26657|tcp://0.0.0.0:2011|g' $v1_CONFIG
sed -i -E 's|tcp://127.0.0.1:26657|tcp://0.0.0.0:2021|g' $v2_CONFIG
sed -i -E 's|tcp://127.0.0.1:26657|tcp://0.0.0.0:2031|g' $v3_CONFIG
sed -i -E 's|tcp://127.0.0.1:26657|tcp://0.0.0.0:2041|g' $v4_CONFIG

# change p2p ports
sed -i -E 's|tcp://0.0.0.0:26656|tcp://0.0.0.0:2010|g' $v1_CONFIG
sed -i -E 's|tcp://0.0.0.0:26656|tcp://0.0.0.0:2020|g' $v2_CONFIG
sed -i -E 's|tcp://0.0.0.0:26656|tcp://0.0.0.0:2030|g' $v3_CONFIG
sed -i -E 's|tcp://0.0.0.0:26656|tcp://0.0.0.0:2040|g' $v4_CONFIG

# allow duplicated ip
sed -i -E 's|allow_duplicate_ip = false|allow_duplicate_ip = true|g' $v1_CONFIG
sed -i -E 's|allow_duplicate_ip = false|allow_duplicate_ip = true|g' $v2_CONFIG
sed -i -E 's|allow_duplicate_ip = false|allow_duplicate_ip = true|g' $v3_CONFIG
sed -i -E 's|allow_duplicate_ip = false|allow_duplicate_ip = true|g' $v4_CONFIG

PEERS="$node1@0.0.0.0:2010,$node2@0.0.0.0:2020,$node3@0.0.0.0:2030,$node4@0.0.0.0:2040"
echo $PEERS
sed -i.bak -e "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" localnet/v1/config/config.toml
sed -i.bak -e "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" localnet/v2/config/config.toml
sed -i.bak -e "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" localnet/v3/config/config.toml
sed -i.bak -e "s/^persistent_peers *=.*/persistent_peers = \"$PEERS\"/" localnet/v4/config/config.toml

# reset state
$binary tendermint unsafe-reset-all --home ./localnet/v1
$binary tendermint unsafe-reset-all --home ./localnet/v2
$binary tendermint unsafe-reset-all --home ./localnet/v3
$binary tendermint unsafe-reset-all --home ./localnet/v4

sudo tee /etc/systemd/system/v1.service > /dev/null <<EOF
[Unit]
Description=v1
After=network-online.target

[Service]
User=root
WorkingDirectory=/root/localnet
ExecStart=$(which $binary) start --home v1
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo tee /etc/systemd/system/v2.service > /dev/null <<EOF
[Unit]
Description=v2
After=network-online.target

[Service]
User=root
WorkingDirectory=/root/localnet
ExecStart=$(which $binary) start --home v2
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo tee /etc/systemd/system/v3.service > /dev/null <<EOF
[Unit]
Description=v3
After=network-online.target

[Service]
User=root
WorkingDirectory=/root/localnet
ExecStart=$(which $binary) start --home v3
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo tee /etc/systemd/system/v4.service > /dev/null <<EOF
[Unit]
Description=v4
After=network-online.target

[Service]
User=root
WorkingDirectory=/root/localnet
ExecStart=$(which $binary) start --home v4
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl restart v1 v2 v3 v4
journalctl -fu v2 -n100

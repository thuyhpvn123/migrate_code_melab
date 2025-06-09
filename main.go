package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/cmd/client"
	c_config "github.com/meta-node-blockchain/meta-node/cmd/client/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/transaction"
	"github.com/meta-node-blockchain/migrate_code_melab/config"
)

// Contract binding và MiningCode struct (ví dụ)
type MiningCode struct {
    PublicKey    []byte
    BoostRate    *big.Int
    MaxDuration  *big.Int
    Status uint8
    AssignedTo common.Address
    Referrer common.Address
    ReferralReward *big.Int
    Transferable bool
    LockUntil *big.Int
    LockType uint8
    
}

func main() {
    // 1. Load Excel file
    file, err := excelize.OpenFile("listCode_melab.xlsx")
    if err != nil {
        log.Fatalf("Failed to open Excel file: %v", err)
    }
    config, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal("invalid configuration", err)
	}
    // 2. Connect to blockchain
    client, err := client.NewClient(
		&c_config.ClientConfig{
			Version_:                config.MetaNodeVersion,
			PrivateKey_:             config.PrivateKeyAdmin,
			ParentAddress:           config.AdminAddress,
			ParentConnectionAddress: config.ParentConnectionAddress,
			DnsLink_:                config.DnsLink(),
			ConnectionAddress_:      config.ConnectionAddress_,
			ParentConnectionType:    config.ParentConnectionType,
			ChainId:                 config.ChainId,
		},
	)
	if err != nil {
		logger.Error(fmt.Sprintf("error when create chain client %v", err))
	}
    // 3. Load Contract

	reader, err := os.Open("code.abi")
	if err != nil {
		logger.Error("Error occured while read create Code smart contract abi")
	}
	defer reader.Close()

	abi, err := abi.JSON(reader)
	if err != nil {
		logger.Error("Error occured while parse create Code smart contract abi")
	}

    // 4. Prepare MiningCode slice
    var miningCodes []MiningCode

    rows := file.GetRows("ListCode_MeLab")
    if err != nil {
        log.Fatalf("Failed to read rows: %v", err)
    }

    // Skip header
    for idx, row := range rows {
        if idx == 0 {
            continue
        }

        if len(row) < 4 {
            log.Printf("Skipping incomplete row %d", idx+1)
            continue
        }

        userAddr := common.HexToAddress(row[0])
        codeHashHex := row[1]
        maxDuration, ok := new(big.Int).SetString(row[2], 10)
        if !ok {
            log.Printf("Invalid ExpirationActiveTime at row %d", idx+1)
            continue
        }
        boostRate, ok := new(big.Int).SetString(row[3], 10)
        if !ok {
            log.Printf("Invalid RateBoost at row %d", idx+1)
            continue
        }

        // Convert codeHash from hex string to [32]byte
        codeHashBytes := common.Hex2Bytes(strings.TrimPrefix(codeHashHex, "0x"))
        miningCodes = append(miningCodes, MiningCode{
            PublicKey:   codeHashBytes,
            BoostRate:   boostRate,
            MaxDuration: maxDuration,
            Status : 2,
            AssignedTo:  userAddr, 
            ReferralReward: big.NewInt(0),
            Transferable: false,
            LockUntil: big.NewInt(0),
            LockType: 0,

        })
    }

    // 6. Call migrateCode
    input, err := abi.Pack(
		"migrateCode",
		miningCodes,
	)
	if err != nil {
		logger.Error("error when pack call data", err)
		panic(err)
	}
    callData := transaction.NewCallData(input)

	bData, err := callData.Marshal()
	if err != nil {
		logger.Error(fmt.Sprintf("Marshal calldata for %s failed", "migrateCode"), err)
	}

	relatedAddress := []common.Address{}
	maxGas := uint64(5_000_000)
	maxGasPrice := uint64(1_000_000_000)
	timeUse := uint64(0)

	receipt, err := client.SendTransactionWithDeviceKey(
		common.HexToAddress(config.AdminAddress),
		common.HexToAddress(config.CodeAddress),
		big.NewInt(0),
		bData,
		relatedAddress,
		maxGas,
		maxGasPrice,
		timeUse,
	)
	if receipt.Status() == pb.RECEIPT_STATUS_RETURNED {
		result := make(map[string]interface{})
		err := abi.UnpackIntoMap(result, "migrateCode", receipt.Return())
		if err != nil {
			logger.Error(fmt.Sprintf("UnpackIntoMap error for %s", "migrateCode"), err)
		}
	}
	fmt.Println( hex.EncodeToString(receipt.Return()))

}


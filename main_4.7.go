package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/360EntSecGroup-Skylar/excelize"
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
    MaxDuration  *big.Int
    AssignedTo common.Address
	ActiveTime *big.Int
	Amount *big.Int
}

func main() {
    // 1. Load Excel file
    file, err := excelize.OpenFile("migrate_data.xlsx")
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
			// DnsLink_:                config.DnsLink(),
			ConnectionAddress_:      config.ConnectionAddress_,
			ParentConnectionType:    config.ParentConnectionType,
			ChainId:                 config.ChainId,
		},
	)
	if err != nil {
		logger.Error(fmt.Sprintf("error when create chain client %v", err))
	}
    // 3. Load Contract

	reader, err := os.Open("migrateSC.abi")
	if err != nil {
		logger.Error("Error occured while read create migrateSC smart contract abi")
	}
	defer reader.Close()

	abiCode, err := abi.JSON(reader)
	if err != nil {
		logger.Error("Error occured while parse create migrateSC smart contract abi")
	}


    // 4. Prepare MiningCode slice
    var miningCodes []MiningCode

    rows := file.GetRows("migrate_data")
    if err != nil {
        log.Fatalf("Failed to read rows: %v", err)
    }

    // Skip header
    for idx, row := range rows {
        if idx == 0 {
            continue
        }

        if len(row) < 4 {
           log.Printf("Skipping incomplete row %d: only %d columns", idx+1, len(row))
            continue
        }

        userAddr := common.HexToAddress(row[0])
		fmt.Println("userAddr:",userAddr)
		amountHex := strings.TrimSpace(row[1])
		if amountHex == "" {
			log.Printf("Skipping row %d: empty amount", idx+1)
			continue
		}
		fmt.Println("amountHex:",amountHex)
        amount, ok := new(big.Int).SetString(amountHex, 10)
        if !ok {
			fmt.Printf("row %d raw amount: '%s'\n", idx+1, row[1])
            continue
        }
		activeTimeHex := strings.TrimSpace(row[2])
		if activeTimeHex == "" {
			log.Printf("Skipping row %d: empty start time", idx+1)
			continue
		}
        activeTime, ok := new(big.Int).SetString(activeTimeHex, 10)
        if !ok {
			fmt.Printf("row %d raw start time: '%s'\n", idx+1, row[2])
            continue
        }


		maxDurationHex := strings.TrimSpace(row[3])
		if maxDurationHex == "" {
			log.Printf("Skipping row %d: empty End Time", idx+1)
			continue
		}
        maxDuration, ok := new(big.Int).SetString(maxDurationHex, 10)
        if !ok {
			fmt.Printf("row %d raw End Time: '%s'\n", idx+1, row[2])
            continue
        }
	
        // Convert codeHash from hex string to [32]byte
        miningCodes = append(miningCodes, MiningCode{
            MaxDuration: maxDuration,
            AssignedTo:  userAddr, 
            ActiveTime: activeTime,
            Amount: amount,
        })
    }

    // 6. Call migrateCode
    input, err := abiCode.Pack(
		"BEmigrateData",
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
		common.HexToAddress(config.MigrateSCAddress),
		big.NewInt(0),
		bData,
		relatedAddress,
		maxGas,
		maxGasPrice,
		timeUse,
	)
	if receipt.Status() == pb.RECEIPT_STATUS_RETURNED {
		result := make(map[string]interface{})
		err := abiCode.UnpackIntoMap(result, "BEmigrateData", receipt.Return())
		if err != nil {
			logger.Error(fmt.Sprintf("UnpackIntoMap error for %s", "BEmigrateData"), err)
		}
	}
	fmt.Println( hex.EncodeToString(receipt.Return()))
	logger.Error("Done")
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(done)
	}()
	<-done
}


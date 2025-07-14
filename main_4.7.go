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
	"github.com/xuri/excelize/v2"
	// "github.com/360EntSecGroup-Skylar/excelize"
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
	MaxDuration *big.Int
	AssignedTo  common.Address
	ActiveTime  *big.Int
	Amount      *big.Int
	IsMigrated  bool
	BoostRate   *big.Int
}

func main() {
	// 1. Load Excel file
	// file, err := excelize.OpenFile("test_migrate1.xlsx")
	file, err := excelize.OpenFile("migrate_data_14.7.xlsx")
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
			ConnectionAddress_:   config.ConnectionAddress_,
			ParentConnectionType: config.ParentConnectionType,
			ChainId:              config.ChainId,
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

	rows,err := file.GetRows("migrate_data")
	if err != nil {
		log.Fatalf("Failed to read rows: %v", err)
	}
	fmt.Println("rows:", rows)
	fmt.Println("All sheet names:", file.GetSheetMap())
	// Skip header
	for idx, row := range rows {
		fmt.Println("2")
		if idx == 0 {
			fmt.Println("no row found")
			continue

		}

		if len(row) < 4 {
			log.Printf("Skipping incomplete row %d: only %d columns", idx+1, len(row))
			continue
		}

		userAddr := common.HexToAddress(row[0])
		fmt.Println("userAddr:", userAddr)
		amountHex := strings.TrimSpace(row[1])
		if amountHex == "" {
			log.Printf("Skipping row %d: empty amount", idx+1)
			continue
		}
		fmt.Println("amountHex:", amountHex)
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
			fmt.Printf("row %d raw End Time: '%s'\n", idx+1, row[3])
			continue
		}
		boostRateHex := strings.TrimSpace(row[4])
		if boostRateHex == "" {
			log.Printf("Skipping row %d: empty Boost Rate", idx+1)
			continue
		}
		boostRate, ok := new(big.Int).SetString(boostRateHex, 10)
		if !ok {
			fmt.Printf("row %d raw Boost Rate: '%s'\n", idx+1, row[4])
			continue
		}
		// Convert codeHash from hex string to [32]byte
		miningCodes = append(miningCodes, MiningCode{
			MaxDuration: maxDuration,
			AssignedTo:  userAddr,
			ActiveTime:  activeTime,
			Amount:      amount,
			IsMigrated:  false,
			BoostRate: boostRate,
		})
	}

	// 6. Call migrateCode
	relatedAddress := []common.Address{}
	maxGas := uint64(5_000_000)
	maxGasPrice := uint64(1_000_000_000)
	timeUse := uint64(0)
	if len(miningCodes) > 0 {
		fmt.Println("call to chain")
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
		fmt.Println(hex.EncodeToString(receipt.Return()))
		logger.Error("Done1")

	} else {
		fmt.Println("get no data")
	}
	//get data
	fmt.Println("call to chain to get data")
	f := excelize.NewFile()
	sheetName := "Sheet1"

	// Ghi tiêu đề cột
	headers := []string{"MaxDuration", "AssignedTo", "ActiveTime", "Amount", "IsMigrated","BoostRate"}

	input, err := abiCode.Pack(
		"getAllMigrateDataUsers",
	)
	if err != nil {
		logger.Error("error when pack call data", err)
		panic(err)
	}
	callData := transaction.NewCallData(input)

	bData, err := callData.Marshal()
	if err != nil {
		logger.Error(fmt.Sprintf("Marshal calldata for %s failed", "get data"), err)
	}
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
		// result := make(map[string]interface{})
		var kq []MiningCode
		err := abiCode.UnpackIntoInterface(&kq, "getAllMigrateDataUsers", receipt.Return())
		if err != nil {
			logger.Error(fmt.Sprintf("UnpackIntoMap error for %s", "getAllMigrateDataUsers"), err)
		}
		for col, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(col+1, 1)
			f.SetCellValue(sheetName, cell, h)
		}
		// kq := result[""].([]MiningCode)

		// Ghi từng dòng dữ liệu
		for i, item := range kq {
			row := i + 2 // Bắt đầu từ hàng 2
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), item.MaxDuration.String())
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), item.AssignedTo.Hex())
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), item.ActiveTime.String())
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), item.Amount.String())
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), item.IsMigrated)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), item.BoostRate)
		}

		// Lưu file
		if err := f.SaveAs("output_mining_codes.xlsx"); err != nil {
			panic(err)
		}
	}
	logger.Error("Done2")

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(done)
	}()
	<-done
}

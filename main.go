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
    ExpireTime *big.Int
}
type MiningAmount struct {
	User common.Address
	PrivateCode [32]byte
	ActiveTime *big.Int
	Amount *big.Int
}
func main() {
    // 1. Load Excel file
    file, err := excelize.OpenFile("listCode_melab_21.6.xlsx")
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

	reader, err := os.Open("code.abi")
	if err != nil {
		logger.Error("Error occured while read create Code smart contract abi")
	}
	defer reader.Close()

	abiCode, err := abi.JSON(reader)
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

        if len(row) < 6 {
           log.Printf("Skipping incomplete row %d: only %d columns", idx+1, len(row))
            continue
        }

        userAddr := common.HexToAddress(row[0])
        codeHashHex := row[1]
		fmt.Println("row[2]",row[2])
		maxDurationHex := strings.TrimSpace(row[2])
		if maxDurationHex == "" {
			log.Printf("Skipping row %d: empty ExpirationActiveTime", idx+1)
			continue
		}
        maxDuration, ok := new(big.Int).SetString(maxDurationHex, 10)
        if !ok {
			fmt.Printf("row %d raw ExpirationActiveTime: '%s'\n", idx+1, row[2])
            continue
        }
        boostRate, ok := new(big.Int).SetString(row[3], 10)
        if !ok {
            log.Printf("Invalid RateBoost at row %d", idx+1)
            continue
        }
        expireTime, ok := new(big.Int).SetString(row[4], 10)
        if !ok {
            log.Printf("Invalid ExpireTime at row %d", idx+1)
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
			ExpireTime: expireTime,
        })

		

    }

    // 6. Call migrateCode
    input, err := abiCode.Pack(
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
		err := abiCode.UnpackIntoMap(result, "migrateCode", receipt.Return())
		if err != nil {
			logger.Error(fmt.Sprintf("UnpackIntoMap error for %s", "migrateCode"), err)
		}
	}
	fmt.Println( hex.EncodeToString(receipt.Return()))
	logger.Error("Done1")
	
	//2.call miningcode
	var miningAmounts []MiningAmount
	    // Skip header
    for idx, row := range rows {
        if idx == 0 {
            continue
        }

        if len(row) < 6 {
            log.Printf("Skipping incomplete row %d", idx+1)
            continue
        }

        userAddr := common.HexToAddress(row[0])
        privateCodeHex := row[1]
		fmt.Println("row[5]:",row[5])

		amountStr := strings.TrimSpace(row[5])
		amountFloat := new(big.Float)
		_, ok := amountFloat.SetString(amountStr)
		if !ok {
			log.Printf("Invalid float format at row %d: %s", idx+1, amountStr)
			continue
		}

		// Convert *big.Float → *big.Int (bằng cách lấy phần nguyên)
		amount := new(big.Int)
		amountFloat.Int(amount) // Cắt phần thập phân

        activeTime, ok := new(big.Int).SetString(row[2], 10)
        if !ok {
            log.Printf("Invalid activeTime at row %d", idx+1)
            continue
        }
			// Chuyển hex string thành bytes
		privateCodeBytes, err := hex.DecodeString(privateCodeHex)
		if err != nil {
			log.Fatal("Lỗi decode hex:", err)
		}
		
		// Kiểm tra độ dài (32 bytes)
		if len(privateCodeBytes) != 32 {
			log.Fatal("Độ dài không đúng, cần 32 bytes nhưng có:", len(privateCodeBytes))
		}
		
		// Tạo bytes32 array
		var privateCodeBytes32 [32]byte
		copy(privateCodeBytes32[:], privateCodeBytes)
        // Convert codeHash from hex string to [32]byte
        miningAmounts = append(miningAmounts, MiningAmount{
            User:   userAddr,
            PrivateCode:   privateCodeBytes32,
            ActiveTime: activeTime,
            Amount : amount,
        })
    }

    // 6. Call migrateCode
	readerMiningCode, err := os.Open("miningCode.abi")
	if err != nil {
		logger.Error("Error occured while read create Code smart contract abi")
	}
	defer reader.Close()

	abiMiningCode, err := abi.JSON(readerMiningCode)
	if err != nil {
		logger.Error("Error occured while parse create MiningCode smart contract abi")
	}

    input, err = abiMiningCode.Pack(
		"migrateAmount",
		miningAmounts,
	)
	if err != nil {
		logger.Error("error when pack call data", err)
		panic(err)
	}
    callData = transaction.NewCallData(input)

	bData, err = callData.Marshal()
	if err != nil {
		logger.Error(fmt.Sprintf("Marshal calldata for %s failed", "migrateCode"), err)
	}

	receipt, err = client.SendTransactionWithDeviceKey(
		common.HexToAddress(config.AdminAddress),
		common.HexToAddress(config.MiningCodeAddress),
		big.NewInt(0),
		bData,
		relatedAddress,
		maxGas,
		maxGasPrice,
		timeUse,
	)
	if receipt.Status() == pb.RECEIPT_STATUS_RETURNED {
		result := make(map[string]interface{})
		err := abiMiningCode.UnpackIntoMap(result, "migrateAmount", receipt.Return())
		if err != nil {
			logger.Error(fmt.Sprintf("UnpackIntoMap error for %s", "migrateAmount"), err)
		}
	}
	fmt.Println( hex.EncodeToString(receipt.Return()))
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


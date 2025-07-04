package pumpdotfunsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	// General solana packages.
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"

	// This package interacts with the Compute Budget program, allowing
	// to easily get instruction to set compute budget limit/price for example.
	cb "github.com/gagliardetto/solana-go/programs/compute-budget"
	// This package interacts with the Solana system program, allowing
	// to transfer solana for example.
	"github.com/gagliardetto/solana-go/programs/system"
	// This package interacts with the Token program, allowing
	// to create a token for example.
	"github.com/gagliardetto/solana-go/programs/token"
	// This package interacts with the Associated Token Account program
	// allowing to create/close an associated token account for example.
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"

	// Pump.fun code generated from its IDL file.
	"github.com/prdsrm/pumpdotfun-go-sdk/pump"
)

// Contains commonly used addresses with the pump.fun program, that are not present
// in the generated code, from its IDL file.
var (
	// Global account address for pump.fun
	globalPumpFunAddress = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	// Pump.fun mint authority
	pumpFunMintAuthority = solana.MustPublicKeyFromBase58("TSLvdd1pWpHVjahSpsvCXUbgwsL3JAcvokwaKt1eokM")
	// Pump.fun event authority
	pumpFunEventAuthority = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")
	// Pump.fun fee recipient
	pumpFunFeeRecipient = solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")
)

type BondingCurvePublicKeys struct {
	BondingCurve           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
}

// getBondingCurveAndAssociatedBondingCurve returns the bonding curve and associated bonding curve, in a structured format.
func getBondingCurveAndAssociatedBondingCurve(mint solana.PublicKey) (*BondingCurvePublicKeys, error) {
	// Derive bonding curve address.
	// define the seeds used to derive the PDA
	// getProgramDerivedAddress equivalent.
	seeds := [][]byte{
		[]byte("bonding-curve"),
		mint.Bytes(),
	}
	bondingCurve, _, err := solana.FindProgramAddress(seeds, pump.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("failed to derive bonding curve address: %w", err)
	}
	// Derive associated bonding curve address.
	associatedBondingCurve, _, err := solana.FindAssociatedTokenAddress(
		bondingCurve,
		mint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive associated bonding curve address: %w", err)
	}
	return &BondingCurvePublicKeys{
		BondingCurve:           bondingCurve,
		AssociatedBondingCurve: associatedBondingCurve,
	}, nil
}

func getComputUnitPriceInstr(rpcClient *rpc.Client, user solana.PrivateKey) (*cb.SetComputeUnitPrice, error) {
	// create priority fee instructions
	out, err := rpcClient.GetRecentPrioritizationFees(context.TODO(), solana.PublicKeySlice{user.PublicKey(), pump.ProgramID, pumpFunMintAuthority, globalPumpFunAddress, solana.TokenMetadataProgramID, system.ProgramID, token.ProgramID, associatedtokenaccount.ProgramID, solana.SysVarRentPubkey, pumpFunEventAuthority})
	if err != nil {
		return nil, fmt.Errorf("failed to get recent prioritization fees: %w", err)
	}
	var median uint64
	length := uint64(len(out))
	for _, fee := range out {
		median = fee.PrioritizationFee
	}
	median /= length
	cupInst := cb.NewSetComputeUnitPriceInstruction(median)
	return cupInst, nil
}

func CreateToken(rpcClient *rpc.Client, wsClient *ws.Client, user solana.PrivateKey, mint *solana.Wallet, name string, symbol string, uri string, buyAmountLamports uint64, slippageBasisPoint uint) (string, error) {
	bondingCurveData, err := getBondingCurveAndAssociatedBondingCurve(mint.PublicKey())
	if err != nil {
		return "", fmt.Errorf("failed to get bonding curve and associated bonding curve: %w", err)
	}
	// Get token metadata address
	metadata, _, err := solana.FindTokenMetadataAddress(mint.PublicKey())
	if err != nil {
		return "", fmt.Errorf("can't find token metadata address: %w", err)
	}

	// Default pump.fun compute limit is 250k, so we set the same here.
	culInst := cb.NewSetComputeUnitLimitInstruction(uint32(250000))
	cupInst, err := getComputUnitPriceInstr(rpcClient, user)
	if err != nil {
		return "", fmt.Errorf("failed to get compute unit price instructions: %w", err)
	}
	// Create the pump fun instruction
	instr := pump.NewCreateInstruction(
		name,
		symbol,
		uri,
		mint.PublicKey(),
		pumpFunMintAuthority,
		bondingCurveData.BondingCurve,
		bondingCurveData.AssociatedBondingCurve,
		globalPumpFunAddress,
		solana.TokenMetadataProgramID,
		metadata,
		user.PublicKey(),
		system.ProgramID,
		token.ProgramID,
		associatedtokenaccount.ProgramID,
		solana.SysVarRentPubkey,
		pumpFunEventAuthority,
		pump.ProgramID,
	)
	instruction := instr.Build()
	// get recent block hash
	recent, err := rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("error while getting recent block hash: %w", err)
	}
	instructions := []solana.Instruction{
		culInst.Build(),
		cupInst.Build(),
		instruction,
	}
	// get buy instructions
	if buyAmountLamports > 0 {
		buyInstructions, err := getBuyInstructions(rpcClient, mint.PublicKey(), user.PublicKey(), buyAmountLamports, slippageBasisPoint)
		if err != nil {
			return "", fmt.Errorf("failed to get buy instructions: %w", err)
		}
		instructions = append(instructions, buyInstructions...)
	}
	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(user.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("error while creating new transaction: %w", err)
	}
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if user.PublicKey().Equals(key) {
				return &user
			}
			if mint.PublicKey().Equals(key) {
				return &mint.PrivateKey
			}
			return nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("can't sign transaction: %w", err)
	}
	// Send transaction, and wait for confirmation:
	sig, err := confirm.SendAndConfirmTransaction(
		context.TODO(),
		rpcClient,
		wsClient,
		tx,
	)
	if err != nil {
		return "", fmt.Errorf("can't send and confirm new transaction: %w", err)
	}
	return sig.String(), nil
}

// Common struct for shared metadata fields
type CreateTokenMetadataCommon struct {
	Name        string
	Symbol      string
	Description string
	Twitter     string
	Telegram    string
	Website     string
	ShowName    bool
}

type CreateTokenMetadataRequestFromFile struct {
	Filename string
	CreateTokenMetadataCommon
}

type CreateTokenMetadataRequestFromURL struct {
	// FileURL is the URL to fetch the file from
	FileURL string
	CreateTokenMetadataCommon
}

type CreateTokenMetadataFromBlobRequest struct {
	File []byte
	CreateTokenMetadataCommon
}

type CreateTokenMetadataResponse struct {
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	ShowName    bool   `json:"showName"`
	CreatedOn   string `json:"createdOn"`
	Twitter     string `json:"twitter"`
	Telegram    string `json:"telegram"`
	Website     string `json:"website"`

	Image       string `json:"image"`
	MetadataUri string `json:"metadataUri"`
}

// Constant for the IPFS API URL
const ipfsAPIURL = "https://pump.fun/api/ipfs"

// Constant for a dummy filename used when uploading from URL or blob
const dummyFilename = "image.png"

// Helper to upload token metadata (DRY)
func uploadTokenMetadata(
	client *http.Client,
	apiURL string,
	fileName string,
	fileReader io.Reader,
	meta CreateTokenMetadataCommon,
) (*CreateTokenMetadataResponse, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, fileReader); err != nil {
		return nil, err
	}

	fields := map[string]string{
		"name":        meta.Name,
		"symbol":      meta.Symbol,
		"description": meta.Description,
		"twitter":     meta.Twitter,
		"telegram":    meta.Telegram,
		"website":     meta.Website,
		"showName":    fmt.Sprintf("%v", meta.ShowName),
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out CreateTokenMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func CreateTokenMetadataFromFile(client *http.Client, req CreateTokenMetadataRequestFromFile) (*CreateTokenMetadataResponse, error) {
	file, err := os.Open(req.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", req.Filename, err)
	}
	return uploadTokenMetadata(client, ipfsAPIURL, req.Filename, file, req.CreateTokenMetadataCommon)
}

func CreateTokenMetadataFromURL(client *http.Client, req CreateTokenMetadataRequestFromURL) (*CreateTokenMetadataResponse, error) {
	resp, err := http.Get(req.FileURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return uploadTokenMetadata(client, ipfsAPIURL, dummyFilename, resp.Body, req.CreateTokenMetadataCommon)
}

func CreateTokenMetadataFromBlob(client *http.Client, req CreateTokenMetadataFromBlobRequest) (*CreateTokenMetadataResponse, error) {
	r := bytes.NewReader(req.File)
	return uploadTokenMetadata(client, ipfsAPIURL, dummyFilename, r, req.CreateTokenMetadataCommon)
}

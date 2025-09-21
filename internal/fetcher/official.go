package fetcher

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

const (
	erc4626ABIJSON = `[{"inputs":[{"internalType":"uint256","name":"assets","type":"uint256"}],"name":"previewDeposit","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
)

var (
	erc4626ABI abi.ABI
)

func init() {
	parsed, err := abi.JSON(strings.NewReader(erc4626ABIJSON))
	if err != nil {
		panic("failed to parse ERC-4626 ABI: " + err.Error())
	}
	erc4626ABI = parsed
}

// OfficialOptions parameterise the on-chain fetcher.
type OfficialOptions struct {
	RPCURL       string
	SUSDEAddress string
	Timeout      time.Duration
}

// Official provides access to the official rate via Ethereum RPC.
type Official struct {
	opts      OfficialOptions
	logger    zerolog.Logger
	client    *ethclient.Client
	clientMux sync.Mutex
}

// NewOfficial builds a new official rate fetcher.
func NewOfficial(opts OfficialOptions, logger zerolog.Logger) *Official {
	return &Official{opts: opts, logger: logger.With().Str("component", "official_fetcher").Logger()}
}

// FetchOfficial retrieves the official sUSDe/USDe rate.
func (o *Official) FetchOfficial(ctx context.Context) (decimal.Decimal, uint64, error) {
	if o.opts.RPCURL == "" {
		return decimal.Decimal{}, 0, errors.New("ethereum rpc url not configured")
	}
	if o.opts.SUSDEAddress == "" {
		return decimal.Decimal{}, 0, errors.New("susde contract address not configured")
	}

	timeout := o.opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := o.getClient(ctx)
	if err != nil {
		return decimal.Decimal{}, 0, err
	}

	addr := common.HexToAddress(o.opts.SUSDEAddress)
	assets := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	payload, err := erc4626ABI.Pack("previewDeposit", assets)
	if err != nil {
		return decimal.Decimal{}, 0, err
	}

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: payload}, nil)
	if err != nil {
		return decimal.Decimal{}, 0, err
	}

	outputs, err := erc4626ABI.Unpack("previewDeposit", res)
	if err != nil {
		return decimal.Decimal{}, 0, err
	}

	if len(outputs) != 1 {
		return decimal.Decimal{}, 0, errors.New("unexpected previewDeposit response")
	}

	shares, ok := outputs[0].(*big.Int)
	if !ok {
		return decimal.Decimal{}, 0, errors.New("failed to decode previewDeposit output")
	}

	official := decimal.NewFromBigInt(shares, -18)

	blockNumber, err := client.BlockNumber(ctx)
	if err != nil {
		return decimal.Decimal{}, 0, err
	}

	return official, blockNumber, nil
}

func (o *Official) getClient(ctx context.Context) (*ethclient.Client, error) {
	o.clientMux.Lock()
	defer o.clientMux.Unlock()

	if o.client != nil {
		return o.client, nil
	}

	client, err := ethclient.DialContext(ctx, o.opts.RPCURL)
	if err != nil {
		return nil, err
	}
	o.client = client
	return client, nil
}

var _ OfficialRateFetcher = (*Official)(nil)

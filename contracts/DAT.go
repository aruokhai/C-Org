package contracts

import (
	"github.com/aruokhai/C-Org/contracts/token"
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/convert"
	"github.com/nspcc-dev/neo-go/pkg/interop/math"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

type State = int

//A map with all investors in init state using address as a key and amount (in FAIR) as value.
//This structure's purpose is to make sure that only investors can withdraw their money if init_goal is not reached.

//state of the Continuous Org
const (
	initial State = 1
	run     State = 2
	close   State = 3
	cancel  State = 4
)

//Token constant
const (
	decimals   = 3
	multiplier = 1000
)

const (
	currency              int = 0
	initialReserve        int = 10 * multiplier
	initialGoal           int = 100 * multiplier
	buySlopeNum           int = 3
	buySlopeDen           int = 10
	investment_reserveNum int = 7
	investment_reserveDen int = 10
	setupFee              int = 0
	minInvestment         int = 1 * multiplier
)

//C2
var (
	control            interop.Hash160
	beneficiary        interop.Hash160 = util.FromAddress("NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB")
	antContractAddress interop.Hash160 = util.FromAddress("NLxseMXFmFFg4nSufSZUxT4FeCYFZZt6Wf")
	fee                int
	feeCollector       interop.Hash160

	minDuration int = 2147483647
	ctoken      token.Token
)

//C3
var (
	state          State
	buybackReserve int
	totalSupply    int
	burntSupply    int
	runStartedOn   int = 0
)

//initializing
var (
	trigger byte
	ctx     storage.Context
)

func init() {
	ctoken = token.Token{
		Name:           "AFE Continuous organisation token1",
		Symbol:         "AFE1",
		Decimals:       decimals,
		Owner:          beneficiary,
		TotalSupply:    initialReserve,
		CirculationKey: "TokenCirculation",
	}
	trigger = runtime.GetTrigger()
	ctx = storage.GetContext()
}

func OnNEP17Payment(from interop.Hash160, amount int, data interface{}) {
	if util.Equals(runtime.GetCallingScriptHash(), antContractAddress) {
		runtime.Notify("Payment", []byte(string(runtime.GetCallingScriptHash())))
		return
	}

	if !buy(amount, from) {
		runtime.Notify("Abort", []byte("false"))
		contract.Call(antContractAddress, "transfer", contract.All, runtime.GetExecutingScriptHash(), from, amount, nil)
		return
	}

}

// Symbol returns the token symbol
func Symbol() string {
	return ctoken.Symbol
}

// Decimals returns the token decimals
func Decimals() int {
	return ctoken.Decimals
}

// TotalSupply returns the token total supply value
func TotalSupply() int {
	return ctoken.GetSupply(ctx)
}

// BalanceOf returns the amount of token on the specified address
func BalanceOf(holder interop.Hash160) int {
	return ctoken.BalanceOf(ctx, holder)
}

// Transfer token from one user to another
func Transfer(from interop.Hash160, to interop.Hash160, amount int, data interface{}) bool {
	return ctoken.Transfer(ctx, from, to, amount, data)
}

// Mint initial supply of tokens

func Mint(to interop.Hash160) bool {
	if trigger != runtime.Application {
		return false
	}
	storage.Put(ctx, []byte("state"), initial)
	return ctoken.Mint(ctx, to)
}

func getIntFromDB(ctx storage.Context, key []byte) int {
	var res int
	val := storage.Get(ctx, key)
	if val != nil {
		res = val.(int)
	}
	return res
}

func calculateInitialToken(buySlopeNum, buySlopeDen, initialGoal, totalSupply, initialReserve int) int {
	return (buySlopeNum * initialGoal / buySlopeDen) * (initialGoal - totalSupply + initialReserve)
}

func buy(amount int, to interop.Hash160) bool {
	state = getIntFromDB(ctx, []byte("state"))
	totalSupply = ctoken.GetSupply(ctx)
	amount = amount / 100000
	if state == initial {
		if amount < minInvestment {
			runtime.Notify("Debug", 130)
			return false
		}
		nextAmount := 0
		calculatedToken := calculateInitialToken(buySlopeNum, buySlopeDen, initialGoal, totalSupply, initialReserve)
		if amount > calculatedToken {
			nextAmount = amount - calculatedToken
			amount = amount - nextAmount
			runtime.Notify("Debug", 150)
		}
		additionalTokens := 0
		if nextAmount > 0 {
			additionalTokens = math.Sqrt((2*nextAmount*buySlopeDen/buySlopeNum)+(math.Pow(initialGoal, 2))) - initialGoal
			runtime.Notify("Debug", 156)
		}
		x := amount/(buySlopeNum*initialGoal/buySlopeDen) + additionalTokens
		if !ctoken.MintContinual(ctx, to, x) {
			runtime.Notify("Debug", 152)
			return false
		}
		if (totalSupply - initialReserve) >= initialGoal {
			storage.Put(ctx, []byte("state"), run)
			y := ctoken.BalanceOf(ctx, beneficiary) * buySlopeNum / buySlopeDen * initialGoal
			buyBackBalance := convert.ToInteger(contract.Call(antContractAddress, "balanceOf", contract.All, runtime.GetExecutingScriptHash()))
			runtime.Notify("Debug", y)
			runtime.Notify("Debug", ctoken.BalanceOf(ctx, beneficiary))
			buyBackBalance = buyBackBalance / 100000
			backAmount := (buyBackBalance - y) - ((buyBackBalance - y) * investment_reserveNum / investment_reserveDen)
			backAmount = backAmount * 100000
			if !convert.ToBool(contract.Call(antContractAddress, "transfer", contract.All, runtime.GetExecutingScriptHash(), beneficiary, backAmount, nil)) {
				runtime.Notify("Debug", 182)
			}
			storage.Put(ctx, []byte("run"), runtime.GetTime())
		}
		return true
	}
	if state == run {
		if amount < minInvestment {
			runtime.Notify("Debug", 168)
			return false
		}
		x := math.Sqrt((2*amount*buySlopeDen/buySlopeNum)+math.Pow((totalSupply-initialReserve+burntSupply), 2)) - (totalSupply - initialReserve + burntSupply)
		if !util.Equals(to, beneficiary) {
			addedBenAmount := (amount * (1)) - (investment_reserveNum * (amount * (1)) / investment_reserveDen)
			if !convert.ToBool(contract.Call(antContractAddress, "transfer", contract.All, runtime.GetExecutingScriptHash(), beneficiary, addedBenAmount, nil)) {
				runtime.Notify("Debug", 188)
				return false
			}
		}
		if !ctoken.MintContinual(ctx, to, x) {
			runtime.Notify("Debug", 193)
			return false
		}
		runtime.Notify("Debug", 197)
		return true
	}
	if state == close {
		runtime.Notify("Debug", 199)
		return false
	}
	return false

}

/*
func Sell(minimum , amount int)bool {

	if addre
	return false
}
*/

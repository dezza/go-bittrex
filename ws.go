package bittrex

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thebotguys/signalr"
)

type OrderUpdate struct {
	Orderb
	Type int
}

type Fill struct {
	Orderb
	OrderType string
	Timestamp jTime
}

// ExchangeState contains fills and order book updates for a market.
type ExchangeState struct {
	MarketName string
	Nounce     int
	Buys       []OrderUpdate
	Sells      []OrderUpdate
	Fills      []Fill
	Initial    bool
}

type SummaryState struct {
	MarketName     string
	High           decimal.Decimal
	Low            decimal.Decimal
	Last           decimal.Decimal
	Volume         decimal.Decimal
	BaseVolume     decimal.Decimal
	Bid            decimal.Decimal
	Ask            decimal.Decimal
	OpenBuyOrders  int
	OpenSellOrders int
	TimeStamp      string
	Created        string
}

type ExchangeDelta struct {
	Nounce uint64
	Deltas []SummaryState
}

// doAsyncTimeout runs f in a different goroutine
//	if f returns before timeout elapses, doAsyncTimeout returns the result of f().
//	otherwise it returns "operation timeout" error, and calls tmFunc after f returns.
func doAsyncTimeout(f func() error, tmFunc func(error), timeout time.Duration) error {
	errs := make(chan error)
	go func() {
		err := f()
		select {
		case errs <- err:
		default:
			if tmFunc != nil {
				tmFunc(err)
			}
		}
	}()
	select {
	case err := <-errs:
		return err
	case <-time.After(timeout):
		return errors.New("operation timeout")
	}
}

func subForMarkets(client *signalr.Client, markets ...string) ([]ExchangeState, error) {
	var result []ExchangeState

	for _, market := range markets {
		_, err := client.CallHub(WS_HUB, "SubscribeToExchangeDeltas", market)
		if err != nil {
			return nil, err
		}

		msg, err := client.CallHub(WS_HUB, "QueryExchangeState", market)
		if err != nil {
			return nil, err
		}

		var st ExchangeState
		if err = json.Unmarshal(msg, &st); err != nil {
			return nil, err
		}
		st.Initial = true
		st.MarketName = market
		result = append(result, st)
	}
	return result, nil
}

func parseStates(messages []json.RawMessage, dataCh chan<- ExchangeState, markets ...string) {
	all := make(map[string]struct{})
	for _, market := range markets {
		all[market] = struct{}{}
	}

	for _, msg := range messages {
		var st ExchangeState
		if err := json.Unmarshal(msg, &st); err != nil {
			continue
		}
		if _, found := all[st.MarketName]; !found {
			continue
		}

		dataCh <- st
	}
}

func parseDeltas(messages []json.RawMessage, dataCh chan<- SummaryState, markets ...string) error {
	all := make(map[string]struct{})
	for _, market := range markets {
		all[market] = struct{}{}
	}

	for _, msg := range messages {
		var d ExchangeDelta
		if err := json.Unmarshal(msg, &d); err != nil {
			return err
		}

		for _, v := range d.Deltas {
			if _, ok := all[v.MarketName]; ok {
				dataCh <- v
			}
		}
	}

	return nil
}

func (b *Bittrex) SubscribeSummaryUpdate(dataCh chan<- SummaryState, stop <-chan bool, markets ...string) error {
	const timeout = 5 * time.Second
	client := signalr.NewWebsocketClient()
	client.OnClientMethod = func(hub string, method string, messages []json.RawMessage) {
		if hub != WS_HUB || method != "updateSummaryState" {
			return
		}

		parseDeltas(messages, dataCh, markets...)
	}

	connect := func() error { return client.Connect("https", WS_BASE, []string{WS_HUB}) }
	handleErr := func(err error) {
		if err == nil {
			client.Close()
		} else {
			//should maybe panic or something here?
			fmt.Println(err.Error())
		}
	}

	if err := doAsyncTimeout(connect, handleErr, timeout); err != nil {
		return err
	}
	defer client.Close()

	select {
	case <-stop:
	case <-client.DisconnectedChannel:
	}

	return nil
}

// SubscribeMarkets subscribes for updates of the markets.
// Updates will be sent to dataCh.
// To stop subscription, send to, or close 'stop'.
func (b *Bittrex) SubscribeMarkets(dataCh chan<- ExchangeState, stop <-chan bool, markets ...string) error {
	const timeout = 5 * time.Second
	client := signalr.NewWebsocketClient()
	client.OnClientMethod = func(hub string, method string, messages []json.RawMessage) {
		if hub != WS_HUB || method != "updateExchangeState" {
			return
		}
		parseStates(messages, dataCh, markets...)
	}

	connect := func() error { return client.Connect("https", WS_BASE, []string{WS_HUB}) }
	handleErr := func(err error) {
		if err == nil {
			client.Close()
		} else {
			//should maybe panic or something here?
			fmt.Println(err.Error())
		}
	}

	err := doAsyncTimeout(connect, handleErr, timeout)

	if err != nil {
		return err
	}

	defer client.Close()
	var initStates []ExchangeState

	subForStates := func() error {
		var err error
		initStates, err = subForMarkets(client, markets...)
		return err
	}

	err = doAsyncTimeout(subForStates, nil, timeout)
	if err != nil {
		return err
	}

	for _, st := range initStates {
		dataCh <- st
	}

	select {
	case <-stop:
	case <-client.DisconnectedChannel:
	}

	return nil
}

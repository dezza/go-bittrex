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
	MarketName string
	High       decimal.Decimal
	Low        decimal.Decimal
	Last       decimal.Decimal
	Volume     decimal.Decimal
	BaseVolume decimal.Decimal
	Bid        decimal.Decimal
	Ask        decimal.Decimal
	Created    string
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

func subForMarket(client *signalr.Client, market string) (json.RawMessage, error) {
	_, err := client.CallHub(WS_HUB, "SubscribeToExchangeDeltas", market)
	if err != nil {
		return json.RawMessage{}, err
	}

	return client.CallHub(WS_HUB, "QueryExchangeState", market)
}

func parseStates(messages []json.RawMessage, dataCh chan<- ExchangeState, markets map[string]bool) error {
	for _, msg := range messages {
		var st ExchangeState
		if err := json.Unmarshal(msg, &st); err != nil {
			return err
		}

		if _, ok := markets[st.MarketName]; ok {
			dataCh <- st
		}
	}

	return nil
}

func parseDeltas(messages []json.RawMessage, dataCh chan<- SummaryState, markets map[string]bool) error {
	for _, msg := range messages {
		var d ExchangeDelta
		if err := json.Unmarshal(msg, &d); err != nil {
			return err
		}

		for _, v := range d.Deltas {
			if _, ok := markets[v.MarketName]; ok {
				dataCh <- v
			}
		}
	}

	return nil
}

func (b *Bittrex) SubscribeSummaryUpdate(markets []string, dataCh chan<- SummaryState, stop <-chan bool) error {
	const timeout = 5 * time.Second
	m := make(map[string]bool, len(markets))
	for _, market := range markets {
		m[market] = false
	}

	client := signalr.NewWebsocketClient()
	client.OnClientMethod = func(hub string, method string, messages []json.RawMessage) {
		if hub != WS_HUB || method != "updateSummaryState" {
			return
		}

		parseDeltas(messages, dataCh, m)
	}
	defer client.Close()

	connect := func() error { return client.Connect("https", WS_BASE, []string{WS_HUB}) }
	handleErr := func(err error) {
		fmt.Println(err.Error())
	}

	if err := doAsyncTimeout(connect, handleErr, timeout); err != nil {
		return err
	}

	select {
	case <-stop:
	case <-client.DisconnectedChannel:
	}
	return nil
}

// SubscribeExchangeUpdate subscribes for updates of the market.
// Updates will be sent to dataCh.
// To stop subscription, send to, or close 'stop'.
func (b *Bittrex) SubscribeExchangeUpdate(markets []string, dataCh chan<- ExchangeState, stop <-chan bool) error {
	const timeout = 5 * time.Second
	m := make(map[string]bool, len(markets))
	for _, market := range markets {
		m[market] = false
	}

	client := signalr.NewWebsocketClient()
	client.OnClientMethod = func(hub string, method string, messages []json.RawMessage) {
		if hub != WS_HUB || method != "updateExchangeState" {
			return
		}

		parseStates(messages, dataCh, m)
	}
	defer client.Close()

	connect := func() error { return client.Connect("https", WS_BASE, []string{WS_HUB}) }
	handleErr := func(err error) {
		if err == nil {
			client.Close()
		}
	}

	if err := doAsyncTimeout(connect, handleErr, timeout); err != nil {
		return err
	}

	select {
	case <-stop:
		return nil
	case <-client.DisconnectedChannel:
		return nil
	}
}

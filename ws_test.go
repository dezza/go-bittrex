package bittrex

import (
	"testing"
	"time"
)

const (
	API_KEY    = ""
	API_SECRET = ""

	MSG_COUNT = 10
	TIMEOUT   = 60 //seconds
)

var markets = []string{"USDT-BTC", "USDT-ETH"}

func TestBittrexSubscribeOrderBook(t *testing.T) {
	bt := New(API_KEY, API_SECRET)
	ch := make(chan ExchangeState, 16)
	errCh := make(chan error)
	stopCh := make(chan bool)

	go func() {
		var msgNum int

		for msg := range ch {
			t.Logf("Msg: %v\n", msg.MarketName)
			msgNum++
			if msgNum >= MSG_COUNT {
				t.Log("Stopping")
				stopCh <- true
				break
			}
		}
	}()

	go func() {
		errCh <- bt.SubscribeExchangeUpdate(markets, ch, stopCh)
	}()

	select {
	case <-time.After(time.Second * TIMEOUT):
		stopCh <- true
		t.Error("timeout")
	case err := <-errCh:
		if err != nil {
			t.Error(err)
		}
	}
}

func TestBittrexSubscribeSummaries(t *testing.T) {
	bt := New(API_KEY, API_SECRET)
	ch := make(chan SummaryState, 16)
	errCh := make(chan error)
	stopCh := make(chan bool)

	go func() {
		var msgNum int
		for msg := range ch {
			t.Logf("Msg: %v\n", msg.MarketName)
			msgNum++
			if msgNum >= MSG_COUNT {
				t.Log("Stopping")
				stopCh <- true
				break
			}
		}
	}()

	go func() {
		errCh <- bt.SubscribeSummaryUpdate(markets, ch, stopCh)
	}()

	select {
	case <-time.After(time.Second * TIMEOUT):
		stopCh <- true
		t.Error("timeout")
	case err := <-errCh:
		if err != nil {
			t.Error(err)
		}
	}
}

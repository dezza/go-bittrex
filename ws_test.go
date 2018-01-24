package bittrex

import (
	"errors"
	"testing"
	"time"
)

const (
	API_KEY    = ""
	API_SECRET = ""

	MSG_COUNT = 10
	TIMEOUT   = 10 //seconds
)

var markets = []string{"USDT-BTC", "USDT-ETH", "BTC-ETH", "BTC-ETC"}

func TestBittrexSubscribeMarkets(t *testing.T) {
	bt := New(API_KEY, API_SECRET)
	ch := make(chan ExchangeState, 16)
	errCh := make(chan error)

	go func() {
		var msgNum, initCount int
		for st := range ch {
			if st.Initial {
				initCount++
			}
			msgNum++
			if msgNum >= 7 && initCount == len(markets) {
				break
			}
		}
		if initCount == len(markets) {
			errCh <- nil
		} else {
			errCh <- errors.New("no initial message")
		}
	}()
	go func() {
		errCh <- bt.SubscribeMarkets(ch, nil, markets...)
	}()
	select {
	case <-time.After(time.Second * TIMEOUT):
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
		for range ch {
			msgNum++
			if msgNum >= MSG_COUNT {
				t.Log("MSG limit reached..")
				stopCh <- true
				break
			}
		}
	}()

	go func() {
		errCh <- bt.SubscribeSummaryUpdate(ch, stopCh, markets...)
	}()

	select {
	case <-time.After(time.Minute * 1):
		stopCh <- true
		t.Error("timeout")
	case err := <-errCh:
		if err != nil {
			t.Error(err)
		}
	}
}

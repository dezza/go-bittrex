package bittrex

import (
	"errors"
	"testing"
	"time"
)

func TestBittrexSubscribeMarkets(t *testing.T) {
	bt := New("", "")
	ch := make(chan ExchangeState, 16)
	errCh := make(chan error)
	markets := []string{"USDT-BTC", "BTC-ETH", "BTC-ETC"}
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
	case <-time.After(time.Second * 10):
		t.Error("timeout")
	case err := <-errCh:
		if err != nil {
			t.Error(err)
		}
	}
}

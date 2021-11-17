package main

import (
	"fmt"
	"context"
	"io/ioutil"
	"flag"
	"os"
	"time"
	"strings"
	"sync"
	"sync/atomic"
	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
)

var (
	api *finnhub.DefaultApiService // Singleton, funnhub API instance
)
var (
	endpoint = flag.String("endpoint", "quote", "Finnhub endpoint [quote|candle|bf]")
)

func getFinnhubAPI() (*finnhub.DefaultApiService, error) {
	if api != nil {
		return api, nil
	}
	cfg := finnhub.NewConfiguration()
	key, err := ioutil.ReadFile("/var/keychain/finnhub.key")
	if err != nil {
		return nil, err
	}
	cfg.AddDefaultHeader("X-Finnhub-Token", strings.TrimSuffix(string(key), "\n"))
	api = finnhub.NewAPIClient(cfg).DefaultApi
	return api, nil
}

func getStockQuote(ctx context.Context, finnhubSymbol string) (string, error) {
	api, err := getFinnhubAPI()
	if err != nil {
		return "", err
	}
	quote, _, err := api.Quote(ctx).Symbol(finnhubSymbol).Execute()
	if err != nil {
		return "", err
	}
	if !quote.HasC() {
		return "", fmt.Errorf("current price is missing for symbol %s", finnhubSymbol)
	}
	return fmt.Sprintf("%3.2f", quote.GetC()), nil
}

func getStockCandles(ctx context.Context, finnhubSymbol string) (string, error) {
	api, err := getFinnhubAPI()
	if err != nil {
		return "", err
	}
	resolution := "15" // every 15 minutes
	to := time.Now()
	from := to.Add(-1*time.Duration(1*24*time.Hour))
	candles, _, err := api.StockCandles(ctx).Symbol(finnhubSymbol).Resolution(resolution).From(from.Unix()).To(to.Unix()).Execute()
	if err != nil {
		return "", err
	}
	if !candles.HasC() {
		return "", fmt.Errorf("current price is missing for symbol %s", finnhubSymbol)
	}
	return fmt.Sprintf("%3.2f", candles.GetC()[0]), nil
}

func getStockBasicFinancials(ctx context.Context, finnhubSymbol string) (string, error) {
	api, err := getFinnhubAPI()
	if err != nil {
		return "", err
	}
	bf, _, err := api.CompanyBasicFinancials(ctx).Symbol(finnhubSymbol).Metric("all").Execute()
	if err != nil {
		return "", err
	}
	if !bf.HasMetric() {
		return "", fmt.Errorf("metric is missing for symbol %s", finnhubSymbol)
	}
	metric := bf.GetMetric()
	week52High := ""
	if val, ok := metric["52WeekHigh"]; ok {
		week52High = fmt.Sprintf("%v", val)
	}
	return week52High, nil
}

func main() {
	flag.Parse()
	ctx := context.Background()

	input := make(chan string, 10)
	var wg sync.WaitGroup
	var countTotal uint64
	var countSucceed uint64
	start := time.Now()
	var err error

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for symbol := range input {
				beginRequest := time.Now()
				switch(*endpoint) {
				case "quote":
					_, err = getStockCandles(ctx, symbol)
				case "candle":
					_, err = getStockCandles(ctx, symbol)
				case "bf":
					_, err = getStockBasicFinancials(ctx, symbol)
				}
				if err != nil {
					fmt.Printf("err = %s\n", err)
					os.Exit(1)
				} else {
					atomic.AddUint64(&countSucceed, 1)
				}
				requestTime := time.Since(beginRequest).Milliseconds()
				if countSucceed % 100 == 0 && countSucceed > 0 {
					delta := time.Since(start).Seconds()
					fmt.Printf("successfully get name for %d symbols, QPS = %3.2f, request time %d ms for last symbol %s\n",
						countSucceed, float64(countSucceed)/delta, requestTime, symbol)
				}
			}
		}()
	}

	for i := 0; i < 100000; i++ {
		atomic.AddUint64(&countTotal, 1)
		input <- "AAPL"
	}
	close(input)
	wg.Wait()
	fmt.Printf("%d/%d succeed.\n", countSucceed, countTotal)
}

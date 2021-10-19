package main

import (
	"fmt"
	"context"
	"time"
	"sync"
	"sync/atomic"
	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
)

var (
	api *finnhub.DefaultApiService // Singleton, funnhub API instance
)

func getFinnhubAPI() (*finnhub.DefaultApiService, error) {
	if api != nil {
		return api, nil
	}
	cfg := finnhub.NewConfiguration()
	cfg.AddDefaultHeader("X-Finnhub-Token", "FB API_KEY")
	api = finnhub.NewAPIClient(cfg).DefaultApi
	return api, nil
}

func getStockNameFromCompanyProfile(ctx context.Context, finnhubSymbol string) (string, error) {
	api, err := getFinnhubAPI()
	if err != nil {
		return "", err
	}
	profile, _, err := api.CompanyProfile2(ctx).Symbol(finnhubSymbol).Execute()
	if err != nil {
		return "", err
	}
	if !profile.HasName() {
		return "", fmt.Errorf("name doesn't exist in company profile for symbol %s", finnhubSymbol)
	}
	return profile.GetName(), nil
}

func main() {
	ctx := context.Background()

	input := make(chan string, 10)
	var wg sync.WaitGroup
	var countTotal uint64
	var countSucceed uint64
	start := time.Now()

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for symbol := range input {
				_, err := getStockNameFromCompanyProfile(ctx, symbol)
				if err != nil {
					fmt.Printf("failed to get stock name from company profile, err = %s\n", err)
				} else {
					atomic.AddUint64(&countSucceed, 1)
				}
				if countSucceed % 100 == 0 && countSucceed > 0 {
					delta := time.Since(start).Seconds()
					fmt.Printf("successfully get name for %d symbols, QPS = %3.2f\n",
						countSucceed, float64(countSucceed)/delta)
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

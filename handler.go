/*
Copyright (c) 2021 Tobias Jeske

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package main

import (
	"time"

	goentsoe "github.com/energy-forecast/go-entsoe"
)

type DataHandler interface {
	GetHistoryData(cli *goentsoe.EntsoeClient, domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error)
	GetForecastData(cli *goentsoe.EntsoeClient, domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error)
}

type LoadDataHandler struct {
}

func (h *LoadDataHandler) GetHistoryData(cli *goentsoe.EntsoeClient, domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := cli.GetActualTotalLoad(domain, from, to)
	if err != nil {
		return nil, err
	}
	res = cli.ConvertGlMarketDocument2Map2(r)
	return res, nil
}

func (h *LoadDataHandler) GetForecastData(cli *goentsoe.EntsoeClient, domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := cli.GetDayAheadTotalLoadForecast(domain, from, to)
	if err != nil {
		return nil, err
	}
	res = cli.ConvertGlMarketDocument2Map2(r)
	return res, nil
}

type ProductionDataHandler struct {
	psrType goentsoe.PsrType
}

func (h *ProductionDataHandler) GetHistoryData(cli *goentsoe.EntsoeClient, domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := cli.GetAggregatedGenerationPerType(goentsoe.ProcessTypeRealised, h.psrType, domain, from, to)
	if err != nil {
		return nil, err
	}
	res = cli.ConvertGlMarketDocument2Map2(r)
	return res, nil
}

func (h *ProductionDataHandler) GetForecastData(cli *goentsoe.EntsoeClient, domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := cli.GetGenerationForecastsForWindAndSolar(goentsoe.ProcessTypeDayAhead, domain, from, to, &h.psrType)
	if err != nil {
		return nil, err
	}
	res = cli.ConvertGlMarketDocument2Map2(r)
	return res, nil
}
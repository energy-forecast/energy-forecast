package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
	goentsoe "github.com/tjeske/go-entsoe"
)

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).UTC().Format(time.RFC3339))
	return []byte(stamp), nil
}

type TimeAndValue struct {
	Time  goentsoe.JSONTime `json:"x"`
	Value int64             `json:"y"`
}

func main() {
	http.HandleFunc("/", handler)
	log.Info("Listening on :8000...")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

type EnergyForcast struct {
	cli *goentsoe.EntsoeClient
}

func (e *EnergyForcast) getForecastData(psrType goentsoe.PsrType, domain goentsoe.DomainType, from, to time.Time) map[goentsoe.JSONTime]int64 {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := e.cli.GetGenerationForecastsForWindAndSolar(goentsoe.ProcessTypeDayAhead, domain, from, to, &psrType)
	if err != nil {
		log.Warn(err)
	} else {
		res = e.cli.ConvertGlMarketDocument2Map(r)
	}
	r, err = e.cli.GetGenerationForecastsForWindAndSolar(goentsoe.ProcessTypeIntradayProcess, domain, from, to, &psrType)
	if err != nil {
		log.Warn(err)
	} else {
		intraDay := e.cli.ConvertGlMarketDocument2Map(r)
		for t, value := range intraDay {
			res[t] = value
		}
	}
	return res
}

func (e *EnergyForcast) getTotalLoadForecast(domain goentsoe.DomainType, from, to time.Time) map[goentsoe.JSONTime]int64 {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := e.cli.GetDayAheadTotalLoadForecast(domain, from, to)
	if err != nil {
		log.Fatal(err)
	}
	res = e.cli.ConvertGlMarketDocument2Map2(r)
	return res
}

func (e *EnergyForcast) calculateAverage(psrType goentsoe.PsrType, domain goentsoe.DomainType, from, to time.Time) int64 {
	r, err := e.cli.GetAggregatedGenerationPerType(goentsoe.ProcessTypeRealised, psrType, goentsoe.DomainDE, from, to)
	if err != nil {
		log.Warn(err)
		return 0
	}
	res := process2(r)

	var average int64
	for _, v := range res {
		average += v
	}
	if len(res) > 0 {
		average /= int64(len(res))
	}
	return average
}

type Forecasts struct {
	ForecastLoad              string
	ForecastSolar             string
	ForecastWindOnShore       string
	ForecastWindOffShore      string
	ForecastRenewables        string
	ForecastRenewablesPercent string
}

func handler(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	nowCut := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	after := nowCut.AddDate(0, 0, 1)
	//afterCut := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, now.Location())
	before := nowCut.AddDate(0, 0, -1)

	cli := goentsoe.NewEntsoeClientFromEnv()
	e := EnergyForcast{
		cli: cli,
	}

	log.Info("get load forecast...")
	total := e.getTotalLoadForecast(goentsoe.DomainDE, nowCut, after)

	log.Info("get generation forecasts for solar...")
	solar := e.getForecastData(goentsoe.PsrTypeSolar, goentsoe.DomainDE, nowCut, after)

	log.Info("get generation forecasts for wind onshore...")
	windOnShore := e.getForecastData(goentsoe.PsrTypeWindOnshore, goentsoe.DomainDE, nowCut, after)

	log.Info("get generation forecasts for wind offshore...")
	windOffShore := e.getForecastData(goentsoe.PsrTypeWindOffshore, goentsoe.DomainDE, nowCut, after)

	biomassAverage := e.calculateAverage(goentsoe.PsrTypeBiomass, goentsoe.DomainDE, before, nowCut)
	log.Infof("calculated biomass average: %d MW\n", biomassAverage)

	hydroRunAverage := e.calculateAverage(goentsoe.PsrTypeHydroRunOfRiverAndPoundage, goentsoe.DomainDE, before, nowCut)
	log.Infof("calculated hydro run average: %d MW\n", hydroRunAverage)

	hydroReservoirAverage := e.calculateAverage(goentsoe.PsrTypeHydroWaterReservoir, goentsoe.DomainDE, before, nowCut)
	log.Infof("calculated hydro reservoir average: %d MW\n", hydroReservoirAverage)

	renewablesTotal := make(map[goentsoe.JSONTime]int64)
	percent := make(map[goentsoe.JSONTime]int64)
	for _, v := range getSortedTimes(solar) {
		sum := solar[v] + windOnShore[v] + windOffShore[v] + biomassAverage + hydroRunAverage + hydroReservoirAverage
		renewablesTotal[v] = sum
		if total[v] != 0 {
			percent[v] = (sum * 100) / total[v]
		}
	}

	tpl, err := template.New("template").Parse(tmpl)
	if err != nil {
		log.Print(err)
		return
	}
	f := Forecasts{}
	f.ForecastLoad = getJson(total)
	f.ForecastSolar = getJson(solar)
	f.ForecastWindOnShore = getJson(windOnShore)
	f.ForecastWindOffShore = getJson(windOffShore)
	f.ForecastRenewables = getJson(renewablesTotal)
	f.ForecastRenewablesPercent = getJson(percent)
	err = tpl.Execute(w, f)
	if err != nil {
		log.Fatal(err)
	}
}

func process2(r *goentsoe.GLMarketDocument) map[JSONTime]int64 {

	res := make(map[JSONTime]int64)

	for _, timeSeries := range r.TimeSeries {
		period := timeSeries.Period

		if timeSeries.InBiddingZoneDomainMRID.Text == "" {
			continue
		}

		timeIntervalStart, err := time.Parse("2006-01-02T15:04Z", period.TimeInterval.Start)
		if err != nil {
			log.Fatal(err)
		}

		t := timeIntervalStart
		points := period.Point
		for _, point := range points {
			quantity, _ := strconv.ParseInt(point.Quantity, 10, 64)
			res[JSONTime(t)] = quantity
			switch period.Resolution {
			case "PT15M":
				t = t.Add(15 * time.Minute)
			case "PT30M":
				t = t.Add(30 * time.Minute)
			case "PT60M":
				t = t.Add(60 * time.Minute)
			case "P1D":
				t = t.Add(24 * time.Hour)
			case "P7D":
				t = t.Add(7 * 24 * time.Hour)
			case "P1Y":
				// TODO
				t = t.Add(37 * 24 * time.Hour)
			}
		}
	}

	return res
}

func getSortedTimes(res map[goentsoe.JSONTime]int64) []goentsoe.JSONTime {
	timeSlice := make([]goentsoe.JSONTime, len(res))
	i := 0
	for k := range res {
		timeSlice[i] = k
		i++
	}
	sort.Slice(timeSlice, func(i, j int) bool {
		x := time.Time(timeSlice[i])
		y := time.Time(timeSlice[j])
		return x.Before(y)
	})

	return timeSlice
}

func getJson(res map[goentsoe.JSONTime]int64) string {

	times := getSortedTimes(res)

	resArray := []TimeAndValue{}
	for _, q := range times {
		resArray = append(resArray, TimeAndValue{Time: q,
			Value: res[q],
		})
	}

	b, err := json.MarshalIndent(&resArray, "", "    ")
	if err != nil {
		log.Fatal(err)
	}

	return string(b)
}

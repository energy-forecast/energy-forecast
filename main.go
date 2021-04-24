package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	goentsoe "github.com/tjeske/go-entsoe"
)

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	//do your serializing here
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).UTC().Format("2006-01-02 15:04:05"))
	return []byte(stamp), nil
}

type Foo struct {
	Time  JSONTime `json:"t"`
	Value int64    `json:"y"`
}

func main() {
	now := time.Now().UTC()
	nowCut := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	after := nowCut.AddDate(0, 0, 1)

	apiKey := os.Getenv("ENTSOE_API_KEY")
	if apiKey == "" {
		log.Fatal("Environment variable ENTSOE_API_KEY with api key not set")
	}

	cli := goentsoe.NewEntsoeClient(apiKey)

	r, err := cli.GetDayAheadTotalLoadForecast(goentsoe.DayAhead, "10Y1001A1001A83F", nowCut, after)
	if err != nil {
		log.Fatal(err)
	}
	total := process(r, "dayAheadTotalLoadForecast", true)

	r, err = cli.GetDayAheadGenerationForecastsForWindAndSolar(goentsoe.IntradayProcess, goentsoe.PsrTypeSolar, "10Y1001A1001A83F", nowCut, after)
	if err != nil {
		log.Fatal(err)
	}
	solar := process(r, "dayAheadGenerationForecastSolar", true)

	r, err = cli.GetDayAheadGenerationForecastsForWindAndSolar(goentsoe.IntradayProcess, goentsoe.PsrTypeWindOnshore, "10Y1001A1001A83F", nowCut, after)
	if err != nil {
		log.Fatal(err)
	}
	windOnShore := process(r, "dayAheadGenerationForecastWindOnshore", true)

	r, err = cli.GetDayAheadGenerationForecastsForWindAndSolar(goentsoe.IntradayProcess, goentsoe.PsrTypeWindOffshore, "10Y1001A1001A83F", nowCut, after)
	if err != nil {
		log.Fatal(err)
	}
	windOffShore := process(r, "dayAheadGenerationForecastWindOffshore", true)

	// biomass

	before := nowCut.AddDate(0, 0, -1)
	r, err = cli.GetAggregatedGenerationPerType(goentsoe.Realised, goentsoe.PsrTypeBiomass, "10Y1001A1001A83F", before, nowCut)
	if err != nil {
		log.Fatal(err)
	}
	biomass := process(r, "", true)

	spew.Dump(biomass)

	var biomassAverage int64
	for _, v := range biomass {
		biomassAverage += v.Value
	}
	biomassAverage /= int64(len(biomass))

	// hydro run

	r, err = cli.GetAggregatedGenerationPerType(goentsoe.Realised, goentsoe.PsrTypeHydroRunOfRiverAndPoundage, "10Y1001A1001A83F", before, nowCut)
	if err != nil {
		log.Fatal(err)
	}
	hydroRun := process(r, "", true)

	spew.Dump(hydroRun)

	var hydroRunAverage int64
	for _, v := range hydroRun {
		hydroRunAverage += v.Value
	}
	hydroRunAverage /= int64(len(hydroRun))

	// hydro Reservoir

	r, err = cli.GetAggregatedGenerationPerType(goentsoe.Realised, goentsoe.PsrTypeHydroWaterReservoir, "10Y1001A1001A83F", before, nowCut)
	if err != nil {
		log.Fatal(err)
	}
	hydroReservoir := process(r, "", true)

	spew.Dump(hydroReservoir)

	var hydroReservoirAverage int64
	for _, v := range hydroReservoir {
		hydroReservoirAverage += v.Value
	}
	hydroReservoirAverage /= int64(len(hydroReservoir))

	renewablesTotal := []Foo{}
	percent := []Foo{}
	for i, v := range solar {
		sum := v.Value + windOnShore[i].Value + windOffShore[i].Value + biomassAverage + hydroRunAverage + hydroReservoirAverage
		renewablesTotal = append(renewablesTotal, Foo{Time: v.Time,
			Value: sum,
		})
		percent = append(percent, Foo{Time: v.Time,
			Value: (sum * 100) / total[i].Value,
		})
	}

	b, err := json.MarshalIndent(&renewablesTotal, "", "    ")
	if err != nil {
		panic(err)
	}

	fh, err := os.Create("renewables" + ".js")
	defer fh.Close()
	if err != nil {
		panic(err)
	}
	fh.WriteString("var " + "renewables" + " = ")
	fh.Write(b)
	fmt.Println(string(b))

	b, err = json.MarshalIndent(&percent, "", "    ")
	if err != nil {
		panic(err)
	}

	fh, err = os.Create("percent" + ".js")
	defer fh.Close()
	if err != nil {
		panic(err)
	}
	fh.WriteString("var " + "percent" + " = ")
	fh.Write(b)
	fmt.Println(string(b))
}

func process(r *goentsoe.GLMarketDocument, name string, onlyFirstSeries bool) []Foo {

	var res []Foo
	spew.Dump(r)

	for i, v := range r.TimeSeries {
		if i > 0 && onlyFirstSeries {
			break
		}
		period := v.Period

		timeIntervalStart, err := time.Parse("2006-01-02T15:04Z", period.TimeInterval.Start)
		fmt.Println(v.BusinessType)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(timeIntervalStart)

		t := timeIntervalStart
		points := period.Point
		for _, point := range points {
			x, _ := strconv.ParseInt(point.Quantity, 10, 64)
			res = append(res, Foo{Time: JSONTime(t),
				Value: x,
			})
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

		timeIntervalEnd, err := time.Parse("2006-01-02T15:04Z", period.TimeInterval.End)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(timeIntervalEnd)
	}

	b, err := json.MarshalIndent(&res, "", "    ")
	if err != nil {
		panic(err)
	}

	if name != "" {
		fh, err := os.Create(name + ".js")
		defer fh.Close()
		if err != nil {
			panic(err)
		}
		fh.WriteString("var " + name + " = ")
		fh.Write(b)
		fmt.Println(string(b))
	}

	return res
}

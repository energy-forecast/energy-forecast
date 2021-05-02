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
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"text/template"
	"time"

	goentsoe "github.com/energy-forecast/go-entsoe"
	"github.com/foomo/tlsconfig"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"github.com/tjeske/simplecert"
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
	Time  goentsoe.JSONTime `json:"time"`
	Value int64             `json:"value"`
}

//go:embed dashboard.templ.html
var dashBoardTempl string
var log = logrus.New()

func main() {
	// do the cert magic
	cfg := simplecert.Default
	cfg.Domains = []string{"energy-forecast.de"}
	cfg.SSLEmail = "you@emailprovider.com"
	cfg.CacheDir = "letsencrypt"
	cfg.UpdateHosts = false
	cfg.Local = false
	certReloader, err := simplecert.Init(cfg, nil)
	if err != nil {
		log.Fatal("simplecert init failed: ", err)
	}

	// redirect HTTP to HTTPS
	log.Println("starting HTTP Listener on Port 80")
	go http.ListenAndServe(":80", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		target := "https://"
		target += req.Host

		target += req.URL.Path
		if len(req.URL.RawQuery) > 0 {
			target += "?" + req.URL.RawQuery
		}

		log.Info("redirecting client to https: ", target, " ("+req.Host+")", "UserAgent:", req.UserAgent())
		http.Redirect(w, req, target, http.StatusTemporaryRedirect)
	}))

	// init strict tlsConfig with certReloader
	// you could also use a default &tls.Config{}, but be warned this is highly insecure
	tlsconf := tlsconfig.NewServerTLSConfig(tlsconfig.TLSModeServerStrict)

	// now set GetCertificate to the reloaders GetCertificateFunc to enable hot reload
	tlsconf.GetCertificate = certReloader.GetCertificateFunc()

	router := chi.NewRouter()
	chiLogger := middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log, NoColor: false})
	router.Use(middleware.NoCache, chiLogger)

	e := createEnergyForecast()
	defer e.close()

	router.Get("/api/load", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		params, err := getApiParameters(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		res, err := e.getData(params, e.dbLoadHistory, e.dbLoadForecast, &LoadDataHandler{})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		jsonString := getJson(res)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonString))
	}))

	router.Get("/api/production/solar", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		params, err := getApiParameters(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		res, err := e.getData(params, e.dbSolarHistory, e.dbSolarForecast, &ProductionDataHandler{goentsoe.PsrTypeSolar})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		jsonString := getJson(res)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonString))
	}))

	router.Get("/api/production/windOnshore", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		params, err := getApiParameters(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		res, err := e.getData(params, e.dbWindOnshoreHistory, e.dbWindOnshoreForecast, &ProductionDataHandler{goentsoe.PsrTypeWindOnshore})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		jsonString := getJson(res)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonString))
	}))

	router.Get("/api/production/windOffshore", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		params, err := getApiParameters(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		res, err := e.getData(params, e.dbWindOffshoreHistory, e.dbWindOffshoreForecast, &ProductionDataHandler{goentsoe.PsrTypeWindOffshore})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - " + err.Error()))
			return
		}
		jsonString := getJson(res)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonString))
	}))

	router.Get("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defaultHandler(w, req, e)
	}))

	// init server
	s := &http.Server{
		Addr:      ":443",
		TLSConfig: tlsconf,
		Handler:   router,
	}

	log.Info("Listening on :8000...")
	log.Fatal(s.ListenAndServeTLS("", ""))
}

type apiParameters struct {
	from        time.Time
	to          time.Time
	domain      string
	domainShort string
}

func getApiParameters(req *http.Request) (*apiParameters, error) {
	q := req.URL.Query()
	domainMapping := map[string]goentsoe.DomainType{
		"AL":     goentsoe.DomainAL,
		"AT":     goentsoe.DomainAT,
		"BA":     goentsoe.DomainBA,
		"BE":     goentsoe.DomainBE,
		"BG":     goentsoe.DomainBG,
		"BY":     goentsoe.DomainBY,
		"CH":     goentsoe.DomainCH,
		"CZ":     goentsoe.DomainCZ,
		"DE":     goentsoe.DomainDE,
		"DK":     goentsoe.DomainDK,
		"EE":     goentsoe.DomainEE,
		"ES":     goentsoe.DomainES,
		"FI":     goentsoe.DomainFI,
		"FR":     goentsoe.DomainFR,
		"GB":     goentsoe.DomainGB,
		"GBNIR":  goentsoe.DomainGBNIR,
		"GR":     goentsoe.DomainGR,
		"HR":     goentsoe.DomainHR,
		"HU":     goentsoe.DomainHU,
		"IE":     goentsoe.DomainIE,
		"IT":     goentsoe.DomainIT,
		"LT":     goentsoe.DomainLT,
		"LU":     goentsoe.DomainLU,
		"LV":     goentsoe.DomainLV,
		"ME":     goentsoe.DomainME,
		"MK":     goentsoe.DomainMK,
		"MT":     goentsoe.DomainMT,
		"NL":     goentsoe.DomainNL,
		"NO":     goentsoe.DomainNO,
		"PL":     goentsoe.DomainPL,
		"PT":     goentsoe.DomainPT,
		"RO":     goentsoe.DomainRO,
		"RS":     goentsoe.DomainRS,
		"RU":     goentsoe.DomainRU,
		"RUKGD":  goentsoe.DomainRUKGD,
		"SE":     goentsoe.DomainSE,
		"SI":     goentsoe.DomainSI,
		"SK":     goentsoe.DomainSK,
		"TR":     goentsoe.DomainTR,
		"UA":     goentsoe.DomainUA,
		"DEATLU": goentsoe.DomainDEATLU,
	}
	domainShortStr := q.Get("domain")
	domain, ok := domainMapping[domainShortStr]
	if !ok {
		domainShortStr = "DE"
		domain = domainMapping[domainShortStr]
	}
	var err error

	var from time.Time
	if q.Get("from") != "" {
		from, err = time.Parse(time.RFC3339, q.Get("from"))
		if err != nil {
			return nil, err
		}
		from = time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), 0, 0, 0, from.Location())
	} else {
		fmt.Println(from)
		now := time.Now().UTC()
		from = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		fmt.Println(from)
	}

	var to time.Time
	if q.Get("to") != "" {
		to, err = time.Parse(time.RFC3339, q.Get("to"))
		if err != nil {
			return nil, err
		}
		to = time.Date(to.Year(), to.Month(), to.Day(), to.Hour(), 0, 0, 0, to.Location())
	} else {
		to = from.AddDate(0, 0, 1)
	}

	if to.Before(from) {
		return nil, errors.New("'to' timestamp before 'from' timestamp")
	}

	return &apiParameters{from, to, domain, domainShortStr}, nil
}

type EnergyForcast struct {
	cli                    *goentsoe.EntsoeClient
	dbLoadHistory          *buntdb.DB
	dbLoadForecast         *buntdb.DB
	dbSolarHistory         *buntdb.DB
	dbSolarForecast        *buntdb.DB
	dbWindOnshoreHistory   *buntdb.DB
	dbWindOnshoreForecast  *buntdb.DB
	dbWindOffshoreHistory  *buntdb.DB
	dbWindOffshoreForecast *buntdb.DB
}

func createEnergyForecast() *EnergyForcast {
	cli := goentsoe.NewEntsoeClientFromEnv()

	err := os.MkdirAll("cache", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	dbLoadHistory, err := buntdb.Open("cache/loadHistory.db")
	if err != nil {
		log.Fatal(err)
	}
	dbLoadForecast, err := buntdb.Open("cache/loadForecast.db")
	if err != nil {
		log.Fatal(err)
	}
	dbSolarHistory, err := buntdb.Open("cache/solarHistory.db")
	if err != nil {
		log.Fatal(err)
	}
	dbSolarForecast, err := buntdb.Open("cache/solarForecast.db")
	if err != nil {
		log.Fatal(err)
	}
	dbWindOnshoreHistory, err := buntdb.Open("cache/windOnshoreHistory.db")
	if err != nil {
		log.Fatal(err)
	}
	dbWindOnshoreForecast, err := buntdb.Open("cache/windOnshoreForecast.db")
	if err != nil {
		log.Fatal(err)
	}
	dbWindOffshoreHistory, err := buntdb.Open("cache/windOffshoreHistory.db")
	if err != nil {
		log.Fatal(err)
	}
	dbWindOffshoreForecast, err := buntdb.Open("cache/windOffshoreForecast.db")
	if err != nil {
		log.Fatal(err)
	}

	e := EnergyForcast{
		cli:                    cli,
		dbLoadHistory:          dbLoadHistory,
		dbLoadForecast:         dbLoadForecast,
		dbSolarHistory:         dbSolarHistory,
		dbSolarForecast:        dbSolarForecast,
		dbWindOnshoreHistory:   dbWindOnshoreHistory,
		dbWindOnshoreForecast:  dbWindOnshoreForecast,
		dbWindOffshoreHistory:  dbWindOffshoreHistory,
		dbWindOffshoreForecast: dbWindOffshoreForecast,
	}
	return &e
}

func (e *EnergyForcast) close() {
	e.dbLoadHistory.Close()
	e.dbLoadForecast.Close()
	e.dbSolarHistory.Close()
	e.dbSolarForecast.Close()
	e.dbWindOnshoreHistory.Close()
	e.dbWindOnshoreForecast.Close()
	e.dbWindOffshoreHistory.Close()
	e.dbWindOffshoreForecast.Close()
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

func (e *EnergyForcast) getTotalLoadForecast(domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := e.cli.GetDayAheadTotalLoadForecast(domain, from, to)
	if err != nil {
		return nil, err
	}
	res = e.cli.ConvertGlMarketDocument2Map2(r)
	return res, nil
}

func (e *EnergyForcast) getActualTotalLoad(domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
	res := make(map[goentsoe.JSONTime]int64)
	r, err := e.cli.GetActualTotalLoad(domain, from, to)
	if err != nil {
		return nil, err
	}
	res = e.cli.ConvertGlMarketDocument2Map2(r)
	return res, nil
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

func (e *EnergyForcast) foo(res map[goentsoe.JSONTime]int64, t1, t2 time.Time, domain, domainShortStr string, db *buntdb.DB, fn func(domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error)) error {
	t := t1
	needLoad := false
	err := db.View(func(tx *buntdb.Tx) error {
		for t.Before(t2) {
			v, err := tx.Get(getKey(domainShortStr, t))
			if err != nil {
				needLoad = true
				break
			}
			if v != "" {
				v2, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				res[goentsoe.JSONTime(t)] = v2
			}
			t = t.Add(15 * time.Minute)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if needLoad {
		data, err := fn(domain, t1, t2)
		if err != nil {
			return err
		}
		t := t1
		err = db.Update(func(tx *buntdb.Tx) error {
			for t.Before(t2) {
				val, ok := data[goentsoe.JSONTime(t)]
				key := getKey(domainShortStr, t)
				if !ok {
					_, _, err = tx.Set(key, "", nil)
				} else {
					_, _, err = tx.Set(key, strconv.FormatInt(val, 10), nil)
					res[goentsoe.JSONTime(t)] = val
				}
				t = t.Add(15 * time.Minute)
			}
			return err
		})
	}
	return nil
}

func (e *EnergyForcast) getData(params *apiParameters, dbLoad *buntdb.DB, dbLoadForecast *buntdb.DB, dh DataHandler) (map[goentsoe.JSONTime]int64, error) {
	from := params.from
	to := params.to
	domain := params.domain
	domainShort := params.domainShort

	nowTemp := time.Now().UTC()
	now := time.Date(nowTemp.Year(), nowTemp.Month(), nowTemp.Day(), nowTemp.Hour(), 0, 0, 0, nowTemp.Location())

	res := make(map[goentsoe.JSONTime]int64, 0)
	t1 := from
	for t1.Before(to) {
		t2 := t1.AddDate(1, 0, 0)
		if t2.After(to) {
			t2 = to
		}
		if t1.Before(now) {
			if t2.After(now) {
				// overlapping with forecast
				// -> restrict to now
				t2 = now
			}
			e.foo(res, t1, t2, domain, domainShort, dbLoad, func(domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
				return dh.GetHistoryData(e.cli, domain, t1, t2)
			})
		} else {
			// only forecast
			e.foo(res, t1, t2, domain, domainShort, dbLoadForecast, func(domain goentsoe.DomainType, from, to time.Time) (map[goentsoe.JSONTime]int64, error) {
				return dh.GetForecastData(e.cli, domain, t1, t2)
			})
		}
		t1 = t2
	}
	return res, nil
}

type Forecasts struct {
	ForecastLoad              string
	ForecastBiomass           string
	ForecastHydro             string
	ForecastSolar             string
	ForecastWindOnShore       string
	ForecastWindOffShore      string
	ForecastRenewables        string
	ForecastRenewablesPercent string
}

func defaultHandler(w http.ResponseWriter, r *http.Request, e *EnergyForcast) {
	now := time.Now().UTC()
	nowCut := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	after := nowCut.AddDate(0, 0, 1)
	//afterCut := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, now.Location())
	before := nowCut.AddDate(0, 0, -1)

	log.Info("get load forecast...")
	total, _ := e.getTotalLoadForecast(goentsoe.DomainDE, nowCut, after)

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
	biomass := make(map[goentsoe.JSONTime]int64)
	hydro := make(map[goentsoe.JSONTime]int64)
	for _, v := range getSortedTimes(solar) {
		biomass[v] = biomassAverage
		hydro[v] = hydroRunAverage + hydroReservoirAverage
		sum := solar[v] + windOnShore[v] + windOffShore[v] + biomassAverage + hydroRunAverage + hydroReservoirAverage
		renewablesTotal[v] = sum
		if total[v] != 0 {
			percent[v] = (sum * 100) / total[v]
		}
	}

	tpl, err := template.New("template").Parse(dashBoardTempl)
	if err != nil {
		log.Print(err)
		return
	}
	f := Forecasts{}
	f.ForecastLoad = getJson(total)
	f.ForecastBiomass = getJson(biomass)
	f.ForecastHydro = getJson(hydro)
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

func getKey(domainShortStr string, t time.Time) string {
	return domainShortStr + "_" + time.Time(t).UTC().Format(time.RFC3339)
}

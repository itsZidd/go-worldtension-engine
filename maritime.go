package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	portsMetaURL = "https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/PortWatch_ports_database/FeatureServer/0/query?where=1%3D1&outFields=ISO3%2Cportid%2Cvessel_count_total%2Cshare_country_maritime_import%2Cshare_country_maritime_export&outSR=4326&f=json"

	chokepointsMetaURL = "https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/PortWatch_chokepoints_database/FeatureServer/0/query?where=1%3D1&outFields=portid%2Cportname%2Cvessel_count_total&outSR=4326&f=json"

	dailyPortsBase       = "https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/Daily_Ports_Data/FeatureServer/0/query"
	dailyChokepointsBase = "https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/Daily_Chokepoints_Data/FeatureServer/0/query"

	disruptionsBase = "https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/portwatch_disruptions_database/FeatureServer/0/query"
)

var (
	maritimeCache   map[string]MaritimeData
	maritimeMu      sync.Mutex
	maritimeFetched bool

	// portToISO maps portid → ISO3, built from static ports DB
	portToISO map[string]string

	// chokepointBaseline maps portid → baseline vessel_count_total
	chokepointBaseline map[string]float64

	// chokepointExposure maps portid → list of exposed ISO3 countries
	// built dynamically from the static chokepoints DB + known exposure map
	chokepointExposure map[string][]string
)

type MaritimeData struct {
	PortCallsAvg      float64
	BaselineCallsAvg  float64
	ChokepointRisk    float64
	ActiveDisruptions int
}

func FetchPortThroughput(iso string) (float64, error) {
	maritimeMu.Lock()
	defer maritimeMu.Unlock()

	if !maritimeFetched {
		maritimeCache = make(map[string]MaritimeData)
		if err := refreshMaritimeCache(); err != nil {
			log.Printf("[maritime] fetch failed, will retry next run: %v", err)
			return 0, err
		}
		maritimeFetched = true
	}

	data, ok := maritimeCache[iso]
	if !ok {
		return 0, nil
	}

	pctChange := 0.0
	if data.BaselineCallsAvg > 0 {
		dailyBaseline := data.BaselineCallsAvg / 365.0
		pctChange = ((data.PortCallsAvg - dailyBaseline) / dailyBaseline) * 100.0
	}

	blended := pctChange
	blended -= data.ChokepointRisk * 5.0
	blended -= float64(data.ActiveDisruptions) * 1.0
	return blended, nil
}

func refreshMaritimeCache() error {
	// Step 1: load static baselines for ports AND chokepoints
	if err := fetchPortsBaseline(); err != nil {
		return fmt.Errorf("ports baseline: %w", err)
	}
	if err := fetchChokepointsBaseline(); err != nil {
		// Non-fatal — we can still run without chokepoint risk
		log.Printf("[maritime] chokepoints baseline failed (non-fatal): %v", err)
	}

	// Step 2: daily actuals
	if err := fetchDailyPorts(); err != nil {
		return fmt.Errorf("daily ports: %w", err)
	}
	if err := fetchDailyChokepoints(); err != nil {
		log.Printf("[maritime] daily chokepoints failed (non-fatal): %v", err)
	}

	// Step 3: active disruptions
	if err := fetchDisruptionsData(); err != nil {
		log.Printf("[maritime] disruptions failed (non-fatal): %v", err)
	}

	log.Printf("[maritime] cache refreshed: %d countries", len(maritimeCache))
	return nil
}

// ── Static baselines ──────────────────────────────────────────────────────────

func fetchPortsBaseline() error {
	portToISO = make(map[string]string)
	type agg struct {
		weightedCalls float64
		totalWeight   float64
	}
	byCountry := make(map[string]*agg)

	offset := 0
	pageSize := 1000
	for {
		pageURL := fmt.Sprintf(
			"%s&resultRecordCount=%d&resultOffset=%d",
			portsMetaURL, pageSize, offset,
		)
		var data arcgisResponse
		if err := arcgisGet(pageURL, &data); err != nil {
			return err
		}
		if len(data.Features) == 0 {
			break
		}
		for _, f := range data.Features {
			iso := strings.ToUpper(stringField(f.Attributes, "ISO3"))
			portID := stringField(f.Attributes, "portid")
			if iso == "" {
				continue
			}
			if portID != "" {
				portToISO[portID] = iso
			}
			calls := floatField(f.Attributes, "vessel_count_total")
			importShare := floatField(f.Attributes, "share_country_maritime_import")
			exportShare := floatField(f.Attributes, "share_country_maritime_export")
			weight := (importShare + exportShare) / 2.0
			if weight == 0 {
				weight = 0.01
			}
			if _, ok := byCountry[iso]; !ok {
				byCountry[iso] = &agg{}
			}
			byCountry[iso].weightedCalls += calls * weight
			byCountry[iso].totalWeight += weight
		}
		if !data.ExceededTransferLimit {
			break
		}
		offset += pageSize
	}

	for iso, b := range byCountry {
		if b.totalWeight == 0 {
			continue
		}
		maritimeCache[iso] = MaritimeData{
			BaselineCallsAvg: b.weightedCalls / b.totalWeight,
		}
	}
	log.Printf("[maritime] ports baseline: %d countries, %d port IDs mapped", len(byCountry), len(portToISO))
	return nil
}

func fetchChokepointsBaseline() error {
	var data arcgisResponse
	if err := arcgisGet(chokepointsMetaURL, &data); err != nil {
		return err
	}
	if len(data.Features) == 0 {
		return fmt.Errorf("chokepoints metadata returned 0 features")
	}

	chokepointBaseline = make(map[string]float64)

	// Known exposure: chokepoint portname → exposed ISO3 list
	// Names confirmed from the static DB: "Suez Canal", "Panama Canal", "Bosporus Strait"...
	nameToExposure := map[string][]string{
		"Suez Canal":            {"EGY", "ISR", "JOR", "SAU", "YEM", "ITA", "GRC", "TUR", "LBN"},
		"Panama Canal":          {"PAN", "COL", "MEX", "USA", "CRI", "HND"},
		"Bosporus Strait":       {"TUR", "BGR", "ROU", "UKR", "RUS"},
		"Strait of Hormuz":      {"IRN", "OMN", "ARE", "QAT", "KWT", "IRQ", "SAU"},
		"Strait of Malacca":     {"MYS", "SGP", "IDN", "THA", "IND", "CHN", "JPN", "KOR"},
		"Bab-el-Mandeb":         {"DJI", "ERI", "YEM", "ETH", "SOM", "EGY"},
		"Danish Straits":        {"DNK", "SWE", "NOR", "FIN", "DEU", "POL"},
		"English Channel":       {"GBR", "FRA", "BEL", "NLD", "DEU"},
		"Lombok Strait":         {"IDN", "AUS", "CHN", "JPN"},
		"Mozambique Channel":    {"MOZ", "MDG", "ZAF", "TZA"},
		"Cape of Good Hope":     {"ZAF", "NAM", "AGO"},
		"Cape Horn":             {"CHL", "ARG"},
		"Luzon Strait":          {"PHL", "CHN", "TWN", "JPN"},
		"Taiwan Strait":         {"CHN", "TWN", "JPN", "KOR"},
		"Korea Strait":          {"KOR", "JPN", "CHN"},
		"Windward Passage":      {"HTI", "CUB", "JAM"},
		"Florida Strait":        {"USA", "CUB", "BHS"},
		"Yucatan Channel":       {"MEX", "CUB"},
		"Gulf of Aden":          {"YEM", "DJI", "SOM", "ERI"},
		"Strait of Sicily":      {"ITA", "TUN", "MLT"},
		"Otranto Strait":        {"ITA", "ALB", "HRV", "GRC"},
		"Kattegat":              {"DNK", "SWE"},
		"Sound (Oresund)":       {"DNK", "SWE"},
		"Great Belt":            {"DNK"},
		"Tsugaru Strait":        {"JPN"},
		"Soya Strait":           {"JPN", "RUS"},
		"Strait of Gibraltar":   {"ESP", "MAR", "PRT", "GBR", "FRA"},
		"Korea/Tsushima Strait": {"KOR", "JPN"},
		"Bab el-Mandeb Strait":  {"DJI", "ERI", "YEM", "ETH", "SOM", "EGY"},
		"Malacca Strait":        {"MYS", "SGP", "IDN", "THA", "IND", "CHN", "JPN", "KOR"},
		"Gibraltar Strait":      {"ESP", "MAR", "PRT", "GBR", "FRA"},
		"Dover Strait":          {"GBR", "FRA", "BEL", "NLD", "DEU"},
		"Oresund Strait":        {"DNK", "SWE"},
		"Ombai Strait":          {"IDN", "TLS", "AUS"},
		"Bohai Strait":          {"CHN"},
		"Torres Strait":         {"AUS", "PNG"},
		"Sunda Strait":          {"IDN"},
		"Makassar Strait":       {"IDN", "MYS"},
		"Magellan Strait":       {"CHL", "ARG"},
		"Mona Passage":          {"DOM", "PRI", "HTI"},
		"Balabac Strait":        {"PHL", "MYS", "BRN"},
		"Bering Strait":         {"USA", "RUS"},
		"Mindoro Strait":        {"PHL"},
		"Kerch Strait":          {"RUS", "UKR"},
	}

	chokepointExposure = make(map[string][]string)

	for _, f := range data.Features {
		portID := stringField(f.Attributes, "portid")
		name := stringField(f.Attributes, "portname")
		baseline := floatField(f.Attributes, "vessel_count_total")

		if portID == "" {
			continue
		}

		chokepointBaseline[portID] = baseline

		if exposed, ok := nameToExposure[name]; ok {
			chokepointExposure[portID] = exposed
		} else {
			log.Printf("[maritime] unmapped chokepoint name: %q (portid: %s) — add to nameToExposure", name, portID)
		}
	}

	log.Printf("[maritime] chokepoints baseline: %d chokepoints loaded", len(chokepointBaseline))
	return nil
}

// ── Daily actuals ─────────────────────────────────────────────────────────────

func fetchDailyPorts() error {
	// Probe latest date using integer fields
	probeURL := dailyPortsBase +
		"?where=1%3D1" +
		"&outFields=year,month,day" +
		"&orderByFields=year+DESC,month+DESC,day+DESC" +
		"&resultRecordCount=1" +
		"&f=json"

	var probe arcgisResponse
	if err := arcgisGet(probeURL, &probe); err != nil {
		return fmt.Errorf("probe latest date: %w", err)
	}
	if len(probe.Features) == 0 {
		return fmt.Errorf("daily ports probe returned 0 features")
	}

	latestYear := int(floatField(probe.Features[0].Attributes, "year"))
	latestMonth := int(floatField(probe.Features[0].Attributes, "month"))
	latestDay := int(floatField(probe.Features[0].Attributes, "day"))

	log.Printf("[maritime] daily ports latest date: %04d-%02d-%02d", latestYear, latestMonth, latestDay)

	whereClause := url.QueryEscape(fmt.Sprintf("year=%d AND month=%d AND day=%d", latestYear, latestMonth, latestDay))

	type agg struct {
		portcallsSum float64
		count        int
	}
	byCountry := make(map[string]*agg)

	offset := 0
	pageSize := 1000
	totalRecords := 0

	for {
		pageURL := fmt.Sprintf(
			"%s?where=%s&outFields=ISO3,portid,portcalls&resultRecordCount=%d&resultOffset=%d&f=json",
			dailyPortsBase, whereClause, pageSize, offset,
		)

		var data arcgisResponse
		if err := arcgisGet(pageURL, &data); err != nil {
			return fmt.Errorf("daily ports fetch (offset %d): %w", offset, err)
		}
		if len(data.Features) == 0 {
			break
		}

		for _, f := range data.Features {
			iso := strings.ToUpper(stringField(f.Attributes, "ISO3"))
			if iso == "" {
				if pid := stringField(f.Attributes, "portid"); pid != "" {
					iso = portToISO[pid]
				}
			}
			if iso == "" {
				continue
			}
			if _, ok := byCountry[iso]; !ok {
				byCountry[iso] = &agg{}
			}
			byCountry[iso].portcallsSum += floatField(f.Attributes, "portcalls")
			byCountry[iso].count++
		}

		totalRecords += len(data.Features)

		if !data.ExceededTransferLimit {
			break
		}
		offset += pageSize
	}

	for iso, a := range byCountry {
		existing := maritimeCache[iso]
		existing.PortCallsAvg = a.portcallsSum / float64(a.count)
		maritimeCache[iso] = existing
	}

	log.Printf("[maritime] daily ports: %d countries from %d records (%04d-%02d-%02d)",
		len(byCountry), totalRecords, latestYear, latestMonth, latestDay)
	return nil
}

func fetchDailyChokepoints() error {
	// Probe latest date using integer fields (same pattern as daily ports)
	probeURL := dailyChokepointsBase +
		"?where=1%3D1" +
		"&outFields=year,month,day" +
		"&orderByFields=year+DESC,month+DESC,day+DESC" +
		"&resultRecordCount=1" +
		"&f=json"

	var probe arcgisResponse
	if err := arcgisGet(probeURL, &probe); err != nil {
		return fmt.Errorf("probe chokepoint date: %w", err)
	}
	if len(probe.Features) == 0 {
		return fmt.Errorf("daily chokepoints probe returned 0 features")
	}

	latestYear := int(floatField(probe.Features[0].Attributes, "year"))
	latestMonth := int(floatField(probe.Features[0].Attributes, "month"))
	latestDay := int(floatField(probe.Features[0].Attributes, "day"))

	log.Printf("[maritime] daily chokepoints latest date: %04d-%02d-%02d", latestYear, latestMonth, latestDay)

	// Fetch all 28 chokepoints for that exact date
	whereClause := fmt.Sprintf("year=%d AND month=%d AND day=%d", latestYear, latestMonth, latestDay)
	dataURL := fmt.Sprintf(
		"%s?where=%s&outFields=portid,portname,n_total,capacity&resultRecordCount=100&f=json",
		dailyChokepointsBase,
		url.QueryEscape(whereClause),
	)

	var data arcgisResponse
	if err := arcgisGet(dataURL, &data); err != nil {
		return fmt.Errorf("daily chokepoints fetch: %w", err)
	}
	if len(data.Features) == 0 {
		return fmt.Errorf("daily chokepoints returned 0 features for %04d-%02d-%02d", latestYear, latestMonth, latestDay)
	}

	log.Printf("[maritime] daily chokepoints: %d chokepoints for %04d-%02d-%02d", len(data.Features), latestYear, latestMonth, latestDay)

	for _, f := range data.Features {
		portID := stringField(f.Attributes, "portid")
		name := stringField(f.Attributes, "portname")
		if portID == "" {
			continue
		}

		baseline, hasBaseline := chokepointBaseline[portID]
		if !hasBaseline || baseline == 0 {
			log.Printf("[maritime] no baseline for chokepoint portid=%s name=%q", portID, name)
			continue
		}

		nTotal := floatField(f.Attributes, "n_total")
		pctChange := ((nTotal - baseline) / baseline) * 100.0

		if pctChange >= -10.0 {
			continue // within normal range
		}

		severity := (-pctChange - 10.0) / 25.0
		if severity > 1.0 {
			severity = 1.0
		}

		exposed, ok := chokepointExposure[portID]
		if !ok {
			log.Printf("[maritime] no exposure map for chokepoint portid=%s name=%q — add to nameToExposure", portID, name)
			continue
		}

		log.Printf("[maritime] chokepoint disruption: %s — today=%.0f baseline=%.0f (%.1f%%) severity=%.2f",
			name, nTotal, baseline, pctChange, severity)

		for _, iso := range exposed {
			existing := maritimeCache[iso]
			if severity > existing.ChokepointRisk {
				existing.ChokepointRisk = severity
			}
			maritimeCache[iso] = existing
		}
	}

	return nil
}

// ── Disruptions ───────────────────────────────────────────────────────────────

func fetchDisruptionsData() error {
	// ArcGIS date filter syntax uses TIMESTAMP format, not epoch ms
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	whereClause := fmt.Sprintf("todate >= TIMESTAMP '%s'", nowStr)

	dataURL := fmt.Sprintf(
		"%s?where=%s&outFields=eventtype,eventname,alertlevel,country,fromdate,todate,affectedports,n_affectedports&outSR=4326&f=json",
		disruptionsBase,
		url.QueryEscape(whereClause),
	)

	var data arcgisResponse
	if err := arcgisGet(dataURL, &data); err != nil {
		return err
	}

	log.Printf("[maritime] disruptions: %d active events", len(data.Features))

	for _, f := range data.Features {
		alertLevel := strings.ToUpper(stringField(f.Attributes, "alertlevel"))
		nAffected := floatField(f.Attributes, "n_affectedports")
		eventType := stringField(f.Attributes, "eventtype")

		weight := 1
		switch alertLevel {
		case "RED":
			weight = 3
		case "ORANGE":
			weight = 2
		}
		if nAffected > 5 {
			weight++
		}
		// GE=Geopolitical, CO=Conflict — higher tension relevance
		if eventType == "GE" || eventType == "CO" {
			weight++
		}

		// Primary: resolve affectedports portids → ISO3
		affectedISOs := resolvePortsToISO(stringField(f.Attributes, "affectedports"))

		// Fallback: parse country name string
		if len(affectedISOs) == 0 {
			affectedISOs = resolveCountryNamesToISO(stringField(f.Attributes, "country"))
		}

		if len(affectedISOs) == 0 {
			continue
		}

		for iso := range affectedISOs {
			existing := maritimeCache[iso]
			existing.ActiveDisruptions += weight
			maritimeCache[iso] = existing
		}
	}

	return nil
}

func resolvePortsToISO(affectedPorts string) map[string]struct{} {
	result := make(map[string]struct{})
	if affectedPorts == "" || portToISO == nil {
		return result
	}
	for _, pid := range strings.Split(affectedPorts, ",") {
		pid = strings.TrimSpace(pid)
		if iso, ok := portToISO[pid]; ok {
			result[iso] = struct{}{}
		}
	}
	return result
}

func resolveCountryNamesToISO(countryStr string) map[string]struct{} {
	result := make(map[string]struct{})
	if countryStr == "" {
		return result
	}
	lookup := map[string]string{
		"Afghanistan": "AFG", "Albania": "ALB", "Algeria": "DZA", "Angola": "AGO",
		"Argentina": "ARG", "Australia": "AUS", "Austria": "AUT", "Azerbaijan": "AZE",
		"Bangladesh": "BGD", "Belgium": "BEL", "Benin": "BEN", "Bolivia": "BOL",
		"Brazil": "BRA", "Bulgaria": "BGR", "Cambodia": "KHM", "Cameroon": "CMR",
		"Canada": "CAN", "Chile": "CHL", "China": "CHN", "Colombia": "COL",
		"Congo": "COG", "Costa Rica": "CRI", "Croatia": "HRV", "Cuba": "CUB",
		"Cyprus": "CYP", "Czech Republic": "CZE", "Denmark": "DNK", "Djibouti": "DJI",
		"Dominican Republic": "DOM", "DR Congo": "COD", "Ecuador": "ECU",
		"Egypt": "EGY", "El Salvador": "SLV", "Eritrea": "ERI", "Estonia": "EST",
		"Ethiopia": "ETH", "Finland": "FIN", "France": "FRA", "Gabon": "GAB",
		"Georgia": "GEO", "Germany": "DEU", "Ghana": "GHA", "Greece": "GRC",
		"Guatemala": "GTM", "Guinea": "GIN", "Haiti": "HTI", "Honduras": "HND",
		"Hungary": "HUN", "India": "IND", "Indonesia": "IDN", "Iran": "IRN",
		"Iraq": "IRQ", "Ireland": "IRL", "Israel": "ISR", "Italy": "ITA",
		"Ivory Coast": "CIV", "Jamaica": "JAM", "Japan": "JPN", "Jordan": "JOR",
		"Kazakhstan": "KAZ", "Kenya": "KEN", "Kuwait": "KWT", "Latvia": "LVA",
		"Lebanon": "LBN", "Libya": "LBY", "Lithuania": "LTU", "Luxembourg": "LUX",
		"Madagascar": "MDG", "Malaysia": "MYS", "Malta": "MLT", "Mauritania": "MRT",
		"Mauritius": "MUS", "Mexico": "MEX", "Moldova": "MDA", "Morocco": "MAR",
		"Mozambique": "MOZ", "Myanmar": "MMR", "Namibia": "NAM", "Netherlands": "NLD",
		"New Zealand": "NZL", "Nicaragua": "NIC", "Nigeria": "NGA", "Norway": "NOR",
		"Oman": "OMN", "Pakistan": "PAK", "Panama": "PAN", "Peru": "PER",
		"Philippines": "PHL", "Poland": "POL", "Portugal": "PRT", "Qatar": "QAT",
		"Romania": "ROU", "Russia": "RUS", "Saudi Arabia": "SAU", "Senegal": "SEN",
		"Singapore": "SGP", "Somalia": "SOM", "South Africa": "ZAF",
		"South Korea": "KOR", "Spain": "ESP", "Sri Lanka": "LKA", "Sudan": "SDN",
		"Sweden": "SWE", "Syria": "SYR", "Taiwan": "TWN", "Tanzania": "TZA",
		"Thailand": "THA", "Tunisia": "TUN", "Turkey": "TUR", "Ukraine": "UKR",
		"United Arab Emirates": "ARE", "UAE": "ARE", "United Kingdom": "GBR",
		"United States": "USA", "Uruguay": "URY", "Venezuela": "VEN",
		"Vietnam": "VNM", "Yemen": "YEM", "Zimbabwe": "ZWE",
	}
	for _, part := range strings.Split(countryStr, ",") {
		part = strings.TrimSpace(part)
		if iso, ok := lookup[part]; ok {
			result[iso] = struct{}{}
		}
	}
	return result
}

// ── ArcGIS helpers ────────────────────────────────────────────────────────────

type arcgisResponse struct {
	Features []struct {
		Attributes map[string]interface{} `json:"attributes"`
	} `json:"features"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ExceededTransferLimit bool `json:"exceededTransferLimit"`
}

func arcgisGet(rawURL string, out *arcgisResponse) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}
	if out.Error != nil {
		return fmt.Errorf("arcgis error %d: %s", out.Error.Code, out.Error.Message)
	}
	if out.ExceededTransferLimit {
		log.Printf("[maritime] WARNING: transfer limit exceeded for %s — results truncated, consider pagination", rawURL)
	}
	return nil
}

func stringField(attrs map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func floatField(attrs map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case int:
				return float64(n)
			case json.Number:
				f, _ := n.Float64()
				return f
			}
		}
	}
	return 0
}

func int64Field(attrs map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			switch n := v.(type) {
			case float64:
				return int64(n)
			case int64:
				return n
			case int:
				return int64(n)
			case json.Number:
				i, _ := n.Int64()
				return i
			}
		}
	}
	return 0
}
